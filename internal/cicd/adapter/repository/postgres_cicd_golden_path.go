package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresCICDGoldenPathRepository 는 cicd_golden_paths 표를 읽고 쓴다.
// 종전 구현(MemoryCICDGoldenPathRepository)은 프로세스 메모리에 세 건을 박아 두어
// 화면의 생성·수정·삭제가 재시작과 함께 사라졌다.
type PostgresCICDGoldenPathRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCICDGoldenPathRepository(pool *pgxpool.Pool) *PostgresCICDGoldenPathRepository {
	return &PostgresCICDGoldenPathRepository{pool: pool}
}

// goldenPathColumns 는 GetByID 와 List 가 같은 순서로 읽는 열이다. scanGoldenPath 와 맞춘다.
const goldenPathColumns = `
		id, name, COALESCE(description, ''), COALESCE(tools, '[]'::jsonb),
		estimated_install_time, COALESCE(recommended_use_case, ''),
		COALESCE(min_resources, ''), created_at`

func (r *PostgresCICDGoldenPathRepository) GetByID(ctx context.Context, id string) (*domain.CICDGoldenPath, error) {
	const q = `SELECT ` + goldenPathColumns + `
		FROM cicd_golden_paths
		WHERE id = $1`

	gp, err := scanGoldenPath(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, goldenPathNotFound(id)
		}
		return nil, err
	}

	return gp, nil
}

// List 는 카드가 놓이는 순서(sort_order)대로 돌려준다. 맵을 돌던 종전 구현은
// 요청마다 순서가 바뀌어 고른 카드를 다시 찾기 어려웠다.
func (r *PostgresCICDGoldenPathRepository) List(ctx context.Context) ([]*domain.CICDGoldenPath, error) {
	const q = `SELECT ` + goldenPathColumns + `
		FROM cicd_golden_paths
		ORDER BY sort_order ASC, id ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	goldenPaths := make([]*domain.CICDGoldenPath, 0)
	for rows.Next() {
		gp, err := scanGoldenPath(rows)
		if err != nil {
			return nil, err
		}
		goldenPaths = append(goldenPaths, gp)
	}

	return goldenPaths, rows.Err()
}

// Create 는 새 골든패스를 넣는다. 같은 ID 가 있으면 덮어쓰지 않고 domain.ErrGoldenPathAlreadyExists 를
// 돌려준다. 순서는 맨 뒤에 붙인다 — 기본 세 건 사이에 끼어들면 설치 안내 순서가 흐트러진다.
func (r *PostgresCICDGoldenPathRepository) Create(ctx context.Context, goldenPath *domain.CICDGoldenPath) error {
	const q = `
		INSERT INTO cicd_golden_paths
			(id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7,
			COALESCE((SELECT MAX(sort_order) FROM cicd_golden_paths), 0) + 1)`

	tools, err := marshalGoldenPathTools(goldenPath)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, q,
		goldenPath.ID, goldenPath.Name, goldenPath.Description, tools,
		goldenPath.EstimatedInstallTime, goldenPath.RecommendedUseCase, goldenPath.MinResources)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("cicd golden path %q: %w", goldenPath.ID, domain.ErrGoldenPathAlreadyExists)
		}
		return err
	}

	return nil
}

// Update 는 기존 골든패스를 덮어쓴다. 없으면 만들지 않고 domain.ErrGoldenPathNotFound 를 돌려준다.
// sort_order 는 건드리지 않는다 — 내용을 고쳤다고 카드 위치가 바뀌면 안 된다.
func (r *PostgresCICDGoldenPathRepository) Update(ctx context.Context, goldenPath *domain.CICDGoldenPath) error {
	const q = `
		UPDATE cicd_golden_paths
		SET name = $2, description = $3, tools = $4, estimated_install_time = $5,
			recommended_use_case = $6, min_resources = $7, updated_at = NOW()
		WHERE id = $1`

	tools, err := marshalGoldenPathTools(goldenPath)
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx, q,
		goldenPath.ID, goldenPath.Name, goldenPath.Description, tools,
		goldenPath.EstimatedInstallTime, goldenPath.RecommendedUseCase, goldenPath.MinResources)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return goldenPathNotFound(goldenPath.ID)
	}

	return nil
}

func (r *PostgresCICDGoldenPathRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM cicd_golden_paths WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return goldenPathNotFound(id)
	}

	return nil
}

func goldenPathNotFound(id string) error {
	return fmt.Errorf("cicd golden path %q: %w", id, domain.ErrGoldenPathNotFound)
}

// marshalGoldenPathTools 는 도구 목록을 jsonb 로 옮긴다. nil 은 빈 배열로 둔다 —
// null 이 들어가면 화면이 목록을 돌리다 멈춘다.
func marshalGoldenPathTools(goldenPath *domain.CICDGoldenPath) ([]byte, error) {
	tools := goldenPath.Tools
	if tools == nil {
		tools = []domain.CICDTool{}
	}

	encoded, err := json.Marshal(tools)
	if err != nil {
		return nil, fmt.Errorf("cicd golden path %q tools: %w", goldenPath.ID, err)
	}

	return encoded, nil
}

// rowScanner 는 QueryRow 와 Rows 가 함께 만족하는 부분이다.
type goldenPathRowScanner interface {
	Scan(dest ...any) error
}

func scanGoldenPath(row goldenPathRowScanner) (*domain.CICDGoldenPath, error) {
	var (
		gp    domain.CICDGoldenPath
		tools []byte
	)

	if err := row.Scan(
		&gp.ID, &gp.Name, &gp.Description, &tools,
		&gp.EstimatedInstallTime, &gp.RecommendedUseCase,
		&gp.MinResources, &gp.CreatedAt,
	); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(tools, &gp.Tools); err != nil {
		return nil, fmt.Errorf("cicd golden path %q tools: %w", gp.ID, err)
	}

	return &gp, nil
}
