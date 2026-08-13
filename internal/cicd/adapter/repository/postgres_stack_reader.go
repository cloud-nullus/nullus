package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// PostgresStackReader implements port.StackReader by querying the stacks
// table directly. This is acceptable within a Modular Monolith since both
// modules share the same database. When splitting into microservices,
// replace this with an HTTP/gRPC client calling the Stack service.
type PostgresStackReader struct {
	pool *pgxpool.Pool
}

// NewPostgresStackReader constructs a PostgresStackReader.
func NewPostgresStackReader(pool *pgxpool.Pool) *PostgresStackReader {
	return &PostgresStackReader{pool: pool}
}

// GetStackSummary retrieves minimal stack information by ID.
// Returns nil and no error if the stack does not exist.
func (r *PostgresStackReader) GetStackSummary(ctx context.Context, stackID string) (*port.StackSummary, error) {
	// 도구 이름과 접근 도메인은 config JSONB 안에 있다. 없을 수 있으므로
	// COALESCE 로 빈 문자열을 돌려준다 — 호출부가 nil 검사를 하지 않아도 되게.
	const q = `
		SELECT id, org_id, cluster_id, state,
		       COALESCE(namespace, ''),
		       COALESCE(config->'artifacts'->'source_repository'->>'name', ''),
		       COALESCE(config->'artifacts'->'container_registry'->>'name', ''),
		       COALESCE(config->>'access_domain', ''),
		       COALESCE(config->'logging'->'trace_exporter'->>'enabled', 'false')
		FROM stacks
		WHERE id = $1`

	var s port.StackSummary
	var collectorEnabled string
	err := r.pool.QueryRow(ctx, q, stackID).Scan(
		&s.ID, &s.OrgID, &s.ClusterID, &s.State,
		&s.Namespace, &s.SourceRepository, &s.ContainerRegistry, &s.AccessDomain,
		&collectorEnabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query stack summary: %w", err)
	}
	// 수집기를 고른 스택에만 주소를 준다. 없는 스택에 주소를 주면 배포된 앱이
	// 닿지 않는 곳으로 계속 재시도하며 오류 로그만 쌓는다.
	//
	// 주소 조립 규칙은 shared 가 소유한다 — 모듈끼리 서로의 internal 을 참조할 수
	// 없으므로 stack 과 cicd 가 같은 함수를 본다.
	if collectorEnabled == "true" {
		s.OTLPEndpoint = shareddomain.OTelCollectorOTLPGRPCEndpoint(s.Namespace)
	}

	return &s, nil
}
