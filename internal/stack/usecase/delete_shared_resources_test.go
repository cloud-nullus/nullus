package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 클러스터의 마지막 스택을 지우면 Nullus 가 설치한 공용 자원(cert-manager·내부 CA·metrics-server·
// Gateway API)을 회수한다. Nullus 가 설치하지 않았거나, 아직 쓰는 곳이 있거나, 확인하지 못하면 남기고
// 그 이유를 알린다.

const (
	sharedStackID   = "stk-shared"
	sharedStackName = "demo"
	sharedNamespace = "nullus-demo" // domain.DefaultStackNamespaceFor("demo") — 스택 정리가 통째로 회수한다
	sharedClusterID = "cluster-shared"

	cmdStackNamespace      = "get namespace nullus-demo --ignore-not-found -o name"
	cmdHelmCertManager     = "get secret -A -l owner=helm,name=cert-manager --no-headers -o custom-columns=NS:.metadata.namespace"
	cmdHelmMetrics         = "get secret -A -l owner=helm,name=metrics-server --no-headers -o custom-columns=NS:.metadata.namespace"
	cmdCertificates        = "get certificates.cert-manager.io -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,ISSUER:.spec.issuerRef.name"
	cmdCertificateRequests = "get certificaterequests.cert-manager.io -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,ISSUER:.spec.issuerRef.name,OWNER:.metadata.ownerReferences[0].kind"
	cmdClusterIssuers      = "get clusterissuers.cert-manager.io -o name"
	cmdInternalCASecrets   = "get secret -A --field-selector metadata.name=nullus-internal-ca --no-headers -o custom-columns=NS:.metadata.namespace"
	cmdCRDs                = `get crd --no-headers -o custom-columns=NAME:.metadata.name,BY:.metadata.labels.nullus\.io/installed-by,RELEASE:.metadata.annotations.meta\.helm\.sh/release-name,RELEASE_NS:.metadata.annotations.meta\.helm\.sh/release-namespace,CREATED:.metadata.creationTimestamp`
	cmdCAInjection         = `get validatingwebhookconfigurations,mutatingwebhookconfigurations,apiservices,crd --no-headers -o custom-columns=KIND:.kind,NAME:.metadata.name,FROM:.metadata.annotations.cert-manager\.io/inject-ca-from,FROM_SECRET:.metadata.annotations.cert-manager\.io/inject-ca-from-secret,APISERVER_CA:.metadata.annotations.cert-manager\.io/inject-apiserver-ca,RELEASE:.metadata.annotations.meta\.helm\.sh/release-name`
	cmdAPIResources        = "api-resources --verbs=list --namespaced -o name"
	cmdCertManagerNSLeft   = "get configmaps,secrets,serviceaccounts,pods -n cert-manager -o name --ignore-not-found"
	cmdHPAs                = "get horizontalpodautoscalers -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name"
	cmdVPAs                = "get verticalpodautoscalers.autoscaling.k8s.io -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name"
	cmdGatewayClasses      = "get gatewayclasses.gateway.networking.k8s.io -o json"
	cmdGateways            = "get gateways.gateway.networking.k8s.io -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,CLASS:.spec.gatewayClassName"
	cmdEnvoyControllers    = "get deployments -A -l control-plane=envoy-gateway --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name"
	cmdEnvoyClassRemaining = "get gatewayclasses.gateway.networking.k8s.io envoy --ignore-not-found -o name"

	envoyController = "gateway.envoyproxy.io/gatewayclass-controller"
	crdCreated      = "2026-09-15T01:30:46Z"
)

func crdUsersCmd(crd string) string {
	return "get " + crd + " -A --no-headers -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,OWNER:.metadata.ownerReferences[0].kind"
}

// crdLine 은 cmdCRDs 출력 한 줄이다.
func crdLine(name, by, release, releaseNamespace, created string) string {
	return fmt.Sprintf("%s   %s   %s   %s   %s\n", name, by, release, releaseNamespace, created)
}

var certManagerCRDs = []string{
	"certificaterequests.cert-manager.io",
	"certificates.cert-manager.io",
	"challenges.acme.cert-manager.io",
	"clusterissuers.cert-manager.io",
	"issuers.cert-manager.io",
	"orders.acme.cert-manager.io",
}

// fakeSharedValues 는 공용 릴리스의 values 를 돌려준다(키: release@namespace).
type fakeSharedValues struct {
	values map[string]map[string]any
}

func (f *fakeSharedValues) ListReleases(context.Context, string) ([]port.ReleaseInfo, error) {
	return nil, nil
}

func (f *fakeSharedValues) GetValues(_ context.Context, releaseName, namespace string) (map[string]any, error) {
	values, ok := f.values[releaseName+"@"+namespace]
	if !ok {
		return nil, errors.New("release: not found")
	}
	return values, nil
}

func (f *fakeSharedValues) Upgrade(context.Context, port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	return nil, nil
}

func nullusCertManagerValues() map[string]any {
	return map[string]any{
		"global": map[string]any{"commonLabels": map[string]any{domain.LabelInstalledBy: domain.InstalledByNullus}},
	}
}

func nullusMetricsServerValues() map[string]any {
	return map[string]any{"commonLabels": map[string]any{domain.LabelInstalledBy: domain.InstalledByNullus}}
}

type sharedFixture struct {
	t         *testing.T
	rec       *kubectlRecorder
	repo      *fakeStackRepo
	installer *fakeHelmInstaller
	values    *fakeSharedValues
	streamer  *captureStreamer
	uc        *DeleteStack
}

func newSharedFixture(t *testing.T, namespace string, others ...*domain.Stack) *sharedFixture {
	t.Helper()
	f := &sharedFixture{
		t:         t,
		rec:       newKubectlRecorder(),
		installer: &fakeHelmInstaller{},
		values:    &fakeSharedValues{values: map[string]map[string]any{}},
		streamer:  &captureStreamer{},
	}
	f.repo = newFakeStackRepo(append([]*domain.Stack{{
		ID: sharedStackID, Name: sharedStackName, ClusterID: sharedClusterID, Namespace: namespace, State: domain.StateCompleted,
	}}, others...)...)
	f.uc = NewDeleteStack(f.repo,
		&fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")},
		func([]byte) port.HelmInstaller { return f.installer },
		f.streamer,
	)
	f.uc.runKubectlFunc = f.rec.run
	f.uc.SetReleaseManagerFactory(func([]byte) port.HelmReleaseManager { return f.values })
	f.uc.sharedWaitTimeout = 30 * time.Millisecond
	f.uc.sharedPollInterval = time.Millisecond
	return f
}

func (f *sharedFixture) execute() {
	f.t.Helper()
	require.NoError(f.t, f.uc.Execute(context.Background(), sharedStackID))
}

func (f *sharedFixture) messages(level string) string {
	var out []string
	for _, e := range f.streamer.entries {
		if e.Step == deleteStepShared && e.Level == level {
			out = append(out, e.Message)
		}
	}
	return strings.Join(out, "\n")
}

// withNullusCertManager 는 Nullus 가 cert-manager 네임스페이스에 깐 cert-manager 와 내부 CA 를 흉내 낸다.
func (f *sharedFixture) withNullusCertManager(values map[string]any) *sharedFixture {
	f.rec.stubs[cmdHelmCertManager] = "cert-manager\ncert-manager\n"
	f.values.values["cert-manager@cert-manager"] = values
	f.rec.stubs[cmdCertificates] = "cert-manager   nullus-internal-ca-cert   nullus-selfsigned-bootstrap\n"
	// 내부 CA 인증서가 만든 요청은 주인(Certificate)이 있어 쓰는 곳이 아니다.
	f.rec.stubs[cmdCertificateRequests] = "cert-manager   nullus-internal-ca-cert-1   nullus-selfsigned-bootstrap   Certificate\n"
	f.rec.stubs[cmdClusterIssuers] = "clusterissuer.cert-manager.io/nullus-internal-ca-issuer\n" +
		"clusterissuer.cert-manager.io/nullus-selfsigned-bootstrap\n"
	f.rec.stubs[cmdInternalCASecrets] = "cert-manager\n"
	f.rec.stubs[crdUsersCmd("clusterissuers.cert-manager.io")] = "<none>   nullus-internal-ca-issuer   <none>\n" +
		"<none>   nullus-selfsigned-bootstrap   <none>\n"
	f.rec.stubs[crdUsersCmd("certificates.cert-manager.io")] = "cert-manager   nullus-internal-ca-cert   <none>\n"
	f.rec.stubs[crdUsersCmd("certificaterequests.cert-manager.io")] = "cert-manager   nullus-internal-ca-cert-1   <none>   Certificate\n"
	f.rec.stubs[cmdCAInjection] = "ValidatingWebhookConfiguration   cert-manager-webhook   <none>   cert-manager/cert-manager-webhook-ca   <none>   cert-manager\n"
	f.rec.stubs[cmdAPIResources] = "configmaps\nsecrets\nserviceaccounts\npods\nevents\n"
	f.rec.stubs[cmdCertManagerNSLeft] = "configmap/kube-root-ca.crt\nsecret/cert-manager-webhook-ca\nserviceaccount/default\n"
	for _, crd := range certManagerCRDs {
		f.rec.stubs[cmdCRDs] += crdLine(crd, "nullus", "cert-manager", "cert-manager", crdCreated)
	}
	return f
}

func (f *sharedFixture) withNullusMetricsServer(values map[string]any) *sharedFixture {
	f.rec.stubs[cmdHelmMetrics] = "kube-system\n"
	f.values.values["metrics-server@kube-system"] = values
	return f
}

// gatewayClassJSON 은 cmdGatewayClasses 의 항목 하나다.
func gatewayClassJSON(name, controller, label, lastApplied, created string) string {
	labels := "{}"
	if label != "" {
		labels = fmt.Sprintf(`{%q:%q}`, domain.LabelInstalledBy, label)
	}
	annotations := "{}"
	if lastApplied != "" {
		annotations = fmt.Sprintf(`{"kubectl.kubernetes.io/last-applied-configuration":%q}`, lastApplied+"\n")
	}
	return fmt.Sprintf(`{"metadata":{"name":%q,"labels":%s,"annotations":%s,"creationTimestamp":%q},"spec":{"controllerName":%q}}`,
		name, labels, annotations, created, controller)
}

func (f *sharedFixture) withGatewayClasses(items ...string) *sharedFixture {
	f.rec.stubs[cmdGatewayClasses] = `{"items":[` + strings.Join(items, ",") + `]}`
	return f
}

func (f *sharedFixture) withCRDs(lines ...string) *sharedFixture {
	f.rec.stubs[cmdCRDs] += strings.Join(lines, "")
	return f
}

// --- cert-manager · 내부 CA ---

func TestDeleteStack_ReclaimsNullusCertManagerAndInternalCAOnLastStack(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())

	f.execute()

	// 스택 네임스페이스가 다 빠진 뒤에 본다.
	nsGone := f.rec.indexOf(cmdStackNamespace)
	issuers := f.rec.indexOf("delete clusterissuers.cert-manager.io nullus-internal-ca-issuer nullus-selfsigned-bootstrap")
	require.GreaterOrEqual(t, nsGone, 0)
	require.Greater(t, issuers, nsGone, "내부 CA 발급자를 지우지 않았다")
	assert.True(t, f.rec.has("delete certificates.cert-manager.io nullus-internal-ca-cert -n cert-manager"))
	assert.True(t, f.rec.has("delete secret nullus-internal-ca -n cert-manager"), "10년짜리 내부 CA 개인키가 남는다")

	assert.Contains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	crdDelete := f.rec.indexOf("delete crd certificaterequests.cert-manager.io certificates.cert-manager.io")
	require.Greater(t, crdDelete, issuers, "keep 정책이라 릴리스를 지워도 CRD 는 남는다 — 직접 지워야 한다")
	assert.True(t, f.rec.has("delete namespace cert-manager"))
	assert.True(t, f.rec.has("delete lease cert-manager-controller cert-manager-cainjector-leader-election -n kube-system"))
	assert.Empty(t, f.messages("warn"))
	assert.Contains(t, f.messages("info"), "모두 회수")
}

