package kube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 클러스터의 마지막 스택을 지우면 Nullus 가 설치한 공용 자원이 회수된다(stack 모듈의
// delete_shared_resources.go). 네임스페이스 덤프는 그것을 담지 않고, 클러스터 범위 덤프는
// Helm 소유 어노테이션으로 거르는데 Gateway API CRD 는 kubectl 로·GatewayClass 는 create 로
// 만들어져 어노테이션이 없다. 내부 CA 는 cert-manager 네임스페이스에 있다. 여기 테스트는
// 그 빈틈을 고정한다.

// fakeSharedKubectl 은 kubectl 호출을 흉내 낸다. 호출 순서와 stdin 을 남긴다.
type fakeSharedKubectl struct {
	calls   [][]string
	stdins  []string
	respond func(args []string, stdin string) (string, error)
}

func (f *fakeSharedKubectl) run(_ context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	var in string
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	f.calls = append(f.calls, args)
	f.stdins = append(f.stdins, in)
	if f.respond == nil {
		return nil
	}
	out, err := f.respond(args, in)
	if stdout != nil {
		_, _ = io.WriteString(stdout, out)
	}
	return err
}

func (f *fakeSharedKubectl) joined() []string {
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, strings.Join(c, " "))
	}
	return out
}

func jsonList(items ...string) string {
	return `{"apiVersion":"v1","kind":"List","items":[` + strings.Join(items, ",") + `]}`
}

const (
	crdGateways = `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"gateways.gateway.networking.k8s.io","labels":{"nullus.io/installed-by":"nullus"},"uid":"u1","resourceVersion":"9"},"spec":{"group":"gateway.networking.k8s.io"},"status":{"conditions":[]}}`
	crdEnvoy    = `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"envoyproxies.gateway.envoyproxy.io","labels":{"nullus.io/installed-by":"nullus"}},"spec":{"group":"gateway.envoyproxy.io"}}`
	// cert-manager CRD 도 설치 표시를 달고 깔린다. 그러나 컨트롤러 없는 CRD 는 되살려도 소용없고,
	// 남아 있으면 cert-manager 재설치를 Helm 소유권 충돌로 막는다 — 담지 않는다.
	crdCertManager = `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"certificates.cert-manager.io","labels":{"nullus.io/installed-by":"nullus"}},"spec":{"group":"cert-manager.io"}}`
	gatewayClass   = `{"apiVersion":"gateway.networking.k8s.io/v1","kind":"GatewayClass","metadata":{"name":"envoy","labels":{"nullus.io/installed-by":"nullus"}},"spec":{"controllerName":"gateway.envoyproxy.io/gatewayclass-controller"}}`
	issuerBoot     = `{"apiVersion":"cert-manager.io/v1","kind":"ClusterIssuer","metadata":{"name":"nullus-selfsigned-bootstrap"},"spec":{"selfSigned":{}}}`
	issuerCA       = `{"apiVersion":"cert-manager.io/v1","kind":"ClusterIssuer","metadata":{"name":"nullus-internal-ca-issuer"},"spec":{"ca":{"secretName":"nullus-internal-ca"}}}`
	caCert         = `{"apiVersion":"cert-manager.io/v1","kind":"Certificate","metadata":{"name":"nullus-internal-ca-cert","namespace":"cert-manager"},"spec":{"isCA":true,"secretName":"nullus-internal-ca"}}`
	caSecret       = `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"nullus-internal-ca","namespace":"cert-manager"},"type":"kubernetes.io/tls","data":{"tls.crt":"Y2E=","tls.key":"a2V5"}}`
)

const clusterKindsAll = "customresourcedefinitions.apiextensions.k8s.io\ngatewayclasses.gateway.networking.k8s.io\nclusterissuers.cert-manager.io\n"
const namespacedKindsAll = "secrets\ncertificates.cert-manager.io\n"

// respondFull 은 공용 자원이 전부 있는 클러스터다.
func respondFull(args []string, _ string) (string, error) {
	a := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(a, "api-resources") && strings.Contains(a, "--namespaced=false"):
		return clusterKindsAll, nil
	case strings.HasPrefix(a, "api-resources") && strings.Contains(a, "--namespaced=true"):
		return namespacedKindsAll, nil
	case strings.HasPrefix(a, "get crd"):
		return jsonList(crdGateways, crdEnvoy, crdCertManager), nil
	case strings.HasPrefix(a, "get gatewayclasses"):
		return jsonList(gatewayClass), nil
	case strings.HasPrefix(a, "get clusterissuers"):
		return jsonList(issuerBoot, issuerCA), nil
	case strings.HasPrefix(a, "get certificates"):
		return jsonList(caCert), nil
	case strings.HasPrefix(a, "get secrets"):
		return jsonList(caSecret), nil
	}
	return "", errors.New("unexpected kubectl call: " + a)
}

type sharedDoc struct {
	ClusterScoped struct {
		Items []map[string]any `json:"items"`
	} `json:"cluster_scoped"`
	Namespaced struct {
		Items []map[string]any `json:"items"`
	} `json:"namespaced"`
}

