package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 클러스터 공용 자원 덤프·복원.
//
// 클러스터의 마지막 스택을 지우면 Nullus 가 설치한 공용 자원이 회수된다(stack 모듈의
// delete_shared_resources.go): 설치 표시가 붙은 Gateway CRD·GatewayClass·내부 CA. 네임스페이스
// 덤프는 그것을 담지 않는다 — 클러스터 범위 덤프는 Helm 소유 어노테이션으로 거르는데 Gateway
// API CRD 는 kubectl 로, GatewayClass 는 create 로 만들어져 어노테이션이 없고, 내부 CA 는
// cert-manager 네임스페이스에 있다. 그 상태에서 복구하면 볼륨까지 되돌린 뒤 Gateway CR 적용에서
// "no matches for kind" 로 죽는다.
//
// cert-manager·metrics-server 릴리스 자체는 담지 않는다. 컨트롤러는 데이터가 아니라 설치다 —
// 재설치하면 되고, CRD 만 되살려 두면 재설치를 Helm 소유권 충돌로 막는다. 내부 CA 는 다르다.
// 개인키는 다시 만들 수 없고, 새로 만들면 이미 발급된 인증서와 스택에 배포된 신뢰 번들이 어긋난다.

const (
	installedBySelector = shareddomain.LabelInstalledBy + "=" + shareddomain.InstalledByNullus

	crdKind            = "customresourcedefinitions.apiextensions.k8s.io"
	gatewayClassKind   = "gatewayclasses.gateway.networking.k8s.io"
	clusterIssuerKind  = "clusterissuers.cert-manager.io"
	certificateKind    = "certificates.cert-manager.io"
	certManagerCRDName = "certificaterequests.cert-manager.io"
)

// DumpShared 는 Nullus 가 설치한 공용 자원 중 백업이 담을 수 있는 것을 뜬다.
//
// 설치 표시 라벨로 고른다 — 사용자가 직접 깐 Gateway API 는 Nullus 것이 아니고, 회수되지도
// 않는다. 없는 종류(cert-manager 가 없는 클러스터의 ClusterIssuer)는 건너뛴다. 담을 것이 없으면
// 0 을 돌려주고 아무것도 쓰지 않는다.
func (d *ResourceDumper) DumpShared(ctx context.Context, out io.Writer) (int64, error) {
	clusterKinds, err := d.kindSet(ctx, "--namespaced=false")
	if err != nil {
		return 0, fmt.Errorf("클러스터 범위 리소스 종류 조회: %w", err)
	}
	nsKinds, err := d.kindSet(ctx, "--namespaced=true")
	if err != nil {
		return 0, fmt.Errorf("리소스 종류 조회: %w", err)
	}

	var cluster, namespaced []map[string]any
	if clusterKinds[crdKind] {
		items, err := d.getItems(ctx, "get", "crd", "-l", installedBySelector, "-o", "json")
		if err != nil {
			return 0, fmt.Errorf("설치 표시가 붙은 CRD 조회: %w", err)
		}
		for _, item := range items {
			// cert-manager CRD 도 설치 표시를 달고 깔리지만 담지 않는다(위 설명).
			if shareddomain.IsGatewayCRD(itemName(item)) {
				cluster = append(cluster, item)
			}
		}
	}
	if clusterKinds[gatewayClassKind] {
		items, err := d.getItems(ctx, "get", gatewayClassKind, "-l", installedBySelector, "-o", "json")
		if err != nil {
			return 0, fmt.Errorf("GatewayClass 조회: %w", err)
		}
		cluster = append(cluster, items...)
	}
	if clusterKinds[clusterIssuerKind] {
		items, err := d.getItems(ctx, "get", clusterIssuerKind,
			shareddomain.InternalCABootstrapIssuerName, shareddomain.InternalCAIssuerName,
			"-o", "json", "--ignore-not-found")
		if err != nil {
			return 0, fmt.Errorf("내부 CA ClusterIssuer 조회: %w", err)
		}
		cluster = append(cluster, items...)
	}
	// 네임스페이스 쪽은 Secret 이 먼저다 — cert-manager 는 Certificate 를 만날 때 secretName 의
	// Secret 이 이미 있으면 그것을 쓰고, 없으면 새 키로 발급한다.
	if nsKinds["secrets"] {
		items, err := d.getItems(ctx, "get", "secrets", "-A",
			"--field-selector", "metadata.name="+shareddomain.InternalCASecretName, "-o", "json")
		if err != nil {
			return 0, fmt.Errorf("내부 CA Secret 조회: %w", err)
		}
		namespaced = append(namespaced, items...)
	}
	if nsKinds[certificateKind] {
		items, err := d.getItems(ctx, "get", certificateKind, "-A",
			"--field-selector", "metadata.name="+shareddomain.InternalCACertName, "-o", "json")
		if err != nil {
			return 0, fmt.Errorf("내부 CA Certificate 조회: %w", err)
		}
		namespaced = append(namespaced, items...)
	}

	cluster, err = sanitizeItems(cluster)
	if err != nil {
		return 0, err
	}
	namespaced, err = sanitizeItems(namespaced)
	if err != nil {
		return 0, err
	}
	if len(cluster) == 0 && len(namespaced) == 0 {
		return 0, nil
	}
	payload, err := json.MarshalIndent(map[string]any{
		"cluster_scoped": listOf(cluster),
		"namespaced":     listOf(namespaced),
	}, "", "  ")
	if err != nil {
		return 0, err
	}
	counter := &countingWriter{w: out}
	_, err = counter.Write(payload)
	return counter.n, err
}

