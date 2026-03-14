package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ValidateCompatibilityInput holds the tool combination to validate.
type ValidateCompatibilityInput struct {
	// Tools maps tool category to tool name, e.g. {"ci_platform": "GitLab CI"}.
	Tools map[string]string
}

// ValidateCompatibilityOutput holds the result of a compatibility validation.
type ValidateCompatibilityOutput struct {
	Compatible bool
	Matrix     *domain.CompatibilityMatrix
	Message    string
}

// ValidateCompatibility checks whether a given tool combination matches a known matrix.
type ValidateCompatibility struct {
	repo port.CompatibilityRepository
}

// NewValidateCompatibility constructs a ValidateCompatibility use case.
func NewValidateCompatibility(repo port.CompatibilityRepository) *ValidateCompatibility {
	return &ValidateCompatibility{repo: repo}
}

// Execute validates the tool combination and returns the matching matrix if found.
func (uc *ValidateCompatibility) Execute(ctx context.Context, input ValidateCompatibilityInput) (*ValidateCompatibilityOutput, error) {
	if len(input.Tools) == 0 {
		return nil, fmt.Errorf("tools map must not be empty")
	}

	matrix, err := uc.repo.Validate(ctx, input.Tools)
	if err != nil {
		return &ValidateCompatibilityOutput{
			Compatible: false,
			Message:    "no compatible matrix found for the given tool combination",
		}, nil
	}

	return &ValidateCompatibilityOutput{
		Compatible: true,
		Matrix:     matrix,
		Message:    fmt.Sprintf("tool combination matches matrix %q (status: %s)", matrix.Name, matrix.Status),
	}, nil
}
