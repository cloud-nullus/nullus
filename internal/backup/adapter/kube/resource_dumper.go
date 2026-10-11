package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cloud-nullus/draft/internal/backup/port"
)

// ResourceDumper 는 네임스페이스 리소스와 Helm 릴리스를 뜬다 (D1).
//
// kubectl 을 exec 하는 이유: 스택은 Gateway/HTTPRoute 같은 CRD 를 만든다.
// 타입 클라이언트로 알려진 종류만 훑으면 카탈로그가 늘 때마다 백업이
// 뒤처진다 — §1.5 의 PVC 열거와 같은 이유다.
//
// kubectl 은 API 이미지에 이미 있다 (Dockerfile).
type ResourceDumper struct {
	kubeconfig []byte
	// exec 는 kubectl 호출이다. nil 이면 실제 kubectl 을 실행한다 — 테스트가 가짜를 끼운다.
	exec kubectlRunner
}

// kubectlRunner 는 kubectl 한 번의 실행이다(stdin → stdout).
type kubectlRunner func(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error

func NewResourceDumper(kubeconfig []byte) *ResourceDumper {
	return &ResourceDumper{kubeconfig: kubeconfig}
}

// dumpKinds 는 뜰 리소스 종류다.
//
// Helm 릴리스는 Secret 에 들어 있으므로(sh.helm.release.v1.*) secret 을 뜨면
// 함께 담긴다 — 릴리스 메타데이터를 따로 다루지 않아도 되는 이유다.
// PVC 가 목록에 없는 것은 의도다.
//
// PVC 는 **볼륨 경로가 소유한다** — 복구는 매니페스트의 크기·StorageClass 로
// EnsurePVC 가 먼저 만들고 그 위에 아카이브를 푼다(§6.1 3~4단계). 리소스
// 덤프에도 PVC 가 들어 있으면 같은 객체를 두 경로가 소유하게 되고, 뒤따르는
// apply 가 `spec.volumeName`(이미 사라진 PV 를 가리킨다) 때문에 "spec is
// immutable" 로 죽는다 — 리허설에서 실제로 그렇게 됐다.
//
// 소유자는 하나여야 한다.
var dumpKinds = []string{
	"deployments", "statefulsets", "daemonsets", "services", "configmaps",
	"secrets", "serviceaccounts", "roles",
	"rolebindings", "ingresses", "gateways.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",

	// ESO 의 시크릿 배선. **빠뜨리면 복구가 성공했다고 보고하고도 스택이
	// 뜨지 않는다** — 실환경 리허설에서 실제로 그랬다.
	//
	// ESO 가 만든 Secret 은 ownerReferences 때문에 skipResource 가 건너뛴다
	// (소유자가 다시 만들 것이므로 그게 맞다). 그런데 그 소유자인 CR 까지
	// 빠지면 다시 만들 주체가 사라진다. 결과는 Gitea·Harbor·Jenkins 가
	// CreateContainerConfigError 로 멈춘 채 남는 것이다.
	//
	// 값 자체는 금고(OpenBao)가 SoT 다(§B1). 여기서 되살리는 것은 **배선**이고,
	// 값은 KV import 가 되돌린 금고에서 ESO 가 다시 끌어온다.
	"externalsecrets.external-secrets.io",
	"secretstores.external-secrets.io",
	"pushsecrets.external-secrets.io",

	// 배포의 주체. 앱의 Deployment 는 **Git 에서 파생된다** — Argo CD 가
	// 매니페스트를 읽어 만든다. Deployment 만 되돌리면 백업 시점의 이미지
	// 태그가 그대로 굳고, 그것을 다시 맞춰 줄 주체가 없다.
	//
	// 실환경 리허설에서 복구 뒤 Application 이 하나도 없었고, 되살아난 앱
	// Deployment 는 스캐폴드 초기값 :bootstrap 을 가리킨 채 남았다 — 그 태그는
	// 레지스트리에 존재한 적이 없다(renderer.go 의 InitialImageTag). CI 가
	// 다시 돌기 전에는 아무도 고쳐 주지 않는다.
	//
	// delete_stack.go 의 argoCDCRDNames 가 이 셋을 이미 알고 있었다 — 지우는
	// 쪽은 알고 백업하는 쪽은 몰랐다.
	"applications.argoproj.io",
	"appprojects.argoproj.io",
	"applicationsets.argoproj.io",

	// 모니터링의 주체. kube-prometheus-stack 의 Prometheus · Alertmanager StatefulSet 은
	// **오퍼레이터가 CR 을 읽어 만든다** — StatefulSet 은 ownerReferences 때문에 덤프에서
	// 빠지고(소유자가 다시 만든다), 소유자인 CR 까지 빠지면 다시 만들 주체가 없다.
	//
	// 2026-10-12 제품 삭제 뒤 복구 리허설에서 복구는 succeeded 인데 두 StatefulSet 이 영영
	// 돌아오지 않았고, 워크로드 재개 단계가 "statefulsets … not found" 를 남겼다.
	// ServiceMonitor · PrometheusRule 이 빠지면 스크랩 대상과 알림 규칙이 사라져 Grafana 가
	// 비어 보인다. Application · ExternalSecret 과 같은 뿌리다.
	"prometheuses.monitoring.coreos.com",
	"alertmanagers.monitoring.coreos.com",
	"servicemonitors.monitoring.coreos.com",
	"podmonitors.monitoring.coreos.com",
	"prometheusrules.monitoring.coreos.com",
	"probes.monitoring.coreos.com",
	"scrapeconfigs.monitoring.coreos.com",
	"alertmanagerconfigs.monitoring.coreos.com",
}

func (d *ResourceDumper) writeKubeconfig() (string, error) {
	f, err := os.CreateTemp("", "nullus-backup-kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(d.kubeconfig); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

func (d *ResourceDumper) run(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	if d.exec != nil {
		return d.exec(ctx, stdin, stdout, args...)
	}
	return d.runKubectl(ctx, stdin, stdout, args...)
}

func (d *ResourceDumper) runKubectl(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	path, err := d.writeKubeconfig()
	if err != nil {
		return fmt.Errorf("kubeconfig 임시 파일: %w", err)
	}
	defer func() { _ = os.Remove(path) }()

	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--kubeconfig", path}, args...)...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// availableKinds 는 대상 클러스터에 실제로 존재하는 종류만 남긴다.
//
// `--ignore-not-found` 는 없는 **객체**를 다루지 없는 **리소스 타입**은
// 다루지 못한다. Gateway API CRD 가 없는 클러스터에서 `kubectl get
// gateways...` 는 exit 1 로 죽고, 그러면 네임스페이스 덤프 전체가 실패한다 —
// 리허설에서 실제로 그렇게 됐다. 스택 구성에 따라 Gateway API 가 없을 수
// 있으므로, 무엇이 있는지 먼저 물어본다.
func (d *ResourceDumper) availableKinds(ctx context.Context) ([]string, error) {
	var buf strings.Builder
	if err := d.run(ctx, nil, &buf, "api-resources", "--verbs=list", "--namespaced=true", "-o", "name"); err != nil {
		return nil, fmt.Errorf("리소스 종류 조회: %w", err)
	}
	return filterAvailable(dumpKinds, buf.String()), nil
}

// filterAvailable 은 `kubectl api-resources -o name` 출력과 대조해 실제로
// 존재하는 종류만 남긴다.
//
// kubectl 호출과 분리해 둔 이유: 이 판정이 틀리면 네임스페이스 덤프가 통째로
// 실패하거나(없는 종류를 조회) 조용히 빠진다(있는 종류를 누락). 둘 다 비싸고,
// 클러스터 없이 검증할 수 있어야 한다.
func filterAvailable(want []string, apiResourcesOutput string) []string {
	present := map[string]bool{}
	for _, line := range strings.Split(apiResourcesOutput, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		present[name] = true
		// `deployments.apps` 처럼 그룹이 붙어 오므로 짧은 이름도 넣는다.
		if i := strings.Index(name, "."); i > 0 {
			present[name[:i]] = true
		}
	}

	out := make([]string, 0, len(want))
	for _, k := range want {
		if present[k] {
			out = append(out, k)
			continue
		}
		if i := strings.Index(k, "."); i > 0 && present[k[:i]] {
			out = append(out, k)
		}
	}
	return out
}

// clusterScopedKinds 는 스택이 만든 **클러스터 범위** 리소스다.
//
// 네임스페이스 리소스만 뜨면 복구가 깨진다 — CRD 가 없는 상태에서
// ExternalSecret/SecretStore 를 apply 하면 "no matches for kind" 로 죽는다.
// 실환경 리허설에서 실제로 그랬다.
//
// 스택이 소유한 것만 고른다(Helm 의 release-namespace 어노테이션). 클러스터
// 범위라 다른 것까지 건드리면 클러스터 전체에 영향을 준다.
var clusterScopedKinds = []string{
	"customresourcedefinitions.apiextensions.k8s.io",

	// 컨트롤러의 권한. **CRD 와 CR 만 되돌리면 모자란다** — 컨트롤러가 자기
	// CR 을 볼 권한을 잃어 조정이 아예 시작되지 않는다. 실환경 리허설에서
	// ESO 가 그렇게 멈췄다:
	//
	//   externalsecrets.external-secrets.io is forbidden: User
	//   "system:serviceaccount:nullus-app:external-secrets" cannot list
	//   resource "externalsecrets" ... at the cluster scope
	//
	// ExternalSecret/SecretStore 는 멀쩡히 복원됐는데 status 가 끝내 비어
	// 있었고, 증상은 결함 ③ 과 구별되지 않았다 — 복구는 succeeded 인데
	// Gitea·Harbor·Jenkins 가 CreateContainerConfigError 로 멈춘 채 남는다.
	//
	// 네임스페이스의 role/rolebinding 만으로는 대체할 수 없다. 컨트롤러가
	// 클러스터 범위로 list/watch 하기 때문이다.
	"clusterroles.rbac.authorization.k8s.io",
	"clusterrolebindings.rbac.authorization.k8s.io",
}

// dumpClusterScoped 는 대상 네임스페이스의 Helm 릴리스가 소유한 클러스터 범위
// 리소스를 뜬다.
func (d *ResourceDumper) dumpClusterScoped(ctx context.Context, namespace string) ([]byte, error) {
	var avail strings.Builder
	if err := d.run(ctx, nil, &avail, "api-resources", "--verbs=list", "--namespaced=false", "-o", "name"); err != nil {
		return nil, fmt.Errorf("클러스터 범위 리소스 종류 조회: %w", err)
	}
	kinds := filterAvailable(clusterScopedKinds, avail.String())
	if len(kinds) == 0 {
		return nil, nil
	}

	var raw bytes.Buffer
	if err := d.run(ctx, nil, &raw,
		"get", strings.Join(kinds, ","), "-o", "json", "--ignore-not-found"); err != nil {
		return nil, err
	}
	if raw.Len() == 0 {
		return nil, nil
	}
	return sanitizeOwnedBy(raw.Bytes(), namespace)
}

// sanitizeOwnedBy 는 지정한 네임스페이스의 Helm 릴리스가 소유한 것만 남긴다.
func sanitizeOwnedBy(raw []byte, namespace string) ([]byte, error) {
	var list struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("클러스터 범위 리소스 해석: %w", err)
	}
	kept := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		meta, _ := item["metadata"].(map[string]any)
		ann, _ := meta["annotations"].(map[string]any)
		if ns, _ := ann["meta.helm.sh/release-namespace"].(string); ns != namespace {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == 0 {
		return nil, nil
	}
	cleanedRaw, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": kept})
	return sanitize(cleanedRaw)
}

func (d *ResourceDumper) Dump(ctx context.Context, namespace string, out io.Writer) (int64, error) {
	kinds, err := d.availableKinds(ctx)
	if err != nil {
		return 0, err
	}
	if len(kinds) == 0 {
		return 0, fmt.Errorf("덤프할 리소스 종류를 하나도 찾지 못했습니다 — 클러스터 접근을 확인하세요")
	}

	var raw bytes.Buffer
	args := []string{
		"get", strings.Join(kinds, ","),
		"-n", namespace, "-o", "json", "--ignore-not-found",
	}
	if err := d.run(ctx, nil, &raw, args...); err != nil {
		return 0, err
	}
	if raw.Len() == 0 {
		return 0, nil
	}

	cleaned, err := sanitize(raw.Bytes())
	if err != nil {
		return 0, err
	}

	// 클러스터 범위가 **먼저** 온다. CRD 가 없으면 그 CR 을 apply 할 수 없다.
	clusterScoped, err := d.dumpClusterScoped(ctx, namespace)
	if err != nil {
		return 0, err
	}

	doc := map[string]any{}
	if len(clusterScoped) > 0 {
		doc["cluster_scoped"] = json.RawMessage(clusterScoped)
	}
	doc["namespaced"] = json.RawMessage(cleaned)

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return 0, err
	}
	counter := &countingWriter{w: out}
	_, err = counter.Write(payload)
	return counter.n, err
}

// sanitize 는 서버가 관리하는 필드를 걷어낸다.
//
// `kubectl get -o json` 은 resourceVersion·uid·status·managedFields 를 그대로
// 담는다. 그것을 되돌리면 "the object has been modified" 로 apply 가 깨진다 —
// 리허설에서 실제로 그렇게 됐다. 되살릴 것은 **의도(spec)** 이지 그때의
// 서버 상태가 아니다.
func sanitize(raw []byte) ([]byte, error) {
	var list struct {
		APIVersion string           `json:"apiVersion"`
		Kind       string           `json:"kind"`
		Items      []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("리소스 목록 해석: %w", err)
	}

	kept := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		if skipResource(item) {
			continue
		}
		delete(item, "status")

		meta, _ := item["metadata"].(map[string]any)
		for _, f := range []string{
			"resourceVersion", "uid", "creationTimestamp", "generation",
			"managedFields", "selfLink", "ownerReferences", "finalizers",
		} {
			delete(meta, f)
		}

		// Service 의 clusterIP 는 남긴다. 설치가 스택 게이트웨이의 ClusterIP 를 러너·Argo CD 의
		// hostAliases 에 박는다 — 복구가 IP 를 버리면 게이트웨이는 새 주소를 받는데 hostAliases 는
		// 옛 주소를 가리켜 파이프라인이 "Could not resolve host" 로 멈춘다. headless(None)는 IP 가
		// 아니라 정체라 더욱 지우면 안 된다(StatefulSet 의 파드별 DNS). 그 IP 를 쓸 수 없으면
		// (이미 쓰임·범위 밖) Apply 가 새 주소로 되돌린다.
		kept = append(kept, item)
	}

	out := map[string]any{"apiVersion": "v1", "kind": "List", "items": kept}
	return json.MarshalIndent(out, "", "  ")
}

// skipResource 는 되돌리면 안 되는 것을 걸러낸다.
func skipResource(item map[string]any) bool {
	meta, _ := item["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	kind, _ := item["kind"].(string)

	// 컨트롤러가 소유한 것은 소유자가 다시 만든다. 먼저 되돌리면 소유자와
	// 다투게 된다.
	if refs, ok := meta["ownerReferences"].([]any); ok && len(refs) > 0 {
		return true
	}

	// 쿠버네티스가 네임스페이스마다 자동으로 만드는 것들.
	switch {
	case kind == "PersistentVolumeClaim":
		// 볼륨 경로가 소유한다 (dumpKinds 주석 참고). 다른 경로로 흘러들어와도
		// 여기서 막는다.
		return true
	case kind == "ServiceAccount" && name == "default":
		return true
	case kind == "ConfigMap" && name == "kube-root-ca.crt":
		return true
	case kind == "Secret":
		// 서비스 계정 토큰은 자동 발급된다. 옛 토큰을 되돌리면 무효한 값이
		// 유효한 것처럼 남는다.
		if t, _ := item["type"].(string); t == "kubernetes.io/service-account-token" {
			return true
		}
	}
	return false
}

// Apply 는 뜬 리소스를 되돌린다.
//
// --server-side 를 쓰는 이유: 덤프에는 resourceVersion·uid 같은 서버 관리
// 필드가 섞여 있다. 클라이언트 사이드 apply 는 그것들 때문에 충돌한다.
// Apply 는 클러스터 범위를 먼저, 그 다음 네임스페이스 리소스를 되돌린다.
//
// 순서가 핵심이다. CRD 가 등록되기 전에 그 CR 을 apply 하면
// "no matches for kind" 로 죽는다 — 실환경 리허설에서 ESO 의
// SecretStore/ExternalSecret 이 그렇게 실패했다.
func (d *ResourceDumper) Apply(ctx context.Context, namespace string, in io.Reader) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	var doc struct {
		ClusterScoped json.RawMessage `json:"cluster_scoped"`
		Namespaced    json.RawMessage `json:"namespaced"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Namespaced) == 0 {
		// 옛 형식(단일 목록)도 받아 준다 — 이전 백업본을 복원할 수 있어야 한다.
		return d.applyNamespaced(ctx, namespace, raw)
	}

	if len(doc.ClusterScoped) > 0 {
		if err := d.applyDoc(ctx, namespace, doc.ClusterScoped, true); err != nil {
			return fmt.Errorf("클러스터 범위 리소스 복원: %w", err)
		}
		// CRD 등록이 API 서버에 반영될 때까지 기다린다. 바로 CR 을 밀면
		// 아직 discovery 에 없어 같은 오류가 난다.
		if err := d.run(ctx, nil, io.Discard, "wait", "--for=condition=established",
			"--timeout=120s", "crd", "--all"); err != nil {
			// established 를 못 봐도 계속 간다 — 이미 있던 CRD 일 수 있다.
			_ = err
		}
	}
	return d.applyNamespaced(ctx, namespace, doc.Namespaced)
}

// applyNamespaced 는 네임스페이스 리소스를 되돌린다. Service 는 백업 때의 ClusterIP 로 먼저 세운다.
//
// 그 IP 를 쓸 수 없으면(다른 Service 가 이미 씀·다른 클러스터라 범위 밖·이미 있는 Service 의 IP 와
// 다름) API 서버가 spec.clusterIP 로 거절한다. 복구를 멈추지 않고 IP 를 빼고 다시 적용한다 — 새
// 주소를 받은 Service 는 경고로 남긴다. 설치가 그 주소를 hostAliases 에 박은 게이트웨이라면 러너·
// Argo CD 의 이름 해석을 다시 맞춰야 한다. headless(None)는 빼지 않는다.
func (d *ResourceDumper) applyNamespaced(ctx context.Context, namespace string, payload []byte) error {
	err := d.applyDoc(ctx, namespace, payload, false)
	if err == nil || !strings.Contains(err.Error(), "spec.clusterIP") {
		return err
	}
	// 거절된 Service 만 IP 를 뺀다. 나머지는 첫 적용에서 이미 백업 때의 IP 로 섰다 — 그것까지
	// "새 주소" 라고 알리면 운영자가 멀쩡한 게이트웨이의 hostAliases 를 고치러 간다. 오류에서
	// 이름을 못 읽으면 headless 가 아닌 모든 Service 에서 뺀다.
	stripped, renumbered, serr := stripServiceClusterIPs(payload, rejectedServices(err.Error()))
	if serr != nil || len(renumbered) == 0 {
		return err
	}
	slog.Warn("백업 때의 ClusterIP 로 Service 를 되살리지 못해 새 주소로 복원합니다 — "+
		"게이트웨이 주소가 바뀌었다면 러너·Argo CD 의 hostAliases 를 새 주소로 맞춰야 합니다",
		"namespace", namespace, "services", renumbered, "cause", err)
	return d.applyDoc(ctx, namespace, stripped, false)
}

// rejectedClusterIP 는 API 서버가 ClusterIP 를 거절한 Service 이름을 뽑는다(실측 문구:
// `The Service "gw" is invalid: spec.clusterIPs: ... provided IP is already allocated`).
var rejectedClusterIP = regexp.MustCompile(`The Service "([^"]+)" is invalid: spec\.clusterIP`)

func rejectedServices(msg string) map[string]bool {
	out := map[string]bool{}
	for _, m := range rejectedClusterIP.FindAllStringSubmatch(msg, -1) {
		out[m[1]] = true
	}
	return out
}

// stripServiceClusterIPs 는 headless 가 아닌 Service 의 clusterIP·clusterIPs 를 뺀다. only 가 비어 있지
// 않으면 그 이름만 뺀다. 뺀 Service 이름을 돌려준다.
func stripServiceClusterIPs(payload []byte, only map[string]bool) ([]byte, []string, error) {
	var list map[string]any
	if err := json.Unmarshal(payload, &list); err != nil {
		return nil, nil, err
	}
	items, _ := list["items"].([]any)
	var names []string
	for _, it := range items {
		item, _ := it.(map[string]any)
		if kind, _ := item["kind"].(string); kind != "Service" {
			continue
		}
		spec, _ := item["spec"].(map[string]any)
		ip, has := spec["clusterIP"].(string)
		if !has || ip == "" || ip == "None" {
			continue
		}
		meta, _ := item["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		if len(only) > 0 && !only[name] {
			continue
		}
		delete(spec, "clusterIP")
		delete(spec, "clusterIPs")
		names = append(names, name)
	}
	out, err := json.Marshal(list)
	return out, names, err
}

// applyDoc 은 한 덩어리를 적용한다.
//
// 클러스터 범위에는 -n 을 붙이지 않는다. 붙이면 kubectl 이 네임스페이스를
// 무시하긴 하지만, 의도가 흐려지고 다음 사람이 헷갈린다.
func (d *ResourceDumper) applyDoc(ctx context.Context, namespace string, payload []byte, clusterScoped bool) error {
	args := []string{"apply"}
	if !clusterScoped {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-f", "-", "--server-side", "--force-conflicts")
	return d.run(ctx, bytes.NewReader(payload), io.Discard, args...)
}

var (
	_ port.ResourceDumper       = (*ResourceDumper)(nil)
	_ port.SharedResourceDumper = (*ResourceDumper)(nil)
)
