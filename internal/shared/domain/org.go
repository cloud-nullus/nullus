package domain

import (
	"os"
	"strings"
)

// SeededDefaultOrgID 는 시드 마이그레이션이 만드는 기본 조직이다.
//
// 인증이 꺼진 개발 모드에는 principal 도 헤더도 없어 폴백 조직이 필요하다.
// 이 값이 실제 organizations 행과 달라지면 쓰기 경로는 stacks_org_id_fkey
// FK 위반으로 실패하고, 읽기 경로는 빈 결과만 돌려준다.
//
// 이전에 쓰이던 00000000-0000-0000-0000-000000000001 은 어떤 마이그레이션에도
// 존재하지 않는 유령 조직이었다.
const SeededDefaultOrgID = "11111111-1111-1111-1111-111111111111"

// DefaultOrgID 는 폴백 조직 ID 를 반환한다.
// 시드 조직 ID 가 다른 환경에서는 NULLUS_DEFAULT_ORG_ID 로 덮어쓴다.
func DefaultOrgID() string {
	if orgID := strings.TrimSpace(os.Getenv("NULLUS_DEFAULT_ORG_ID")); orgID != "" {
		return orgID
	}
	return SeededDefaultOrgID
}