// 표시를 달기 전의 설치는 values 지문으로 Nullus 것을 알아본다.
func TestDeleteStack_ReclaimsLegacyCertManagerByValuesFingerprint(t *testing.T) {
	resources := map[string]any{"requests": map[string]any{"cpu": "500m"}}
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(map[string]any{
		"installCRDs": true,
		"resources":   resources,
		"webhook":     map[string]any{"resources": resources},
		"cainjector":  map[string]any{"resources": resources},
	})

	f.execute()

	assert.Contains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.True(t, f.rec.has("delete crd certificaterequests.cert-manager.io"))
}

// 설치가 재사용한 고객의 cert-manager 는 지우지 않는다. CRD 를 지우면 클러스터의 인증서가 전부 사라진다.
// 내부 CA 는 Nullus 가 만든 것이라 회수한다.
func TestDeleteStack_KeepsCertManagerNotInstalledByNullus(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(map[string]any{"crds": map[string]any{"enabled": true}})

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete crd", "cert-manager.io"))
	assert.False(t, f.rec.has("delete namespace cert-manager"))
	assert.True(t, f.rec.has("delete secret nullus-internal-ca -n cert-manager"))
	assert.Contains(t, f.messages("warn"), "Nullus 가 설치하지 않은 cert-manager")
}

