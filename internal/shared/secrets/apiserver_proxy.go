package secrets

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/client-go/kubernetes"
)

// apiServerProxyTransport 는 Kubernetes API server proxy 를 통해 클러스터 내부
// 서비스로 HTTP 요청을 보낸다.
//
// 게이트웨이 도메인(openbao.<access_domain>)을 쓰지 않는 이유는 컨트롤 플레인
// 쪽 DNS 해석과 인증서 배포가 필요하기 때문이다. API server proxy 는
// kubeconfig 만으로 도달하므로 추가 노출 없이 스택별 OpenBao 에 접근할 수 있다.
//
// 경로 형식:
//
//	/api/v1/namespaces/{ns}/services/http:{svc}:{port}/proxy/{path}
type apiServerProxyTransport struct {
	clientset kubernetes.Interface
	namespace string
	service   string
	port      string
}

// resourceName 은 proxy 서브리소스가 요구하는 scheme:name:port 형식이다.
func (t *apiServerProxyTransport) resourceName() string {
	return fmt.Sprintf("http:%s:%s", t.service, t.port)
}

func (t *apiServerProxyTransport) get(ctx context.Context, path string, headers map[string]string) ([]byte, error) {
	req := t.clientset.CoreV1().RESTClient().Get().
		Namespace(t.namespace).
		Resource("services").
		Name(t.resourceName()).
		SubResource("proxy").
		Suffix(strings.TrimPrefix(path, "/"))
	for k, v := range headers {
		req = req.SetHeader(k, v)
	}
	return req.DoRaw(ctx)
}

func (t *apiServerProxyTransport) post(ctx context.Context, path string, body []byte, headers ...map[string]string) ([]byte, error) {
	req := t.clientset.CoreV1().RESTClient().Post().
		Namespace(t.namespace).
		Resource("services").
		Name(t.resourceName()).
		SubResource("proxy").
		Suffix(strings.TrimPrefix(path, "/")).
		SetHeader("Content-Type", "application/json").
		Body(body)
	for _, h := range headers {
		for k, v := range h {
			req = req.SetHeader(k, v)
		}
	}
	return req.DoRaw(ctx)
}

// do 는 OpenBaoStore 가 사용하는 통합 진입점이다.
// method 는 GET/POST 만 지원한다 — KV v2 읽기/쓰기와 로그인에 필요한 전부다.
func (t *apiServerProxyTransport) do(ctx context.Context, method, path string, body []byte, token string) ([]byte, error) {
	headers := map[string]string{}
	if strings.TrimSpace(token) != "" {
		headers["X-Vault-Token"] = token
	}
	switch method {
	case http.MethodGet:
		return t.get(ctx, path, headers)
	case http.MethodPost:
		return t.post(ctx, path, body, headers)
	default:
		return nil, fmt.Errorf("지원하지 않는 메서드: %s", method)
	}
}
