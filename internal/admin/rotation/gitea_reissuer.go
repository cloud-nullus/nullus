package rotation

import (
	"context"
	"fmt"
	"strings"
)

// GiteaReissuer 는 스택 안에 설치된 Gitea 의 액세스 토큰을 재발급한다.
//
// GitHub 재발급기와 근본적으로 다르다. GitHub 은 App JWT 로 외부 API 를 부르지만
// Gitea 는 클러스터 안에 있고 외부에 노출되지 않을 수 있어 HTTP 로 닿지 못한다 —
// 발급 경로와 같은 방식으로 파드 안의 gitea CLI 를 쓴다.
//
// 발급 자체는 CI/CD 모듈의 TokenIssuer 가 이미 구현하고 있다. 모듈 간 직접
// import 는 금지되므로 그 동작을 함수로 주입받는다 — 재발급 로직을 두 곳에
// 복제하면 스코프나 토큰 이름이 갈라져 회전 후 인증이 조용히 실패한다.
type GiteaReissuer struct {
	issue GiteaTokenIssueFunc
}

// GiteaTokenIssueFunc 는 스택의 Gitea 토큰을 강제로 새로 발급한다.
//
// 어느 스택인지는 Metadata 가 알려준다 — 회전 스케줄러는 token_sources 행을
// 훑을 뿐 스택 구조를 모른다.
type GiteaTokenIssueFunc func(ctx context.Context, spec GiteaReissueSpec) (string, error)

// GiteaReissueSpec 은 재발급에 필요한 스택 좌표다.
type GiteaReissueSpec struct {
	StackID   string
	ClusterID string
	Namespace string
	OrgID     string
	Env       string
}

// NewGiteaReissuer 는 GiteaReissuer 를 만든다.
func NewGiteaReissuer(issue GiteaTokenIssueFunc) *GiteaReissuer {
	return &GiteaReissuer{issue: issue}
}

// Reissue 는 Gitea 액세스 토큰을 새로 발급한다.
//
// 발급 경로가 배선되지 않았으면 ErrReissueUnsupported 를 돌려 회전 스케줄러가
// 이 provider 를 건너뛰게 한다 — 조용히 성공을 반환하면 만료된 토큰이 그대로
// 남은 채 회전이 끝난 것으로 기록된다.
func (r *GiteaReissuer) Reissue(ctx context.Context, input ReissueInput) (string, error) {
	if strings.ToLower(strings.TrimSpace(input.Provider)) != "gitea" {
		return "", ErrReissueUnsupported
	}
	if r == nil || r.issue == nil {
		return "", ErrReissueUnsupported
	}

	spec, err := giteaReissueSpec(input.Metadata)
	if err != nil {
		return "", err
	}

	token, err := r.issue(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("gitea 토큰 재발급 실패 (stack %s): %w", spec.StackID, err)
	}
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("gitea 토큰 재발급 결과가 비어 있습니다 (stack %s)", spec.StackID)
	}
	return token, nil
}

// giteaReissueSpec 은 token_sources 의 metadata 에서 스택 좌표를 읽는다.
//
// 하나라도 없으면 끊는다 — 빈 값으로 발급을 시도하면 엉뚱한 네임스페이스의
// 파드에 exec 하거나 다른 스택의 시크릿 경로에 쓰게 된다.
func giteaReissueSpec(metadata map[string]any) (GiteaReissueSpec, error) {
	spec := GiteaReissueSpec{
		StackID:   metadataString(metadata, "stack_id"),
		ClusterID: metadataString(metadata, "cluster_id"),
		Namespace: metadataString(metadata, "namespace"),
		OrgID:     metadataString(metadata, "org_id"),
		Env:       metadataString(metadata, "env"),
	}

	var missing []string
	if spec.StackID == "" {
		missing = append(missing, "stack_id")
	}
	if spec.Namespace == "" {
		missing = append(missing, "namespace")
	}
	if spec.OrgID == "" {
		missing = append(missing, "org_id")
	}
	if len(missing) > 0 {
		return GiteaReissueSpec{}, fmt.Errorf(
			"gitea 토큰 재발급에 필요한 metadata 가 없습니다: %s", strings.Join(missing, ", "))
	}
	return spec, nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if v, ok := metadata[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