// 릴리스가 둘인데 하나가 고객 것이면 CRD 를 함께 쓰므로 전부 남긴다.
func TestDeleteStack_KeepsCertManagerWhenAnyReleaseIsNotNullus(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[cmdHelmCertManager] = "cert-manager\nsecurity\n"
	f.values.values["cert-manager@security"] = map[string]any{"installCRDs": true}

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.Contains(t, f.messages("warn"), "네임스페이스 security")
}

// Nullus 가 깐 cert-manager 라도 사용자가 그것으로 인증서를 받고 있으면 남기고 무엇이 쓰는지 알린다.
func TestDeleteStack_KeepsCertManagerWhileUserResourcesUseIt(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[crdUsersCmd("certificates.cert-manager.io")] += "team-a   web-tls   <none>\n"
	f.rec.stubs[crdUsersCmd("certificaterequests.cert-manager.io")] += "team-a   web-tls-1   Certificate\n"

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete crd", "cert-manager.io"))
	warn := f.messages("warn")
	assert.Contains(t, warn, "team-a/certificates/web-tls")
	assert.NotContains(t, warn, "web-tls-1", "Certificate 가 만든 요청은 따로 세지 않는다")
}

// 스택 네임스페이스를 회수하지 않는 자리(default·플랫폼·사용자가 고른 이름)에 남은 것은 사용자 것이다.
// 예전에는 스택 네임스페이스를 무조건 빼고 세어, default 의 사용자 Issuer 가 있는데도 cert-manager CRD 를
// 지웠다 — Issuer 가 함께 사라진다.
func TestDeleteStack_CountsUsersInStackNamespaceThatIsNotReclaimed(t *testing.T) {
	f := newSharedFixture(t, "default").withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[crdUsersCmd("issuers.cert-manager.io")] = "default   letsencrypt   <none>\n"

	f.execute()

	assert.False(t, f.rec.has(cmdStackNamespace), "회수하지 않는 네임스페이스를 기다리지 않는다")
	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.Contains(t, f.messages("warn"), "default/issuers/letsencrypt")
}