// ApplyShared 는 공용 자원을 **없을 때만** 만든다.
//
// 순서: CRD → 등록 대기 → 그 밖의 클러스터 범위(GatewayClass·ClusterIssuer) → 내부 CA Secret →
// Certificate. 이미 있는 것은 그대로 둔다 — 다른 스택이 쓰는 지금의 CA 나 CRD 를 백업 때의 것으로
// 덮으면 그 스택이 깨진다. 종류의 CRD 가 없으면(cert-manager 없음) 어느 종류가 왜 안 되는지와
// 무엇을 먼저 설치해야 하는지 말하고 멈춘다.
func (d *ResourceDumper) ApplyShared(ctx context.Context, in io.Reader) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var doc struct {
		ClusterScoped json.RawMessage `json:"cluster_scoped"`
		Namespaced    json.RawMessage `json:"namespaced"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("공용 자원 덤프 해석: %w", err)
	}
	cluster, err := itemsOf(doc.ClusterScoped)
	if err != nil {
		return err
	}
	namespaced, err := itemsOf(doc.Namespaced)
	if err != nil {
		return err
	}

	var crds, others []map[string]any
	for _, item := range cluster {
		if item["kind"] == "CustomResourceDefinition" {
			crds = append(crds, item)
		} else {
			others = append(others, item)
		}
	}
	for _, item := range crds {
		if err := d.createIfMissing(ctx, "", item); err != nil {
			return err
		}
	}
	if len(crds) > 0 {
		// CRD 등록이 API 서버에 반영될 때까지 기다린다. 못 봐도 계속 간다 — 이미 있던 CRD 일 수 있다.
		_ = d.run(ctx, nil, io.Discard, "wait", "--for=condition=established", "--timeout=120s", "crd", "--all")
	}
	for _, item := range others {
		if err := d.createIfMissing(ctx, "", item); err != nil {
			return err
		}
	}

	if len(namespaced) == 0 {
		return nil
	}
	// 내부 CA 는 cert-manager 가 **지금** 설치된 네임스페이스에 있어야 한다 — ClusterIssuer 의
	// secretName 은 cluster-resource-namespace 에서만 읽힌다. 백업 때와 다른 곳에 재설치됐으면
	// 그쪽으로 옮긴다. 알아내지 못하면 백업 때의 자리다.
	target := d.certManagerNamespace(ctx)
	ensured := map[string]bool{}
	for _, item := range namespaced {
		ns := itemNamespace(item)
		if target != "" {
			ns = target
			setNamespace(item, ns)
		}
		if ns == "" {
			return fmt.Errorf("공용 자원 %s/%s 의 네임스페이스를 알 수 없습니다", item["kind"], itemName(item))
		}
		if !ensured[ns] {
			if err := d.ensureNamespace(ctx, ns); err != nil {
				return err
			}
			ensured[ns] = true
		}
		if err := d.createIfMissing(ctx, ns, item); err != nil {
			return err
		}
	}
	return nil
}

// ApplySharedSecrets 는 덤프 가운데 CRD 가 필요 없는 것 — 내부 CA Secret 과 그 네임스페이스 — 만 없을 때
// 만든다. cert-manager 를 다시 세우기 **전에** 부른다.
//
// 순서가 전부다. 설치기의 cert-manager 단계는 끝에 내부 CA 를 부트스트랩한다(Certificate
// nullus-internal-ca-cert → Secret nullus-internal-ca). 그때 Secret 이 이미 있고 발급 정보가 맞으면
// cert-manager 는 그것을 채택하고, 없으면 **새 키를 발급**한다. 새 키가 생긴 뒤에는 ApplyShared 의
// 없을-때만-만들기가 옛 키를 되살릴 수 없다 — 도구들이 신뢰하는 CA 가 백업 때와 달라진다.
// Secret 은 CRD 가 필요 없으니 컨트롤러보다 먼저 둘 수 있다. 그 밖의 것(CRD·GatewayClass·
// ClusterIssuer·Certificate)은 ApplyShared 가 컨트롤러가 선 뒤에 만든다.
//
// 자리는 cert-manager 가 지금 있는 네임스페이스, 모르면(회수된 뒤라 보통 모른다) 백업 때의 자리다.
// 설치기는 CRD 어노테이션이 없으면 차트 기본 네임스페이스에 깐다 — 백업이 그 기본 배치에서 떠졌으면
// 같은 자리다.
func (d *ResourceDumper) ApplySharedSecrets(ctx context.Context, in io.Reader) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var doc struct {
		Namespaced json.RawMessage `json:"namespaced"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("공용 자원 덤프 해석: %w", err)
	}
	namespaced, err := itemsOf(doc.Namespaced)
	if err != nil {
		return err
	}
	target := d.certManagerNamespace(ctx)
	ensured := map[string]bool{}
	for _, item := range namespaced {
		if item["kind"] != "Secret" {
			continue
		}
		ns := itemNamespace(item)
		if target != "" {
			ns = target
			setNamespace(item, ns)
		}
		if ns == "" {
			return fmt.Errorf("공용 자원 Secret/%s 의 네임스페이스를 알 수 없습니다", itemName(item))
		}
		if !ensured[ns] {
			if err := d.ensureNamespace(ctx, ns); err != nil {
				return err
			}
			ensured[ns] = true
		}
		if err := d.createIfMissing(ctx, ns, item); err != nil {
			return err
		}
	}
	return nil
}

