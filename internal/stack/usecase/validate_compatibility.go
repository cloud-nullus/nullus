package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ValidateCompatibilityInput holds the tool combination to validate.
type ValidateCompatibilityInput struct {
	// Tools maps tool category to tool name, e.g. {"ci_platform": "GitLab CI"}.
	Tools map[string]string
}

// ValidationIssue represents a detailed compatibility finding.
type ValidationIssue struct {
	Tool     string `json:"tool"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Code     string `json:"code,omitempty"`
}

// ValidationOverall represents the rolled-up compatibility state.
type ValidationOverall struct {
	State string `json:"state"`
	Score int    `json:"score"`
}

// ValidateCompatibilityOutput holds the result of a compatibility validation.
type ValidateCompatibilityOutput struct {
	Compatible bool
	Matrix     *domain.CompatibilityMatrix
	Message    string
	Overall    ValidationOverall
	Issues     []ValidationIssue
	CheckedAt  time.Time
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

	checkedAt := time.Now().UTC()
	matrix, err := uc.repo.Validate(ctx, input.Tools)
	if err != nil {
		return &ValidateCompatibilityOutput{
			Compatible: false,
			Message:    "no compatible matrix found for the given tool combination",
			Overall: ValidationOverall{
				State: "fail",
				Score: 0,
			},
			Issues: []ValidationIssue{{
				Tool:     "matrix",
				Message:  "No matching compatibility matrix for requested tools",
				Severity: "error",
				Code:     "MATRIX_NOT_FOUND",
			}},
			CheckedAt: checkedAt,
		}, nil
	}

	overall, issues := evaluateMatrixStatus(matrix.Status)

	return &ValidateCompatibilityOutput{
		Compatible: overall.State != "fail",
		Matrix:     matrix,
		Message:    fmt.Sprintf("tool combination matches matrix %q (status: %s)", matrix.Name, matrix.Status),
		Overall:    overall,
		Issues:     issues,
		CheckedAt:  checkedAt,
	}, nil
}

func evaluateMatrixStatus(status string) (ValidationOverall, []ValidationIssue) {
	switch status {
	case "verified":
		return ValidationOverall{State: "pass", Score: 100}, nil
	case "untested":
		return ValidationOverall{State: "warn", Score: 70}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  "Matched matrix is untested; proceed with caution",
			Severity: "warning",
			Code:     "MATRIX_UNTESTED",
		}}
	case "unsupported":
		return ValidationOverall{State: "fail", Score: 0}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  "Matched matrix is marked as unsupported",
			Severity: "error",
			Code:     "MATRIX_UNSUPPORTED",
		}}
	default:
		return ValidationOverall{State: "warn", Score: 50}, []ValidationIssue{{
			Tool:     "matrix",
			Message:  fmt.Sprintf("Matched matrix has unknown status %q", status),
			Severity: "warning",
			Code:     "MATRIX_STATUS_UNKNOWN",
		}}
	}
}