// trust-manager 같은 구성요소는 cert-manager 위에서 돈다.
func TestDeleteStack_KeepsCertManagerWhileAnotherComponentDependsOnIt(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues()).
		withCRDs(crdLine("bundles.trust.cert-manager.io", "<none>", "trust-manager", "cert-manager", crdCreated))

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.Contains(t, f.messages("warn"), "bundles.trust.cert-manager.io")
}

// cainjector 로 CA 를 주입받는 웹훅·APIService 가 있으면 cert-manager 를 지울 때 그 CA 가 갱신되지 않는다.
func TestDeleteStack_KeepsCertManagerWhileCAInjectionIsUsed(t *testing.T) {
	cases := map[string]string{
		"inject-ca-from":      "MutatingWebhookConfiguration   kyverno-policy   kyverno/kyverno-ca   <none>   <none>   <none>\n",
		"inject-apiserver-ca": "MutatingWebhookConfiguration   kyverno-policy   <none>   <none>   true   <none>\n",
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
			f.rec.stubs[cmdCAInjection] += line

			f.execute()

			assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
			assert.Contains(t, f.messages("warn"), "mutatingwebhookconfiguration/kyverno-policy")
		})
	}
}

// 확인하지 못하면 남긴다. 실패 문구에 다른 API 의 discovery 오류가 섞여도 "종류가 없다" 로 읽지 않는다 —
// 그렇게 읽으면 쓰는 곳이 없다고 보고 CRD 를 지운다.
func TestDeleteStack_KeepsCertManagerWhenUsersCannotBeChecked(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.errs[crdUsersCmd("issuers.cert-manager.io")] = "couldn't get resource list for external.metrics.k8s.io/v1beta1: " +
		"the server could not find the requested resource\nError from server (Forbidden): issuers.cert-manager.io is forbidden"

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.Contains(t, f.messages("warn"), "issuers.cert-manager.io(조회 실패)")
}

// 내부 CA 로 인증서를 받는 곳이 있으면 CA 를 남긴다 — 지우면 그 인증서가 갱신되지 않는다. bootstrap 발급자는
// 평범한 selfSigned 발급자라 사용자가 가져다 쓰기 쉽다. 주인 없이 직접 만든 요청(istio-csr 등)도 쓰는 곳이다.
func TestDeleteStack_KeepsInternalCAWhileItIssuesCertificates(t *testing.T) {
	cases := map[string]func(*sharedFixture){
		"CA 발급자를 쓰는 인증서": func(f *sharedFixture) {
			f.rec.stubs[cmdCertificates] += "team-a   web-tls   nullus-internal-ca-issuer\n"
		},
		"bootstrap 발급자를 쓰는 인증서": func(f *sharedFixture) {
			f.rec.stubs[cmdCertificates] += "team-a   web-tls   nullus-selfsigned-bootstrap\n"
		},
		"주인 없는 인증서 요청(istio-csr)": func(f *sharedFixture) {
			f.rec.stubs[cmdCertificateRequests] += "istio-system   mesh-1   nullus-internal-ca-issuer   <none>\n"
		},
		"파드가 주인인 인증서 요청(csi-driver)": func(f *sharedFixture) {
			f.rec.stubs[cmdCertificateRequests] += "apps   web-7f9c   nullus-internal-ca-issuer   Pod\n"
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
			arrange(f)

			f.execute()

			assert.False(t, f.rec.has("delete clusterissuers.cert-manager.io"))
			assert.False(t, f.rec.has("delete secret nullus-internal-ca"))
			assert.Contains(t, f.messages("warn"), "내부 CA 를 남깁니다")
		})
	}
}