// createIfMissing 은 하나를 만든다. 이미 있으면 그대로 둔다.
func (d *ResourceDumper) createIfMissing(ctx context.Context, namespace string, item map[string]any) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	args := []string{"create"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-f", "-")
	err = d.run(ctx, bytes.NewReader(payload), io.Discard, args...)
	if err == nil {
		return nil
	}
	kind, _ := item["kind"].(string)
	name := itemName(item)
	msg := err.Error()
	switch {
	case strings.Contains(msg, "AlreadyExists"), strings.Contains(msg, "already exists"):
		slog.Debug("공용 자원이 이미 있어 그대로 둡니다", "kind", kind, "name", name)
		return nil
	case strings.Contains(msg, "no matches for kind"), strings.Contains(msg, "doesn't have a resource type"):
		return fmt.Errorf("공용 자원 %s/%s 를 되살릴 수 없습니다: 그 종류의 CRD 가 클러스터에 없습니다 — %s. "+
			"먼저 설치한 뒤 복구를 다시 실행하세요: %w", kind, name, missingCRDHint(kind), err)
	}
	return fmt.Errorf("공용 자원 %s/%s 생성: %w", kind, name, err)
}

// missingCRDHint 는 그 종류를 누가 깔아 주는지다.
func missingCRDHint(kind string) string {
	switch kind {
	case "ClusterIssuer", "Issuer", "Certificate", "CertificateRequest":
		return "cert-manager 가 설치돼 있지 않습니다(스택 설치의 cert-manager 단계가 까는 것과 같은 차트, CRD 포함)"
	case "GatewayClass", "Gateway", "HTTPRoute":
		return "Gateway API CRD 가 설치돼 있지 않습니다(같은 백업의 CRD 복원이 먼저 실패했는지 보세요)"
	}
	return "그 종류를 제공하는 컨트롤러가 설치돼 있지 않습니다"
}

