package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenBaoStore struct {
	addr     string
	provider TokenProvider
	client   *http.Client
	// transport 가 설정되면 직접 HTTP 대신 API server proxy 를 통해 요청한다.
	// 대상 클러스터 내부의 OpenBao 에 kubeconfig 만으로 도달하기 위한 경로다.
	transport *apiServerProxyTransport
}

// request 는 addr 직접 호출과 API server proxy 경유를 하나로 묶는다.
func (s *OpenBaoStore) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	token, err := s.authToken(ctx)
	if err != nil {
		return nil, err
	}
	if s.transport != nil {
		return s.transport.do(ctx, method, path, body, token)
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.addr+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Vault-Token", token)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- best-effort body read for error context
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openbao request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// Check 는 금고가 응답하고, 봉인이 풀려 있고, 자격이 유효한지 확인한다.
// preflight gate 의 판단 근거다.
func (s *OpenBaoStore) Check(ctx context.Context) error {
	sealRaw, err := s.request(ctx, http.MethodGet, "/v1/sys/seal-status", nil)
	if err != nil {
		return fmt.Errorf("openbao 상태 조회 실패: %w", err)
	}
	var sealStatus struct {
		Sealed      bool `json:"sealed"`
		Initialized bool `json:"initialized"`
	}
	if err := json.Unmarshal(sealRaw, &sealStatus); err != nil {
		return fmt.Errorf("seal-status 파싱 실패: %w", err)
	}
	if !sealStatus.Initialized {
		return fmt.Errorf("openbao 가 초기화되지 않았습니다")
	}
	if sealStatus.Sealed {
		return fmt.Errorf("openbao 가 봉인 상태입니다")
	}

	if _, err := s.request(ctx, http.MethodGet, "/v1/auth/token/lookup-self", nil); err != nil {
		return fmt.Errorf("openbao 토큰 검증 실패: %w", err)
	}
	return nil
}

// NewOpenBaoStore 는 정적 토큰을 사용하는 store 를 만든다. 로컬 개발 전용이다.
func NewOpenBaoStore(addr, token string) *OpenBaoStore {
	return NewOpenBaoStoreWithProvider(addr, NewStaticTokenProvider(strings.TrimSpace(token)))
}

// NewOpenBaoStoreWithProvider 는 토큰 공급 전략을 주입해 store 를 만든다.
// 운영 경로는 KubernetesTokenProvider 를 넘겨 단기 자격을 사용한다.
func NewOpenBaoStoreWithProvider(addr string, provider TokenProvider) *OpenBaoStore {
	return &OpenBaoStore{
		addr:     strings.TrimRight(strings.TrimSpace(addr), "/"),
		provider: provider,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// authToken 은 요청에 사용할 토큰을 얻는다.
func (s *OpenBaoStore) authToken(ctx context.Context) (string, error) {
	if s.provider == nil {
		return "", ErrProviderNotConfigured
	}
	return s.provider.Token(ctx)
}

func (s *OpenBaoStore) PutToken(ctx context.Context, path, value string) error {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"token": value}}) // #nosec G104 -- 단순 타입 마샬은 실패하지 않는다
	if _, err := s.request(ctx, http.MethodPost, "/v1/"+mount+"/data/"+subpath, body); err != nil {
		return fmt.Errorf("openbao write failed: %w", err)
	}
	return nil
}

func (s *OpenBaoStore) GetToken(ctx context.Context, path string) (string, error) {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return "", err
	}
	raw, err := s.request(ctx, http.MethodGet, "/v1/"+mount+"/data/"+subpath, nil)
	if err != nil {
		return "", fmt.Errorf("openbao read failed: %w", err)
	}
	var out struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	v, _ := out.Data.Data["token"].(string)
	return v, nil
}

// splitKVPath 는 `kv/nullus/{env}/...` 형태의 경로를 마운트와 하위 경로로 나눈다.
//
// 이전에는 마운트 `kv` 를 `secret` 으로 재작성했다. dev 모드가 `secret/` 을
// 자동 마운트했기 때문인데, 그 결과 문서·DB 의 경로 규약과 실제 요청 경로가
// 어긋나 있었다. 운영 모드에서는 부트스트랩이 `kv` 라는 이름으로 KV v2 를
// 마운트하므로 재작성 없이 경로 규약을 그대로 사용한다.
func splitKVPath(path string) (string, string, error) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid openbao path: %s", path)
	}
	return parts[0], strings.Join(parts[1:], "/"), nil
}

// ── KV 전체 순회 (백업/복구용) ────────────────────────────────────────────
//
// 백업은 "이 스택의 모든 시크릿" 을 떠야 하는데, PutToken/GetToken 은 경로를
// 이미 아는 호출자를 위한 것이라 목록을 줄 수 없다. OpenBao 는 단일 replica
// 라 file 스토리지를 쓰고 raft snapshot API 가 없으므로, 경로를 순회해
// 논리 export 하는 것이 유일한 방법이다.
// (설계: docs/11_기능설계/Nullus_백업복구_설계.md §3.2)

// KVBrowser 는 KV 트리를 훑고 값을 통째로 읽고 쓰는 확장 창구다.
//
// Store 인터페이스를 넓히지 않고 별도 인터페이스로 둔 이유: 이 능력이
// 필요한 곳은 백업뿐이고, 토큰만 다루는 대다수 호출자에게 강제할 이유가 없다.
type KVBrowser interface {
	ListKeys(ctx context.Context, path string) ([]string, error)
	GetSecret(ctx context.Context, path string) (map[string]any, error)
	PutSecret(ctx context.Context, path string, data map[string]any) error
}

// ListKeys 는 경로 바로 아래의 항목을 돌려준다. 디렉터리는 "/" 로 끝난다.
func (s *OpenBaoStore) ListKeys(ctx context.Context, path string) ([]string, error) {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return nil, err
	}
	// KV v2 의 목록은 metadata 엔드포인트에 있다. data 로는 조회되지 않는다.
	url := "/v1/" + mount + "/metadata/" + subpath
	raw, err := s.request(ctx, "LIST", url, nil)
	if err != nil {
		// 잎 경로이거나 비어 있으면 404 다 — 오류가 아니라 "하위가 없다" 이다.
		return nil, nil
	}
	var out struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Data.Keys, nil
}

// GetSecret 은 경로의 값 전체를 돌려준다 (token 필드만이 아니라).
func (s *OpenBaoStore) GetSecret(ctx context.Context, path string) (map[string]any, error) {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return nil, err
	}
	raw, err := s.request(ctx, http.MethodGet, "/v1/"+mount+"/data/"+subpath, nil)
	if err != nil {
		return nil, fmt.Errorf("openbao read failed: %w", err)
	}
	var out struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Data.Data, nil
}

// PutSecret 은 값 전체를 쓴다.
func (s *OpenBaoStore) PutSecret(ctx context.Context, path string, data map[string]any) error {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return fmt.Errorf("시크릿 직렬화: %w", err)
	}
	if _, err := s.request(ctx, http.MethodPost, "/v1/"+mount+"/data/"+subpath, body); err != nil {
		return fmt.Errorf("openbao write failed: %w", err)
	}
	return nil
}

var _ KVBrowser = (*OpenBaoStore)(nil)