// 릴리스를 지우지 못했으면 CRD 를 지우지 않는다. 엉뚱한 클러스터를 보는 오류("context ... not found")를
// "이미 지웠다" 로 읽어도 안 된다.
func TestDeleteStack_DoesNotDeleteCertManagerCRDsWhenUninstallFails(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.installer.uninstallErr = errors.New(`kubernetes cluster unreachable: context "kind" not found`)

	f.execute()

	assert.False(t, f.rec.has("delete crd", "cert-manager.io"))
	assert.Contains(t, f.messages("error"), "cert-manager 릴리스(cert-manager)를 지우지 못했습니다")
}

// CRD 를 지우지 못했으면(Terminating 에 갇혔다) 네임스페이스와 Lease 를 지우지 않는다.
func TestDeleteStack_StopsCertManagerReclaimWhenCRDDeleteFails(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	crdDelete := "delete crd " + strings.Join(certManagerCRDs, " ") + " --ignore-not-found --timeout=120s"
	f.rec.errs[crdDelete] = "timed out waiting for the condition"

	f.execute()

	assert.True(t, f.rec.has(crdDelete))
	assert.False(t, f.rec.has("delete namespace cert-manager"))
	assert.False(t, f.rec.has("delete lease"))
	assert.Contains(t, f.messages("error"), "cert-manager CRD 를 지우지 못했습니다")
}

// 고객이 미리 만든 네임스페이스에 깔렸을 수 있다. 릴리스 몫이 아닌 것이 남아 있으면 네임스페이스는 남긴다.
func TestDeleteStack_KeepsCertManagerNamespaceWithOtherResources(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[cmdCertManagerNSLeft] += "secret/acme-account-key\n"

	f.execute()

	assert.Contains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete namespace cert-manager"))
	assert.Contains(t, f.messages("warn"), "secret/acme-account-key")
}

// 지운 워크로드의 파드는 잠깐 남는다. 다 빠지기를 기다리고, 끝내 남으면 네임스페이스를 남긴다.
func TestDeleteStack_WaitsForCertManagerPodsBeforeDeletingNamespace(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[cmdCertManagerNSLeft] += "pod/cert-manager-6db94c5b99-4kpdz\n"

	f.execute()

	assert.False(t, f.rec.has("delete namespace cert-manager"))
	assert.Contains(t, f.messages("warn"), "파드가 아직 빠지지 않아")
}

// 망가진 aggregated API(KEDA 등)가 있으면 api-resources 는 나머지 목록을 내고도 실패한다. 그 목록으로
// 세어 네임스페이스를 회수한다 — 그러지 않으면 그런 클러스터에서는 cert-manager 네임스페이스가 늘 남는다.
func TestDeleteStack_DeletesCertManagerNamespaceDespitePartialDiscovery(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.errs[cmdAPIResources] = "error: unable to retrieve the complete list of server APIs: external.metrics.k8s.io/v1beta1: " +
		"the server is currently unable to handle the request"

	f.execute()

	assert.True(t, f.rec.has(cmdCertManagerNSLeft))
	assert.True(t, f.rec.has("delete namespace cert-manager"))
}

// --- 스택·네임스페이스 ---

// 클러스터는 여러 스택이 쓸 수 있다(설치에 실패해 기록만 남은 스택 포함). 다른 스택이 있으면 공용 자원은
// 그 스택 것이기도 하다.
func TestDeleteStack_KeepsSharedResourcesWhileAnotherStackUsesCluster(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace, &domain.Stack{
		ID: "stk-other", Name: "other-stack", ClusterID: sharedClusterID, Namespace: "nullus-other", State: domain.StateFailed,
	}).withNullusCertManager(nullusCertManagerValues())

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete clusterissuers.cert-manager.io"))
	assert.False(t, f.rec.has("delete crd", "cert-manager.io"))
	assert.Contains(t, f.messages("warn"), "other-stack")
}

// 회수는 몇 분 걸린다. 그 사이 새 스택이 생기면 그 스택이 공용 자원을 재사용하므로 거기서 멈춘다.
// appear 가 참이 된 뒤부터 새 스택이 보인다.
type stackAppearsRepo struct {
	*fakeStackRepo
	appear func() bool
}

func (r *stackAppearsRepo) ListByCluster(ctx context.Context, clusterID string) ([]*domain.Stack, error) {
	if r.appear() {
		return []*domain.Stack{{ID: "stk-new", Name: "new-stack", ClusterID: clusterID}}, nil
	}
	return r.fakeStackRepo.ListByCluster(ctx, clusterID)
}

func TestDeleteStack_StopsReclaimWhenAStackAppearsMidway(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	// 내부 CA 를 다 지운 뒤 새 스택이 보인다.
	f.uc.stackRepo = &stackAppearsRepo{fakeStackRepo: f.repo, appear: func() bool {
		return f.rec.has("delete secret nullus-internal-ca")
	}}

	f.execute()

	assert.True(t, f.rec.has("delete secret nullus-internal-ca"))
	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	warn := f.messages("warn")
	assert.Contains(t, warn, "new-stack")
	assert.Equal(t, 1, strings.Count(warn, "new-stack"), "멈춘 이유는 한 번만 적는다")
}