func (d *ResourceDumper) ensureNamespace(ctx context.Context, namespace string) error {
	err := d.run(ctx, nil, io.Discard, "create", "namespace", namespace)
	if err == nil || strings.Contains(err.Error(), "AlreadyExists") || strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return fmt.Errorf("네임스페이스 %s 준비: %w", namespace, err)
}

// certManagerNamespace 는 cert-manager 가 지금 설치된 네임스페이스다. 스택 설치기와 같은 방법으로
// CRD 의 Helm 릴리스 어노테이션에서 읽는다. 모르면 빈 문자열이다.
func (d *ResourceDumper) certManagerNamespace(ctx context.Context) string {
	var out strings.Builder
	if err := d.run(ctx, nil, &out, "get", "crd", certManagerCRDName,
		"-o", `jsonpath={.metadata.annotations.meta\.helm\.sh/release-namespace}`); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// kindSet 은 `kubectl api-resources -o name` 의 결과를 집합으로 준다.
func (d *ResourceDumper) kindSet(ctx context.Context, namespacedFlag string) (map[string]bool, error) {
	var buf strings.Builder
	if err := d.run(ctx, nil, &buf, "api-resources", "--verbs=list", namespacedFlag, "-o", "name"); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, line := range strings.Split(buf.String(), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			set[name] = true
		}
	}
	return set, nil
}

// getItems 는 `kubectl get ... -o json` 의 항목들이다. 하나만 돌아오면(이름 하나로 조회) 그것을
// 항목 하나로 본다.
func (d *ResourceDumper) getItems(ctx context.Context, args ...string) ([]map[string]any, error) {
	var raw bytes.Buffer
	if err := d.run(ctx, nil, &raw, args...); err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw.Bytes())) == 0 {
		return nil, nil
	}
	return itemsOf(raw.Bytes())
}

func itemsOf(raw []byte) ([]map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("리소스 목록 해석: %w", err)
	}
	if items, ok := obj["items"].([]any); ok {
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	}
	if _, ok := obj["kind"]; ok {
		return []map[string]any{obj}, nil
	}
	return nil, nil
}

// sanitizeItems 는 네임스페이스 덤프와 같은 규칙으로 서버 관리 필드를 걷어낸다.
func sanitizeItems(items []map[string]any) ([]map[string]any, error) {
	if len(items) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(listOf(items))
	if err != nil {
		return nil, err
	}
	cleaned, err := sanitize(raw)
	if err != nil {
		return nil, err
	}
	return itemsOf(cleaned)
}

func listOf(items []map[string]any) map[string]any {
	if items == nil {
		items = []map[string]any{}
	}
	return map[string]any{"apiVersion": "v1", "kind": "List", "items": items}
}

func itemName(item map[string]any) string {
	meta, _ := item["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	return name
}

func itemNamespace(item map[string]any) string {
	meta, _ := item["metadata"].(map[string]any)
	ns, _ := meta["namespace"].(string)
	return ns
}

func setNamespace(item map[string]any, namespace string) {
	meta, _ := item["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		item["metadata"] = meta
	}
	meta["namespace"] = namespace
}
