package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
)

// 스택은 접속 도메인의 인증서를 자기 내부 CA(nullus-internal-ca-issuer)로 발급한다.
// 그 CA 를 모르는 쪽은 스택 도구에 https 로 닿을 수 없다. 설치가 만든 신뢰를
// 설치가 필요한 곳에 넣지 않으면, 스택은 completed 인데 아무것도 이어지지 않는다:
//
//   - CI 잡의 git clone → "SSL certificate problem: unable to get local issuer certificate"
//   - Argo CD repo-server → "x509: certificate signed by unknown authority"
//
// 둘 다 파이프라인을 돌려 봐야 드러난다.
const (
	// internalCABundleSecretName 은 스택 네임스페이스에 두는 CA 사본이다.
	// 파드는 다른 네임스페이스의 Secret 을 마운트할 수 없다.
	internalCABundleSecretName = "nullus-internal-ca-bundle"
	internalCABundleKey        = "ca.crt"
	// internalCAMountPath 는 CI 잡 파드가 CA 를 받는 자리다.
	internalCAMountPath = "/etc/nullus/ca"
)

// loadInternalCACert 는 내부 CA 인증서를 읽어 캐시한다(base64 그대로).
//
// 못 읽으면 오류가 아니라 빈 값이다 — 사용자가 공인 인증서를 쓰도록 구성했거나
// cert-manager 를 재사용하는 클러스터일 수 있고, 그때는 넣을 것이 없다.
func (o *Orchestrator) loadInternalCACert(ctx context.Context) string {
	o.mu.Lock()
	if o.internalCALoaded {
		encoded := o.internalCAEncoded
		o.mu.Unlock()
		return encoded
	}
	o.mu.Unlock()

	var encoded string
	if ns, err := o.detectCertManagerNamespace(ctx); err == nil {
		if value, err := o.secretDataField(ctx, ns, defaultInternalCASecretName, "tls.crt"); err == nil {
			encoded = strings.TrimSpace(value)
		}
	}

	o.mu.Lock()
	o.internalCAEncoded = encoded
	o.internalCALoaded = true
	o.mu.Unlock()
	return encoded
}

func (o *Orchestrator) internalCACertEncoded() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.internalCAEncoded
}

// internalCACertPEM 은 캐시된 CA 를 PEM 으로 돌려준다. 없으면 빈 문자열이다.
func (o *Orchestrator) internalCACertPEM() string {
	encoded := o.internalCACertEncoded()
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// ensureInternalCABundleSecret 은 스택 네임스페이스에 CA 사본을 둔다.
//
// CI 잡 파드가 볼륨으로 받는다. 멱등하게 다시 적용한다 — CA 가 갱신되면 사본도
// 따라가야 하고, 어긋나면 잡이 다시 인증서 오류로 멈춘다.
func (o *Orchestrator) ensureInternalCABundleSecret(ctx context.Context, namespace string) error {
	encoded := o.loadInternalCACert(ctx)
	if encoded == "" {
		return nil
	}
	namespace = stackNamespaceOrDefault(namespace)
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
data:
  %s: %s
`, internalCABundleSecretName, namespace, internalCABundleKey, encoded)
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return fmt.Errorf("apply internal ca bundle secret: %w", err)
	}
	slog.Info("스택 네임스페이스에 내부 CA 사본을 두었습니다",
		"namespace", namespace, "secret", internalCABundleSecretName)
	return nil
}

func stackNamespaceOrDefault(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return "nullus"
	}
	return namespace
}

// argoCDInternalCAValues 는 Argo CD 가 스택 GitLab 의 인증서를 신뢰하게 한다.
//
// Argo CD 는 저장소 호스트별 CA 를 argocd-tls-certs-cm 에서 읽는다. 차트가 그
// ConfigMap 을 configs.tls.certificates 로 렌더하므로 설치 values 에 함께 담는다 —
// 설치 뒤에 손으로 넣으면 다음 업그레이드에서 지워진다.
//
// 넣지 않으면 Application 이 Synced 에 도달하지 못하고 ComparisonError 로 남는다:
// "x509: certificate signed by unknown authority". 파이프라인이 이미지를 올리고
// 매니페스트까지 갱신해도 배포가 일어나지 않는다.
func argoCDInternalCAValues(caPEM, accessDomain string) map[string]any {
	pem := strings.TrimSpace(caPEM)
	hosts := runnerCIHostnames(accessDomain)
	if pem == "" || len(hosts) == 0 {
		return nil
	}
	certificates := map[string]any{}
	for _, host := range hosts {
		certificates[host] = pem + "\n"
	}
	return map[string]any{
		"configs": map[string]any{
			"tls": map[string]any{
				"certificates": certificates,
			},
		},
	}
}

// argoCDGatewayHostAliasValues 는 Argo CD 파드가 스택 도구 이름을 풀게 한다.
//
// repo-server 는 스택 GitLab 에서 매니페스트를 읽는다. 그 주소는 접속 도메인이고
// 클러스터 안에서는 아무도 풀어 주지 않아, CA 를 신뢰시켜도 그 앞 단계인 이름
// 해석에서 "dial tcp: lookup gitlab.<도메인>" 으로 멈춘다.
//
// 러너와 같은 방식이다 — 파드의 hostAliases 로 끝내고 클러스터 DNS 는 건드리지 않는다.
func argoCDGatewayHostAliasValues(gatewayIP, accessDomain string) map[string]any {
	ip := strings.TrimSpace(gatewayIP)
	hosts := runnerCIHostnames(accessDomain)
	if ip == "" || len(hosts) == 0 {
		return nil
	}
	return map[string]any{
		"global": map[string]any{
			"hostAliases": []any{
				map[string]any{"ip": ip, "hostnames": hosts},
			},
		},
	}
}