// 단계 안에서도 쓰는 곳을 확인한 뒤 지우기까지 시간이 걸린다. 지우기 직전에 다시 본다.
func TestDeleteStack_ChecksForNewStackRightBeforeDestructiveSteps(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	// cert-manager 단계의 마지막 확인(cainjector 주입) 뒤, 릴리스를 지우기 전에 새 스택이 보인다.
	f.uc.stackRepo = &stackAppearsRepo{fakeStackRepo: f.repo, appear: func() bool {
		return f.rec.has("get validatingwebhookconfigurations")
	}}

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete crd", "cert-manager.io"))
	assert.Contains(t, f.messages("warn"), "new-stack")
}

type failingClusterListRepo struct{ *fakeStackRepo }

func (r failingClusterListRepo) ListByCluster(context.Context, string) ([]*domain.Stack, error) {
	return nil, errors.New("db down")
}

func TestDeleteStack_KeepsSharedResourcesWhenOtherStacksCannotBeChecked(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.uc.stackRepo = failingClusterListRepo{f.repo}

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete clusterissuers.cert-manager.io"))
	assert.Contains(t, f.messages("warn"), "db down")
}

// 스택 네임스페이스가 다 빠지기 전에 컨트롤러를 지우면 그 안의 finalizer 를 풀 주체가 사라진다.
func TestDeleteStack_KeepsSharedResourcesWhileStackNamespaceTerminates(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.rec.stubs[cmdStackNamespace] = "namespace/" + sharedNamespace + "\n"

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.False(t, f.rec.has("delete clusterissuers.cert-manager.io"))
	assert.Contains(t, f.messages("warn"), "아직 지워지는 중")
}

func TestDeleteStack_KeepsSharedReleasesWithoutValuesReader(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusCertManager(nullusCertManagerValues())
	f.uc.SetReleaseManagerFactory(nil)

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@cert-manager")
	assert.Contains(t, f.messages("warn"), "릴리스 values 를 읽을 수 없어 cert-manager 를 남깁니다")
}

// 스택 네임스페이스를 통째로 회수하면 그 안의 공용 릴리스는 스택과 함께 지운다(APIService 가 남지 않게).
// default 에서는 누가 깔았는지 모르므로 지우지 않는다.
func TestDeleteStack_UninstallsSharedReleasesOnlyInReclaimedStackNamespace(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace)

	f.execute()

	assert.Contains(t, f.installer.uninstallCalls, "metrics-server@"+sharedNamespace)
	assert.NotContains(t, f.installer.uninstallCalls, "metrics-server@default")
	assert.NotContains(t, f.installer.uninstallCalls, "cert-manager@default")
}

// --- metrics-server ---

func TestDeleteStack_ReclaimsNullusMetricsServer(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusMetricsServer(nullusMetricsServerValues())

	f.execute()

	assert.Contains(t, f.installer.uninstallCalls, "metrics-server@kube-system")
	assert.False(t, f.rec.has("delete namespace kube-system"), "kube-system 은 지우지 않는다")
}

// 사용자 오토스케일러가 메트릭을 쓰고 있으면 metrics-server 를 지울 때 오토스케일이 멈춘다.
func TestDeleteStack_KeepsMetricsServerWhileAutoscalersUseIt(t *testing.T) {
	cases := map[string]func(*sharedFixture){
		"HPA": func(f *sharedFixture) { f.rec.stubs[cmdHPAs] = "shop   api\n" },
		"VPA": func(f *sharedFixture) {
			f.withCRDs(crdLine("verticalpodautoscalers.autoscaling.k8s.io", "<none>", "vpa", "kube-system", crdCreated))
			f.rec.stubs[cmdVPAs] = "shop   api\n"
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSharedFixture(t, sharedNamespace).withNullusMetricsServer(nullusMetricsServerValues())
			arrange(f)

			f.execute()

			assert.NotContains(t, f.installer.uninstallCalls, "metrics-server@kube-system")
			assert.Contains(t, f.messages("warn"), "shop/api")
		})
	}
}

func TestDeleteStack_KeepsMetricsServerNotInstalledByNullus(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).withNullusMetricsServer(map[string]any{"args": []any{"--kubelet-insecure-tls"}})

	f.execute()

	assert.NotContains(t, f.installer.uninstallCalls, "metrics-server@kube-system")
	assert.Contains(t, f.messages("warn"), "Nullus 가 설치하지 않은 metrics-server")
}

// --- Gateway API ---

var gatewayCRDNamesForTest = []string{
	"backends.gateway.envoyproxy.io",
	"gatewayclasses.gateway.networking.k8s.io",
	"gateways.gateway.networking.k8s.io",
	"xlistenersets.gateway.networking.x-k8s.io",
}