func names(items []map[string]any) []string {
	var out []string
	for _, it := range items {
		meta, _ := it["metadata"].(map[string]any)
		kind, _ := it["kind"].(string)
		name, _ := meta["name"].(string)
		out = append(out, kind+"/"+name)
	}
	return out
}

func TestDumpShared_설치_표시가_붙은_Gateway_CRD_와_GatewayClass_내부_CA_를_담는다(t *testing.T) {
	k := &fakeSharedKubectl{respond: respondFull}
	d := &ResourceDumper{exec: k.run}
	var out strings.Builder

	n, err := d.DumpShared(context.Background(), &out)
	require.NoError(t, err)
	assert.Positive(t, n)

	var doc sharedDoc
	require.NoError(t, json.Unmarshal([]byte(out.String()), &doc))
	assert.ElementsMatch(t, []string{
		"CustomResourceDefinition/gateways.gateway.networking.k8s.io",
		"CustomResourceDefinition/envoyproxies.gateway.envoyproxy.io",
		"GatewayClass/envoy",
		"ClusterIssuer/nullus-selfsigned-bootstrap",
		"ClusterIssuer/nullus-internal-ca-issuer",
	}, names(doc.ClusterScoped.Items))
	assert.ElementsMatch(t, []string{
		"Secret/nullus-internal-ca",
		"Certificate/nullus-internal-ca-cert",
	}, names(doc.Namespaced.Items))

	// 서버가 관리하는 필드는 걷어낸다 — 그대로 되돌리면 적용이 깨진다.
	for _, it := range doc.ClusterScoped.Items {
		meta := it["metadata"].(map[string]any)
		assert.NotContains(t, meta, "uid")
		assert.NotContains(t, meta, "resourceVersion")
		assert.NotContains(t, it, "status")
	}
	// 내부 CA 의 네임스페이스는 남긴다 — 어디에 있었는지가 복원 위치의 기본값이다.
	for _, it := range doc.Namespaced.Items {
		meta := it["metadata"].(map[string]any)
		assert.Equal(t, "cert-manager", meta["namespace"])
	}
	// 설치 표시 라벨로 고른다 — 사용자가 직접 깐 Gateway API 는 Nullus 것이 아니다.
	for _, c := range k.joined() {
		if strings.HasPrefix(c, "get crd") || strings.HasPrefix(c, "get gatewayclasses") {
			assert.Contains(t, c, "-l nullus.io/installed-by=nullus")
		}
	}
}

// cert-manager 가 없는 클러스터(옛 스택·외부 TLS)에서 ClusterIssuer 를 조회하면 kubectl 이
// 죽는다 — 없는 종류는 건너뛰고 있는 것만 담는다. 네임스페이스 덤프와 같은 규칙이다.
func TestDumpShared_없는_종류는_건너뛴다(t *testing.T) {
	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(a, "api-resources") && strings.Contains(a, "--namespaced=false"):
			return "customresourcedefinitions.apiextensions.k8s.io\ngatewayclasses.gateway.networking.k8s.io\n", nil
		case strings.HasPrefix(a, "api-resources") && strings.Contains(a, "--namespaced=true"):
			return "secrets\n", nil
		case strings.HasPrefix(a, "get clusterissuers"), strings.HasPrefix(a, "get certificates"):
			return "", errors.New("the server doesn't have a resource type")
		case strings.HasPrefix(a, "get secrets"):
			return jsonList(), nil
		}
		return respondFull(args, in)
	}}
	d := &ResourceDumper{exec: k.run}
	var out strings.Builder

	_, err := d.DumpShared(context.Background(), &out)
	require.NoError(t, err)

	var doc sharedDoc
	require.NoError(t, json.Unmarshal([]byte(out.String()), &doc))
	assert.ElementsMatch(t, []string{
		"CustomResourceDefinition/gateways.gateway.networking.k8s.io",
		"CustomResourceDefinition/envoyproxies.gateway.envoyproxy.io",
		"GatewayClass/envoy",
	}, names(doc.ClusterScoped.Items))
	assert.Empty(t, doc.Namespaced.Items)
}

// Nullus 가 설치한 공용 자원이 하나도 없으면(사용자가 깐 Gateway API 를 재사용한 스택)
// 아무것도 쓰지 않는다 — 빈 산출물은 "담았다" 로 읽힌다.
func TestDumpShared_아무것도_없으면_0(t *testing.T) {
	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		if strings.HasPrefix(a, "get ") {
			return jsonList(), nil
		}
		return respondFull(args, in)
	}}
	d := &ResourceDumper{exec: k.run}
	var out strings.Builder

	n, err := d.DumpShared(context.Background(), &out)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, out.String())
}

// ── 복원 ──────────────────────────────────────────────────────────────

func sharedPayload(t *testing.T) string {
	t.Helper()
	k := &fakeSharedKubectl{respond: respondFull}
	d := &ResourceDumper{exec: k.run}
	var out strings.Builder
	_, err := d.DumpShared(context.Background(), &out)
	require.NoError(t, err)
	return out.String()
}

func kindName(stdin string) string {
	var obj struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal([]byte(stdin), &obj)
	return obj.Kind + "/" + obj.Metadata.Name
}

