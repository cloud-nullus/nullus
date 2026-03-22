package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStackRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresStackRepository(pool *pgxpool.Pool) *PostgresStackRepository {
	return &PostgresStackRepository{pool: pool}
}

func (r *PostgresStackRepository) Create(ctx context.Context, stack *domain.Stack) error {
	configJSON, err := json.Marshal(stack.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	const q = `
		INSERT INTO stacks (id, name, template_id, org_id, cluster_id, namespace, state, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err = r.pool.Exec(ctx, q,
		stack.ID,
		stack.Name,
		stack.TemplateID,
		stack.OrgID,
		stack.ClusterID,
		stack.Namespace,
		string(stack.State),
		configJSON,
		stack.CreatedAt,
		stack.UpdatedAt,
	)
	return err
}

func (r *PostgresStackRepository) GetByID(ctx context.Context, id string) (*domain.Stack, error) {
	return r.FindByID(ctx, id)
}

func (r *PostgresStackRepository) FindByID(ctx context.Context, id string) (*domain.Stack, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, namespace, state, config, created_at, updated_at
		FROM stacks WHERE id = $1`

	stack, configJSON, err := r.scanStackWithConfig(r.pool.QueryRow(ctx, q, id))
	if err != nil || stack == nil {
		return stack, err
	}
	stack.Tools = parseToolsFromConfig(configJSON)
	return stack, nil
}

func (r *PostgresStackRepository) List(ctx context.Context, orgID string) ([]*domain.Stack, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, namespace, state, config, created_at, updated_at
		FROM stacks WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("query stacks: %w", err)
	}
	defer rows.Close()

	var stacks []*domain.Stack
	for rows.Next() {
		s, err := r.scanStack(rows)
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

	ct, err := r.pool.Exec(ctx, q,
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
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("stack %q not found", stack.ID)
	}
	return nil
}

func (r *PostgresStackRepository) UpdateTools(ctx context.Context, stack *domain.Stack) error {
	const selectQ = `SELECT config FROM stacks WHERE id = $1`

	var configJSON []byte
	if err := r.pool.QueryRow(ctx, selectQ, stack.ID).Scan(&configJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("stack %q not found", stack.ID)
		}
		return fmt.Errorf("load stack config: %w", err)
	}

	configMap := map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &configMap); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
	}

	tools := make([]map[string]string, 0, len(stack.Tools))
	for _, tool := range stack.Tools {
		name := tool.Tool
		if name == "" {
			name = tool.Name
		}
		version := tool.Version
		if version == "" {
			if tool.AppVersion != "" {
				version = tool.AppVersion
			} else {
				version = tool.HelmVersion
			}
		}
		tools = append(tools, map[string]string{
			"category": tool.Category,
			"tool":     name,
			"version":  version,
		})
	}
	configMap["tools"] = tools

	mergedJSON, err := json.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	stack.UpdatedAt = time.Now()

	const updateQ = `
		UPDATE stacks
		SET config = $2, updated_at = $3
		WHERE id = $1`

	ct, err := r.pool.Exec(ctx, updateQ, stack.ID, mergedJSON, stack.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update stack tools: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("stack %q not found", stack.ID)
	}

	return nil
}

func (r *PostgresStackRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM stacks WHERE id = $1`

	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("stack %q not found", id)
	}
	return nil
}

type pgxScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresStackRepository) scanStack(row pgxScanner) (*domain.Stack, error) {
	stack, _, err := r.scanStackWithConfig(row)
	return stack, err
}

func (r *PostgresStackRepository) scanStackWithConfig(row pgxScanner) (*domain.Stack, []byte, error) {
	var (
		s          domain.Stack
		state      string
		configJSON []byte
	)

	if err := row.Scan(
		&s.ID,
		&s.Name,
		&s.TemplateID,
		&s.OrgID,
		&s.ClusterID,
		&s.Namespace,
		&state,
		&configJSON,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	s.State = domain.DeploymentState(state)

	var cfg domain.StackConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, nil, fmt.Errorf("unmarshal config: %w", err)
	}
	s.Config = cfg

	return &s, configJSON, nil
}

func parseToolsFromConfig(configJSON []byte) []domain.ToolConfig {
	if len(configJSON) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(configJSON, &raw); err != nil {
		return nil
	}

	toolsRaw, ok := raw["tools"]
	if !ok {
		return nil
	}

	var tools []domain.ToolConfig
	if err := json.Unmarshal(toolsRaw, &tools); err == nil {
		for i := range tools {
			if tools[i].Tool == "" {
				tools[i].Tool = tools[i].Name
			}
			if tools[i].Name == "" {
				tools[i].Name = tools[i].Tool
			}
			if tools[i].Version == "" {
				if tools[i].AppVersion != "" {
					tools[i].Version = tools[i].AppVersion
				} else {
					tools[i].Version = tools[i].HelmVersion
				}
			}
			if tools[i].AppVersion == "" {
				tools[i].AppVersion = tools[i].Version
			}
			if tools[i].HelmVersion == "" {
				tools[i].HelmVersion = tools[i].Version
			}
		}
	}

	return tools
}
