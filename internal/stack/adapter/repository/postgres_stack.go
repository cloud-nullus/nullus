package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// PostgresStackRepository implements port.StackRepository using a *sql.DB.
// It uses the standard database/sql interface so it works with any PostgreSQL driver.
type PostgresStackRepository struct {
	db *sql.DB
}

// NewPostgresStackRepository constructs a PostgresStackRepository.
func NewPostgresStackRepository(db *sql.DB) *PostgresStackRepository {
	return &PostgresStackRepository{db: db}
}

// Create inserts a new stack record.
func (r *PostgresStackRepository) Create(ctx context.Context, stack *domain.Stack) error {
	configJSON, err := json.Marshal(stack.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	const q = `
		INSERT INTO stacks (id, name, template_id, org_id, cluster_id, state, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = r.db.ExecContext(ctx, q,
		stack.ID,
		stack.Name,
		stack.TemplateID,
		stack.OrgID,
		stack.ClusterID,
		string(stack.State),
		configJSON,
		stack.CreatedAt,
		stack.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert stack: %w", err)
	}
	return nil
}

// GetByID retrieves a stack by its ID.
func (r *PostgresStackRepository) GetByID(ctx context.Context, id string) (*domain.Stack, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, state, config, created_at, updated_at
		FROM stacks WHERE id = $1`

	row := r.db.QueryRowContext(ctx, q, id)
	return scanStack(row)
}

// List returns all stacks belonging to an organization.
func (r *PostgresStackRepository) List(ctx context.Context, orgID string) ([]*domain.Stack, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, state, config, created_at, updated_at
		FROM stacks WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100`

	rows, err := r.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("query stacks: %w", err)
	}
	defer rows.Close()

	var stacks []*domain.Stack
	for rows.Next() {
		s, err := scanStackRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan stack: %w", err)
		}
		stacks = append(stacks, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return stacks, nil
}

// Update persists changes to an existing stack.
func (r *PostgresStackRepository) Update(ctx context.Context, stack *domain.Stack) error {
	configJSON, err := json.Marshal(stack.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	stack.UpdatedAt = time.Now()

	const q = `
		UPDATE stacks
		SET name = $2, template_id = $3, cluster_id = $4, state = $5, config = $6, updated_at = $7
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q,
		stack.ID,
		stack.Name,
		stack.TemplateID,
		stack.ClusterID,
		string(stack.State),
		configJSON,
		stack.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update stack: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("stack %q not found", stack.ID)
	}
	return nil
}

// Delete removes a stack by ID.
func (r *PostgresStackRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM stacks WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("stack %q not found", id)
	}
	return nil
}

// rowScanner abstracts *sql.Row and *sql.Rows for scanStack.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanStack(row rowScanner) (*domain.Stack, error) {
	return scanStackRow(row)
}

func scanStackRow(row rowScanner) (*domain.Stack, error) {
	var (
		s          domain.Stack
		state      string
		configJSON []byte
		createdAt  time.Time
		updatedAt  time.Time
	)

	if err := row.Scan(
		&s.ID,
		&s.Name,
		&s.TemplateID,
		&s.OrgID,
		&s.ClusterID,
		&state,
		&configJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	s.State = domain.DeploymentState(state)
	s.CreatedAt = createdAt
	s.UpdatedAt = updatedAt

	var cfg domain.StackConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	s.Config = cfg

	return &s, nil
}
