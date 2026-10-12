package bridge

import (
	"context"
	"fmt"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
)

// AdminOrgRepo 는 admin 모듈이 제공하는 조직 창구다.
//
// admin 의 저장소는 없는 조직에 (nil, nil) 을 돌려준다 — 오류가 아니라 "없음" 이다.
type AdminOrgRepo interface {
	GetByID(ctx context.Context, id string) (*admindomain.Organization, error)
}

// OrganizationChecker 는 port.OrganizationChecker 를 admin 저장소로 구현한다.
type OrganizationChecker struct {
	repo AdminOrgRepo
}

func NewOrganizationChecker(repo AdminOrgRepo) *OrganizationChecker {
	return &OrganizationChecker{repo: repo}
}

// OrganizationExists 는 조직이 있으면 true 다.
//
// 저장소가 배선되지 않았으면 모른다고 답한다(오류). 조용히 true 를 돌려주면
// 검증이 빠진 채로 통과하고, false 면 멀쩡한 조직을 거절한다 — 둘 다 조용한 실패다.
func (c *OrganizationChecker) OrganizationExists(ctx context.Context, orgID string) (bool, error) {
	if c == nil || c.repo == nil {
		return false, fmt.Errorf("조직 저장소가 배선되지 않아 조직 %s 을 확인할 수 없습니다", orgID)
	}
	org, err := c.repo.GetByID(ctx, orgID)
	if err != nil {
		return false, err
	}
	return org != nil, nil
}
