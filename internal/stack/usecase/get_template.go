package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// GetTemplateInput holds parameters for retrieving a single template.
type GetTemplateInput struct {
	ID string
}

// GetTemplateOutput holds the result of retrieving a template.
type GetTemplateOutput struct {
	Template *domain.Template
}

// GetTemplate retrieves a single Golden Path template by ID.
type GetTemplate struct {
	templateRepo port.TemplateRepository
}

// NewGetTemplate constructs a GetTemplate use case.
func NewGetTemplate(templateRepo port.TemplateRepository) *GetTemplate {
	return &GetTemplate{templateRepo: templateRepo}
}

// Execute retrieves a template by ID.
func (uc *GetTemplate) Execute(ctx context.Context, input GetTemplateInput) (*GetTemplateOutput, error) {
	if input.ID == "" {
		return nil, fmt.Errorf("template id is required")
	}

	tmpl, err := uc.templateRepo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	return &GetTemplateOutput{Template: tmpl}, nil
}

// ListTemplatesOutput holds the result of listing templates.
type ListTemplatesOutput struct {
	Templates []*domain.Template
}

// ListTemplates retrieves all available Golden Path templates.
type ListTemplates struct {
	templateRepo port.TemplateRepository
}

// NewListTemplates constructs a ListTemplates use case.
func NewListTemplates(templateRepo port.TemplateRepository) *ListTemplates {
	return &ListTemplates{templateRepo: templateRepo}
}

// Execute lists all templates.
func (uc *ListTemplates) Execute(ctx context.Context) (*ListTemplatesOutput, error) {
	templates, err := uc.templateRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}

	return &ListTemplatesOutput{Templates: templates}, nil
}
