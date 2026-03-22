package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

var ErrStackNotFound = errors.New("stack not found")

type AddToolsInput struct {
	StackID string
	Tools   []domain.ToolConfig
}

type AddToolsUseCase struct {
	repo port.StackRepository
}

func NewAddToolsUseCase(repo port.StackRepository) *AddToolsUseCase {
	return &AddToolsUseCase{repo: repo}
}

func (uc *AddToolsUseCase) Execute(ctx context.Context, input AddToolsInput) (*domain.Stack, error) {
	stack, err := uc.repo.FindByID(ctx, input.StackID)
	if err != nil || stack == nil {
		if err == nil {
			err = fmt.Errorf("stack %s not found", input.StackID)
		}
		return nil, fmt.Errorf("%w: %v", ErrStackNotFound, err)
	}

	if err := stack.AddTools(input.Tools); err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateTools(ctx, stack); err != nil {
		return nil, fmt.Errorf("failed to update tools: %w", err)
	}

	return stack, nil
}