// 공용 자원은 클러스터의 것이다. 다른 스택이 쓰는 지금의 CA 나 CRD 를 백업 때의 것으로
// 덮으면 그 스택이 깨진다 — 없을 때만 만들고, 있으면 그대로 둔다.
func TestApplyShared_없을_때만_만들고_순서를_지킨다(t *testing.T) {
	payload := sharedPayload(t)
	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(a, "create") && kindName(in) == "GatewayClass/envoy":
			return "", errors.New(`Error from server (AlreadyExists): gatewayclasses.gateway.networking.k8s.io "envoy" already exists`)
		case strings.HasPrefix(a, "get crd certificaterequests.cert-manager.io"):
			return "cert-manager", nil
		}
		return "", nil
	}}
	d := &ResourceDumper{exec: k.run}

	require.NoError(t, d.ApplyShared(context.Background(), strings.NewReader(payload)))

	var created []string
	var sawWait bool
	for i, c := range k.calls {
		a := strings.Join(c, " ")
		switch {
		case strings.HasPrefix(a, "create -f -"), strings.HasPrefix(a, "create -n"):
			created = append(created, kindName(k.stdins[i]))
			assert.NotContains(t, a, "apply", "덮어쓰지 않는다")
		case strings.HasPrefix(a, "wait"):
			sawWait = true
			assert.Contains(t, a, "crd")
		}
	}
	assert.Equal(t, []string{
		"CustomResourceDefinition/gateways.gateway.networking.k8s.io",
		"CustomResourceDefinition/envoyproxies.gateway.envoyproxy.io",
		"GatewayClass/envoy",
		"ClusterIssuer/nullus-selfsigned-bootstrap",
		"ClusterIssuer/nullus-internal-ca-issuer",
		"Secret/nullus-internal-ca",
		"Certificate/nullus-internal-ca-cert",
	}, created, "CRD → GatewayClass → ClusterIssuer → CA Secret → CA Certificate")
	assert.True(t, sawWait, "CRD 가 등록되기를 기다린 뒤 CR 을 만든다")
}

// cert-manager 가 없으면 ClusterIssuer 를 만들 곳이 없다. 어느 종류가 왜 안 되는지와 무엇을
// 먼저 해야 하는지를 말하고 멈춘다 — "no matches for kind" 만 남기면 운영자는 뭘 깔아야 하는지
// 모른다.
func TestApplyShared_CRD_가_없으면_이유를_말하고_멈춘다(t *testing.T) {
	payload := sharedPayload(t)
	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		if strings.HasPrefix(a, "create") && strings.HasPrefix(kindName(in), "ClusterIssuer/") {
			return "", errors.New(`error: resource mapping not found for name: "nullus-selfsigned-bootstrap" namespace: "" from "STDIN": no matches for kind "ClusterIssuer" in version "cert-manager.io/v1"`)
		}
		return "", nil
	}}
	d := &ResourceDumper{exec: k.run}

	err := d.ApplyShared(context.Background(), strings.NewReader(payload))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClusterIssuer")
	assert.Contains(t, err.Error(), "cert-manager")

	for i, c := range k.calls {
		if strings.HasPrefix(strings.Join(c, " "), "create") {
			assert.NotContains(t, kindName(k.stdins[i]), "Secret/", "CRD 가 없다고 안 뒤에는 더 만들지 않는다")
		}
	}
}

// 내부 CA 는 cert-manager 가 지금 설치된 네임스페이스에 있어야 한다. 백업 때와 다른
// 네임스페이스에 재설치됐으면 그쪽으로 옮겨 만든다 — ClusterIssuer 의 secretName 은
// cert-manager 의 cluster-resource-namespace 에서만 읽힌다.
func TestApplyShared_내부_CA_는_지금_cert_manager_네임스페이스에_만든다(t *testing.T) {
	payload := sharedPayload(t)
	k := &fakeSharedKubectl{respond: func(args []string, in string) (string, error) {
		a := strings.Join(args, " ")
		if strings.HasPrefix(a, "get crd certificaterequests.cert-manager.io") {
			return "cm-new", nil
		}
		return "", nil
	}}
	d := &ResourceDumper{exec: k.run}

	require.NoError(t, d.ApplyShared(context.Background(), strings.NewReader(payload)))

	var nsCreated bool
	for i, c := range k.calls {
		a := strings.Join(c, " ")
		if strings.HasPrefix(a, "create namespace cm-new") {
			nsCreated = true
		}
		if strings.HasPrefix(a, "create -n") {
			assert.Contains(t, a, "-n cm-new")
			assert.Contains(t, k.stdins[i], `"namespace":"cm-new"`)
		}
	}
	assert.True(t, nsCreated, "네임스페이스가 없으면 만든다(있으면 그대로)")
}

func TestApplyShared_빈_입력은_아무것도_하지_않는다(t *testing.T) {
	k := &fakeSharedKubectl{}
	d := &ResourceDumper{exec: k.run}
	require.NoError(t, d.ApplyShared(context.Background(), strings.NewReader("")))
	assert.Empty(t, k.calls)
}
