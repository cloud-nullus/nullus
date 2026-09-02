// Package bridge 는 다른 모듈의 공개 인터페이스로만 통하는 얇은 어댑터다.
//
// backup 모듈은 다른 모듈의 테이블을 직접 읽지 않는다 (CLAUDE.md 모듈 경계).
// 참조 정합성 검사(§6.4)가 token_sources 를 훑어야 하므로, admin 모듈의
// 공개 Repository 인터페이스를 감싸 backup 의 포트 모양으로 바꾼다.
package bridge

import (
	"context"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// AdminTokenSourceRepo 는 admin 모듈이 제공하는 창구다.
type AdminTokenSourceRepo interface {
	ListSources(ctx context.Context, orgID string) ([]*admindomain.TokenSource, error)
}

type TokenSourceLister struct {
	repo AdminTokenSourceRepo
}

func NewTokenSourceLister(repo AdminTokenSourceRepo) *TokenSourceLister {
	return &TokenSourceLister{repo: repo}
}

func (l *TokenSourceLister) ListPaths(ctx context.Context, orgID string) ([]port.TokenSourceRef, error) {
	if l.repo == nil {
		return nil, nil
	}
	items, err := l.repo.ListSources(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]port.TokenSourceRef, 0, len(items))
	for _, it := range items {
		if it.Path == "" {
			continue
		}
		out = append(out, port.TokenSourceRef{ID: it.ID, Path: it.Path})
	}
	return out, nil
}

var _ port.TokenSourceLister = (*TokenSourceLister)(nil)
