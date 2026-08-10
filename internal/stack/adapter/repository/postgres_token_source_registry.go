package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type PostgresTokenSourceRegistry struct {
	pool   *pgxpool.Pool
	secret *secrets.Router
}

func NewPostgresTokenSourceRegistry(pool *pgxpool.Pool, secret *secrets.Router) *PostgresTokenSourceRegistry {
	return &PostgresTokenSourceRegistry{pool: pool, secret: secret}
}

func (r *PostgresTokenSourceRegistry) Upsert(ctx context.Context, input port.TokenSourceInput) error {
	manager := strings.TrimSpace(input.SecretManager)
	if manager == "" {
		manager = "openbao"
	}
	if r.secret != nil && strings.TrimSpace(input.TokenValue) != "" {
		if err := r.putToken(ctx, manager, input); err != nil {
			return err
		}
	}

	extra, err := json.Marshal(nonEmptyMetadata(input.Metadata))
	if err != nil {
		return fmt.Errorf("marshal token source metadata: %w", err)
	}

	// 고정 필드 위에 호출자 metadata 를 얹는다. 순서가 반대면 secret_manager
	// 같은 값이 덮여 회전 컨트롤러가 저장소를 찾지 못한다.
	const q = `
		INSERT INTO token_sources (org_id, module, provider, path, token_type, status, next_check_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '24 hours',
			jsonb_build_object('secret_manager', $7::text, 'cluster_id', $8::text, 'namespace', $9::text)
				|| $10::jsonb)
		ON CONFLICT (org_id, provider, path) WHERE deleted_at IS NULL
		DO UPDATE SET
			module = EXCLUDED.module,
			token_type = EXCLUDED.token_type,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			next_check_at = EXCLUDED.next_check_at,
			updated_at = now()`
	_, err = r.pool.Exec(ctx, q, input.OrgID, input.Module, input.Provider, input.Path,
		input.TokenType, input.Status, manager, input.ClusterID, input.Namespace, string(extra))
	return err
}

// putToken 은 토큰 값을 시크릿 저장소에 기록한다.
//
// StackID 가 있으면 그 스택의 저장소에 쓴다. OpenBao 는 스택마다 배포되므로
// 전역 저장소에 써 두면 스택 범위로 읽는 쪽이 값을 찾지 못한다.
func (r *PostgresTokenSourceRegistry) putToken(
	ctx context.Context,
	manager string,
	input port.TokenSourceInput,
) error {
	if stackID := strings.TrimSpace(input.StackID); stackID != "" {
		return r.secret.PutTokenForStack(ctx, manager, stackID, input.Path, input.TokenValue)
	}
	return r.secret.PutToken(ctx, manager, input.Path, input.TokenValue)
}

// nonEmptyMetadata 는 빈 값을 걸러낸다.
//
// 빈 문자열을 그대로 넣으면 조회 쪽에서 "설정됨" 으로 보여, 예를 들어 owner
// 가 비어 있는데도 연동이 등록된 것처럼 취급된다.
func nonEmptyMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}
