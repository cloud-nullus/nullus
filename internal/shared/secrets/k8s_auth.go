package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// 컨트롤 플레인이 대상 클러스터의 OpenBao 에 인증하는 경로를 구현한다.
//
// Nullus 백엔드는 대상 클러스터 "밖"에서 동작하므로 제출할 ServiceAccount
// 토큰이 없다. 그래서 이미 보유한 kubeconfig 로 대상 클러스터의 SA 단기 토큰을
// TokenRequest API 로 발급받아 그것으로 로그인한다. 새로운 정적 비밀이 생기지
// 않고, kubeconfig 라는 기존 Source of Truth 에서 파생된 자격만 사용한다.
//
// OpenBao 에는 API server proxy 를 통해 도달한다. kubeconfig 만으로 접근할 수
// 있어 컨트롤 플레인 쪽 DNS 해석이나 인증서 배포가 필요 없다.

const (
	// saTokenTTL 은 TokenRequest 로 발급받는 SA 토큰의 수명이다.
	// OpenBao 로그인 직후 버리므로 짧게 잡는다.
	saTokenTTL = 10 * time.Minute

	// defaultOpenBaoService / Port 는 대상 클러스터의 OpenBao 서비스 좌표다.
	defaultOpenBaoService = "openbao"
	defaultOpenBaoPort    = "8200"

	// ControllerRole / ControllerServiceAccount 는 백엔드가 사용하는 신원이다.
	// 부트스트랩 Job 이 만드는 role/SA 이름과 일치해야 한다.
	ControllerRole           = "nullus-controller"
	ControllerServiceAccount = "nullus-controller"
)

// KubernetesAuthConfig 는 k8s auth 로그인에 필요한 정보를 담는다.
type KubernetesAuthConfig struct {
	Kubeconfig     []byte
	Namespace      string
	Role           string
	ServiceAccount string
	// ServiceName/ServicePort 가 비면 openbao:8200 을 사용한다.
	ServiceName string
	ServicePort string
}

// NewKubernetesAuthStore 는 대상 클러스터의 OpenBao 에 Kubernetes Auth 로
// 접속하는 store 를 만든다.
func NewKubernetesAuthStore(cfg KubernetesAuthConfig) (*OpenBaoStore, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig 파싱 실패: %w", err)
	}
	restConfig.Timeout = 30 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("kubernetes 클라이언트 생성 실패: %w", err)
	}

	svc := strings.TrimSpace(cfg.ServiceName)
	if svc == "" {
		svc = defaultOpenBaoService
	}
	port := strings.TrimSpace(cfg.ServicePort)
	if port == "" {
		port = defaultOpenBaoPort
	}

	transport := &apiServerProxyTransport{
		clientset: clientset,
		namespace: cfg.Namespace,
		service:   svc,
		port:      port,
	}

	provider := NewKubernetesTokenProvider(func(ctx context.Context) (string, time.Duration, error) {
		return loginWithServiceAccount(ctx, clientset, transport, cfg)
	})

	store := NewOpenBaoStoreWithProvider("", provider)
	store.transport = transport
	return store, nil
}

// loginWithServiceAccount 는 SA 단기 토큰을 발급받아 OpenBao 에 로그인한다.
func loginWithServiceAccount(
	ctx context.Context,
	clientset kubernetes.Interface,
	transport *apiServerProxyTransport,
	cfg KubernetesAuthConfig,
) (string, time.Duration, error) {
	seconds := int64(saTokenTTL.Seconds())
	tr, err := clientset.CoreV1().ServiceAccounts(cfg.Namespace).CreateToken(
		ctx,
		cfg.ServiceAccount,
		&authnv1.TokenRequest{
			Spec: authnv1.TokenRequestSpec{ExpirationSeconds: &seconds},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", 0, fmt.Errorf("serviceaccount %s 토큰 발급 실패: %w", cfg.ServiceAccount, err)
	}

	body, _ := json.Marshal(map[string]any{ // #nosec G104 -- 단순 타입 마샬은 실패하지 않는다
		"role": cfg.Role,
		"jwt":  tr.Status.Token,
	})

	raw, err := transport.post(ctx, "/v1/auth/kubernetes/login", body)
	if err != nil {
		return "", 0, fmt.Errorf("openbao kubernetes 로그인 실패: %w", err)
	}

	var out struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, fmt.Errorf("로그인 응답 파싱 실패: %w", err)
	}
	if strings.TrimSpace(out.Auth.ClientToken) == "" {
		return "", 0, fmt.Errorf("로그인 응답에 client_token 이 없습니다")
	}

	ttl := time.Duration(out.Auth.LeaseDuration) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	return out.Auth.ClientToken, ttl, nil
}
