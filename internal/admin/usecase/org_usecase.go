package usecase

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// OrgUseCase handles Organization business logic.
type OrgUseCase struct {
	orgRepo port.OrgRepository
}

// NewOrgUseCase creates a new OrgUseCase.
func NewOrgUseCase(orgRepo port.OrgRepository) *OrgUseCase {
	return &OrgUseCase{orgRepo: orgRepo}
}

// CreateOrgInput holds the input for creating an organization.
type CreateOrgInput struct {
	Name   string
	Slug   string
	Domain string
}

// UpdateOrgInput holds the input for updating an organization.
type UpdateOrgInput struct {
	Name               string
	Domain             string
	ClusterAccessScope []string
}

// CreateOrg creates a new organization after validating the slug.
func (uc *OrgUseCase) CreateOrg(ctx context.Context, input CreateOrgInput) (*domain.Organization, error) {
	org := &domain.Organization{
		Slug: input.Slug,
	}
	if err := org.ValidateSlug(); err != nil {
		return nil, &shareddomain.AppError{
			Code:       "ORG_CREATE_INVALID_SLUG",
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "Invalid slug format",
			Detail:     err.Error(),
			Retryable:  false,
		}
	}

	existing, err := uc.orgRepo.GetBySlug(ctx, input.Slug)
	if err != nil {
		return nil, fmt.Errorf("checking slug uniqueness: %w", err)
	}
	if existing != nil {
		return nil, &shareddomain.AppError{
			Code:       "ORG_CREATE_SLUG_DUPLICATE",
			HTTPStatus: http.StatusConflict,
			Message:    "Organization slug already exists",
			Detail:     fmt.Sprintf("slug %q is already taken", input.Slug),
			Retryable:  false,
		}
	}

	now := time.Now().UTC()
	org = &domain.Organization{
		ID:        uuid.New().String(),
		Name:      input.Name,
		Slug:      input.Slug,
		Domain:    input.Domain,
		Status:    domain.OrgStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := uc.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}

	return org, nil
}

// GetOrg retrieves an organization by ID.
func (uc *OrgUseCase) GetOrg(ctx context.Context, id string) (*domain.Organization, error) {
	org, err := uc.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}
	if org == nil {
		return nil, &shareddomain.AppError{
			Code:       "ORG_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "Organization not found",
			Detail:     fmt.Sprintf("organization with id %q does not exist", id),
			Retryable:  false,
		}
	}
	return org, nil
}

func (uc *OrgUseCase) GetFirstOrg(ctx context.Context) (*domain.Organization, error) {
	orgs, err := uc.orgRepo.List(ctx, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	if len(orgs) == 0 {
		return nil, &shareddomain.AppError{
			Code:       "ORG_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "Organization not found",
			Detail:     "no organization exists",
			Retryable:  false,
		}
	}

	return orgs[0], nil
}

// UpdateOrg updates an organization's mutable fields.
func (uc *OrgUseCase) UpdateOrg(ctx context.Context, id string, input UpdateOrgInput) (*domain.Organization, error) {
	org, err := uc.GetOrg(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		org.Name = input.Name
	}
	if input.Domain != "" {
		org.Domain = input.Domain
	}
	if input.ClusterAccessScope != nil {
		org.ClusterAccessScope = input.ClusterAccessScope
	}
	org.UpdatedAt = time.Now().UTC()

	if err := uc.orgRepo.Update(ctx, org); err != nil {
		return nil, fmt.Errorf("updating organization: %w", err)
	}

	return org, nil
}

func (uc *OrgUseCase) UpdateFirstOrg(ctx context.Context, input UpdateOrgInput) (*domain.Organization, error) {
	org, err := uc.GetFirstOrg(ctx)
	if err != nil {
		return nil, err
	}

	return uc.UpdateOrg(ctx, org.ID, input)
}