// 설치기가 만든 GatewayClass 가 남아 있으면 Gateway API 를 누가 쓰는 것으로 보여 CRD 가 영영 지워지지
// 않았다. 자기 GatewayClass 를 먼저 지우고, 표시된 CRD 만 지운다 — 원래 있던 CRD 는 남긴다.
func TestDeleteStack_ReclaimsMarkedGatewayClassAndCRDs(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "nullus", "", "2026-10-01T00:00:00Z")).
		withCRDs(
			crdLine("gatewayclasses.gateway.networking.k8s.io", "<none>", "<none>", "<none>", crdCreated),
			crdLine("gateways.gateway.networking.k8s.io", "<none>", "<none>", "<none>", crdCreated),
			crdLine("backends.gateway.envoyproxy.io", "nullus", "<none>", "<none>", crdCreated),
			crdLine("xlistenersets.gateway.networking.x-k8s.io", "nullus", "<none>", "<none>", crdCreated),
		)

	f.execute()

	classDelete := f.rec.indexOf("delete gatewayclasses.gateway.networking.k8s.io envoy")
	crdDelete := f.rec.indexOf("delete crd backends.gateway.envoyproxy.io xlistenersets.gateway.networking.x-k8s.io --ignore-not-found")
	require.GreaterOrEqual(t, classDelete, 0)
	require.Greater(t, crdDelete, classDelete)
	assert.False(t, f.rec.has("delete crd", "gateways.gateway.networking.k8s.io"), "표시 없는 CRD 는 Nullus 가 설치하지 않았다")
}

// 컨트롤러가 사라져 GatewayClass 의 finalizer 를 풀 주체가 없으면 걷어 낸다.
func TestDeleteStack_ClearsFinalizerOfOwnGatewayClass(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "nullus", "", "2026-10-01T00:00:00Z")).
		withCRDs(crdLine("gatewayclasses.gateway.networking.k8s.io", "nullus", "<none>", "<none>", crdCreated))
	f.rec.stubs[cmdEnvoyClassRemaining] = "gatewayclass.gateway.networking.k8s.io/envoy\n"

	f.execute()

	assert.True(t, f.rec.has("patch gatewayclasses.gateway.networking.k8s.io envoy --type=merge"))
}

// 표시를 달기 전의 설치: GatewayClass envoy 가 옛 설치기가 apply 한 모양 그대로 남아 있다. 그 무렵 생긴,
// Helm 이 소유하지 않은 Gateway CRD 를 Nullus 것으로 본다. 다른 차트가 소유한 CRD 나 한참 먼저 있던 CRD
// (고객이 미리 깐 것)는 남긴다.
func TestDeleteStack_ReclaimsLegacyGatewayCRDs(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "", legacyEnvoyGatewayClassManifest, "2026-09-15T01:31:17Z")).
		withCRDs(
			crdLine("backends.gateway.envoyproxy.io", "<none>", "<none>", "<none>", "2026-09-15T01:30:50Z"),
			crdLine("gatewayclasses.gateway.networking.k8s.io", "<none>", "<none>", "<none>", "2026-09-15T01:30:50Z"),
			crdLine("gateways.gateway.networking.k8s.io", "<none>", "<none>", "<none>", crdCreated),
			crdLine("grpcroutes.gateway.networking.k8s.io", "<none>", "istio-base", "istio-system", crdCreated),
			crdLine("tlsroutes.gateway.networking.k8s.io", "<none>", "<none>", "<none>", "2026-03-02T10:00:00Z"),
		)

	f.execute()

	assert.True(t, f.rec.has("delete gatewayclasses.gateway.networking.k8s.io envoy"))
	assert.True(t, f.rec.has("delete crd backends.gateway.envoyproxy.io gatewayclasses.gateway.networking.k8s.io "+
		"gateways.gateway.networking.k8s.io --ignore-not-found"))
	assert.False(t, f.rec.has("delete crd", "grpcroutes"))
	assert.False(t, f.rec.has("delete crd", "tlsroutes"))
}

// 이름과 컨트롤러만 같은 고객의 GatewayClass(차트나 다른 매니페스트로 만든 것)는 Nullus 것이 아니다.
func TestDeleteStack_KeepsUnmarkedEnvoyGatewayClassNotFromOldInstaller(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "", "", "2026-09-15T01:31:17Z")).
		withCRDs(
			crdLine("gatewayclasses.gateway.networking.k8s.io", "<none>", "<none>", "<none>", crdCreated),
			crdLine("gateways.gateway.networking.k8s.io", "<none>", "<none>", "<none>", crdCreated),
		)

	f.execute()

	assert.True(t, f.rec.has(cmdGatewayClasses))
	assert.False(t, f.rec.has("delete gatewayclasses.gateway.networking.k8s.io envoy"))
	assert.False(t, f.rec.has("delete crd", "gateway"))
}

