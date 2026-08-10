package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// PostgresSCMConnectionReader 는 port.SCMConnectionReader 를 token_sources
// 테이블로 구현한다.
//
// 외부 SCM 은 토큰과 접속 정보가 한 벌로 묶여야 의미가 있다 — 토큰만 있고
// organization 을 모르면 어디에 리포를 만들지 알 수 없다. 그래서 관리자가
// 토큰을 등록하는 곳(token_sources)의 metadata 에 함께 둔다.
//
// admin 모듈이 소유한 테이블을 직접 읽는다. Modular Monolith 안에서는 허용되는
// 절충이며, 읽기 전용 포트로 좁혀 두었으므로 마이크로서비스로 쪼갤 때는 이
// 구현만 HTTP 클라이언트로 바꾸면 된다.
type PostgresSCMConnectionReader struct {
	pool *pgxpool.Pool
}

// NewPostgresSCMConnectionReader 는 리더를 만든다.
func NewPostgresSCMConnectionReader(pool *pgxpool.Pool) *PostgresSCMConnectionReader {
	return &PostgresSCMConnectionReader{pool: pool}
}

// GetConnection 은 조직에 등록된 SCM 연동 설정을 읽는다.
//
// 등록된 것이 없으면 nil, nil 이다 — 호출부가 "무엇을 등록해야 하는지" 안내를
// 띄울 수 있게 오류와 구분한다.
func (r *PostgresSCMConnectionReader) GetConnection(
	ctx context.Context,
	orgID string,
	platform port.SCMPlatform,
) (*port.SCMConnection, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}

	// 같은 조직에 여러 항목이 남아 있을 수 있다(폐기 예정 토큰, 스택별 등록 등).
	// 가장 최근에 갱신된 활성 항목을 쓴다.
	//
	// owner 가 있는 행만 본다. 같은 provider 로 등록된 다른 용도의 행이 섞이면
	// 소유자를 모르는 행을 집어 "연동은 있는데 리포를 만들 수 없는" 상태가 된다.
	const q = `
		SELECT metadata->>'owner',
		       COALESCE(metadata->>'api_base_url', '')
		FROM token_sources
		WHERE org_id = $1
		  AND provider = $2
		  AND deleted_at IS NULL
		  AND status <> 'revoked'
		  AND COALESCE(metadata->>'owner', '') <> ''
		ORDER BY updated_at DESC
		LIMIT 1`

	var owner, apiBaseURL string
	err := r.pool.QueryRow(ctx, q, orgID, string(platform)).Scan(&owner, &apiBaseURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query %s connection for org %s: %w", platform, orgID, err)
	}

	return &port.SCMConnection{
		Platform:   platform,
		Owner:      strings.TrimSpace(owner),
		APIBaseURL: strings.TrimSpace(apiBaseURL),
	}, nil
}
