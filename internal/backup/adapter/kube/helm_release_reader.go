package kube

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// HelmReleaseReader 는 네임스페이스에 설치된 차트의 버전을 읽는다.
//
// 왜 필요한가: 복구가 다른 버전으로 재설치하면 도구들이 기동하며 스키마
// 마이그레이션을 돌린다. 그 순간 데이터는 백업 시점의 모습이 아니게 되고,
// 되돌릴 수 없다 — "복구했다" 와 "이전 상태다" 가 갈리는 지점이다.
//
// 릴리스 Secret 은 리소스 덤프에도 담기지만 그것은 암호화된 데다 통째로
// 풀어야 읽을 수 있다. 버전 선택은 **복구를 시작하기 전에** 해야 하므로,
// 암호화하지 않는 매니페스트에 따로 기록한다.
type HelmReleaseReader struct {
	client kubernetes.Interface
}

func NewHelmReleaseReader(client kubernetes.Interface) *HelmReleaseReader {
	return &HelmReleaseReader{client: client}
}

// helmReleaseSecretType 은 Helm 3 이 릴리스를 저장하는 Secret 의 타입이다.
const helmReleaseSecretType = "helm.sh/release.v1"

// ListHelmReleases 는 네임스페이스의 릴리스를 릴리스별 **최신 리비전만** 남겨 돌려준다.
//
// Helm 은 리비전마다 Secret 을 하나씩 남긴다. 복구가 재현해야 하는 것은
// 백업 시점에 실제로 돌고 있던 버전이므로 가장 높은 리비전을 고른다.
func (r *HelmReleaseReader) ListHelmReleases(ctx context.Context, namespace string) ([]domain.HelmReleaseSpec, error) {
	list, err := r.client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=" + helmReleaseSecretType,
	})
	if err != nil {
		return nil, fmt.Errorf("Helm 릴리스 조회(%s): %w", namespace, err)
	}

	latest := make(map[string]domain.HelmReleaseSpec, len(list.Items))
	for _, item := range list.Items {
		if item.Type != helmReleaseSecretType {
			// fake 클라이언트는 FieldSelector 를 걸러 주지 않는다. 실클러스터에서
			// 이 검사는 무해하고, 테스트에서는 이것이 유일한 필터다.
			continue
		}
		raw, ok := item.Data["release"]
		if !ok {
			continue
		}
		spec, err := decodeHelmRelease(raw)
		if err != nil {
			// 하나를 못 읽었다고 나머지를 버리지 않는다. 다만 조용히 넘기지도
			// 않는다 — 이름만이라도 남겨 무엇을 못 읽었는지 드러낸다.
			name := releaseNameFromSecret(item.Name)
			if name == "" {
				continue
			}
			if _, seen := latest[name]; !seen {
				latest[name] = domain.HelmReleaseSpec{Name: name, Chart: name}
			}
			continue
		}
		if cur, seen := latest[spec.Name]; !seen || spec.Revision > cur.Revision {
			latest[spec.Name] = spec
		}
	}

	out := make([]domain.HelmReleaseSpec, 0, len(latest))
	for _, v := range latest {
		out = append(out, v)
	}
	return out, nil
}

// releaseNameFromSecret 는 sh.helm.release.v1.<name>.v<rev> 에서 이름을 뽑는다.
func releaseNameFromSecret(secretName string) string {
	const prefix = "sh.helm.release.v1."
	if !strings.HasPrefix(secretName, prefix) {
		return ""
	}
	rest := secretName[len(prefix):]
	if i := strings.LastIndex(rest, ".v"); i > 0 {
		return rest[:i]
	}
	return rest
}

// decodeHelmRelease 는 릴리스 Secret 의 값을 푼다: base64 → gzip → JSON.
//
// 읽지 못하면 오류를 낸다. 빈 값을 조용히 돌려주면 매니페스트에 "차트를
// 모른다" 가 "차트가 없다" 로 기록되고, 복구가 임의 버전을 골라도 아무도
// 막지 못한다.
func decodeHelmRelease(raw []byte) (domain.HelmReleaseSpec, error) {
	gzipped, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return domain.HelmReleaseSpec{}, fmt.Errorf("릴리스 base64 해독: %w", err)
	}
	zr, err := gzip.NewReader(strings.NewReader(string(gzipped)))
	if err != nil {
		return domain.HelmReleaseSpec{}, fmt.Errorf("릴리스 gzip 해제: %w", err)
	}
	defer func() { _ = zr.Close() }()

	plain, err := io.ReadAll(zr)
	if err != nil {
		return domain.HelmReleaseSpec{}, fmt.Errorf("릴리스 읽기: %w", err)
	}

	var rel struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
		Chart   struct {
			Metadata struct {
				Name       string `json:"name"`
				Version    string `json:"version"`
				AppVersion string `json:"appVersion"`
			} `json:"metadata"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(plain, &rel); err != nil {
		return domain.HelmReleaseSpec{}, fmt.Errorf("릴리스 해석: %w", err)
	}
	return domain.HelmReleaseSpec{
		Name:       rel.Name,
		Chart:      rel.Chart.Metadata.Name,
		Version:    rel.Chart.Metadata.Version,
		AppVersion: rel.Chart.Metadata.AppVersion,
		Revision:   rel.Version,
	}, nil
}