// 다른 곳에서 Envoy Gateway 컨트롤러가 돌면 GatewayClass envoy 와 Envoy CRD 를 쓸 수 있다. Gateway 가 아직
// 하나도 없어도 CRD 를 지우면 그 컨트롤러가 깨진다.
func TestDeleteStack_KeepsGatewayWhileAnotherEnvoyControllerRuns(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "", legacyEnvoyGatewayClassManifest, "2026-09-15T01:31:17Z")).
		withCRDs(
			crdLine("backends.gateway.envoyproxy.io", "<none>", "<none>", "<none>", "2026-09-15T01:30:50Z"),
			crdLine("gatewayclasses.gateway.networking.k8s.io", "<none>", "<none>", "<none>", "2026-09-15T01:30:50Z"),
		)
	f.rec.stubs[cmdEnvoyControllers] = "envoy-gateway-system   envoy-gateway\n"

	f.execute()

	assert.False(t, f.rec.has("delete gatewayclasses.gateway.networking.k8s.io envoy"))
	assert.False(t, f.rec.has("patch gatewayclasses"))
	assert.False(t, f.rec.has("delete crd", "gateway"))
	assert.Contains(t, f.messages("warn"), "envoy-gateway-system/envoy-gateway")
}

// 다른 GatewayClass(Istio 등)가 있으면 누군가 Gateway API 를 쓰고 있다. CRD 를 지우면 그쪽이 깨진다.
func TestDeleteStack_KeepsGatewayCRDsWhileForeignGatewayClassExists(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace).
		withGatewayClasses(
			gatewayClassJSON("envoy", envoyController, "nullus", "", "2026-10-01T00:00:00Z"),
			gatewayClassJSON("istio", "istio.io/gateway-controller", "", "", "2026-10-01T00:00:00Z"),
		).
		withCRDs(crdLine("gatewayclasses.gateway.networking.k8s.io", "nullus", "<none>", "<none>", crdCreated))
	f.rec.stubs[crdUsersCmd("gatewayclasses.gateway.networking.k8s.io")] = "<none>   envoy   <none>\n<none>   istio   <none>\n"

	f.execute()

	assert.True(t, f.rec.has("delete gatewayclasses.gateway.networking.k8s.io envoy"))
	assert.False(t, f.rec.has("delete crd", "gateway"))
	warn := f.messages("warn")
	assert.Contains(t, warn, "gatewayclasses/istio")
	assert.NotContains(t, warn, "gatewayclasses/envoy", "방금 지운 자기 GatewayClass 는 쓰는 곳이 아니다")
}

// Gateway 가 GatewayClass envoy 를 쓰면 남긴다. 회수하지 않는 스택 네임스페이스(사용자가 고른 이름)에 남은
// Gateway 도 사용자 것이다.
func TestDeleteStack_KeepsGatewayClassWhileGatewaysUseIt(t *testing.T) {
	f := newSharedFixture(t, "cicd-stack").
		withGatewayClasses(gatewayClassJSON("envoy", envoyController, "nullus", "", "2026-10-01T00:00:00Z")).
		withCRDs(
			crdLine("gatewayclasses.gateway.networking.k8s.io", "nullus", "<none>", "<none>", crdCreated),
			crdLine("gateways.gateway.networking.k8s.io", "nullus", "<none>", "<none>", crdCreated),
		)
	f.rec.stubs[cmdGateways] = "cicd-stack   edge   envoy\n"

	f.execute()

	assert.False(t, f.rec.has("delete gatewayclasses.gateway.networking.k8s.io envoy"))
	assert.False(t, f.rec.has("delete crd", "gateway"))
	assert.Contains(t, f.messages("warn"), "cicd-stack/edge")
}

// 클러스터에 cert-manager·Gateway API 가 아예 없으면(CRD 없음) 조용히 넘어간다.
func TestDeleteStack_SharedReclaimIsQuietWhenNothingInstalled(t *testing.T) {
	f := newSharedFixture(t, sharedNamespace)

	f.execute()

	assert.Empty(t, f.messages("warn"))
	assert.Empty(t, f.messages("error"))
	assert.Contains(t, f.messages("info"), "회수할 클러스터 공용 자원이 없습니다")
	assert.False(t, f.rec.has(cmdGatewayClasses), "CRD 가 없으면 조회하지 않는다")
}

// kubectl 은 표준 오류를 함께 읽는다(runKubectlWithKubeconfig). 죽은 APIService 의 discovery 오류 같은
// 줄을 이름으로 읽으면 없는 볼륨이 남은 것으로 보인다 — 리소스 참조에는 공백이 없다.
func TestParseResourceNames_IgnoresStderrNoticeLines(t *testing.T) {
	out := `E1006 09:26:46.326775 memcache.go:265] "Unhandled Error" err="couldn't get resource list for metrics.k8s.io/v1beta1"
persistentvolumeclaim/data-nullus-postgresql-0
No resources found in nullus-demo namespace.
`
	assert.Equal(t, []string{"data-nullus-postgresql-0"}, parseResourceNames(out))
}
