package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// CICDTemplateHandler handles HTTP requests for CI/CD pipeline template operations.
type CICDTemplateHandler struct {
	templateRepo port.PipelineTemplateRepository
}

// NewCICDTemplateHandler constructs a CICDTemplateHandler.
func NewCICDTemplateHandler(templateRepo port.PipelineTemplateRepository) *CICDTemplateHandler {
	return &CICDTemplateHandler{templateRepo: templateRepo}
}

// RegisterRoutes registers CI/CD template routes on the given Echo group.
func (h *CICDTemplateHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/templates", h.ListTemplates)
	g.GET("/templates/:id", h.GetTemplate)
	g.POST("/templates", h.CreateTemplate)
	g.PUT("/templates/:id", h.UpdateTemplate)
	g.DELETE("/templates/:id", h.DeleteTemplate)
}

// cicdTemplateRequest 는 생성·수정 요청 본문이다.
//
// 빌드 설정과 설명은 포인터·nil 가능 맵으로 받아 "안 보냄"과 "빈 값으로 지움"을 가른다.
// 템플릿 화면은 이름·설명·유형·단계만 보내므로, 수정 때 요청에 없는 필드는 저장된 값을 그대로 둔다 —
// 그렇지 않으면 시드 템플릿의 이름만 고쳐도 Dockerfile 경로와 환경 변수가 지워진다.
type cicdTemplateRequest struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    *string           `json:"description"`
	AppType        string            `json:"app_type"`
	Stages         []string          `json:"stages"`
	GitRepoURL     *string           `json:"git_repo_url"`
	DockerfilePath *string           `json:"dockerfile_path"`
	DockerContext  *string           `json:"docker_context"`
	EnvVars        map[string]string `json:"env_vars"`
	CreatedBy      string            `json:"created_by"`
}

// ListTemplates handles GET /api/v1/cicd/templates.
func (h *CICDTemplateHandler) ListTemplates(c echo.Context) error {
	templates, err := h.templateRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "CICD_TEMPLATE_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, templates)
}

// GetTemplate handles GET /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) GetTemplate(c echo.Context) error {
	id := c.Param("id")

	tmpl, err := h.templateRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return templateErrorResponse(c, err, "CICD_TEMPLATE_GET_FAILED")
	}

	return c.JSON(http.StatusOK, tmpl)
}

// CreateTemplate handles POST /api/v1/cicd/templates.
func (h *CICDTemplateHandler) CreateTemplate(c echo.Context) error {
	var req cicdTemplateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	tmpl := &domain.PipelineTemplate{
		ID:             req.ID,
		Name:           req.Name,
		Description:    derefOrEmpty(req.Description),
		AppType:        domain.AppType(req.AppType),
		Stages:         req.Stages,
		GitRepoURL:     derefOrEmpty(req.GitRepoURL),
		DockerfilePath: derefOrEmpty(req.DockerfilePath),
		DockerContext:  derefOrEmpty(req.DockerContext),
		EnvVars:        req.EnvVars,
		CreatedBy:      req.CreatedBy,
	}
	if err := tmpl.Validate(); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	if err := h.templateRepo.Create(c.Request().Context(), tmpl); err != nil {
		return templateErrorResponse(c, err, "CICD_TEMPLATE_CREATE_FAILED")
	}

	return c.JSON(http.StatusCreated, tmpl)
}

// UpdateTemplate handles PUT /api/v1/cicd/templates/:id.
//
// 경로의 ID 가 대상이다. 저장된 템플릿을 읽어 요청에 있는 필드만 덧씌운다. 만든 사람은 바꾸지 않는다.
func (h *CICDTemplateHandler) UpdateTemplate(c echo.Context) error {
	var req cicdTemplateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	ctx := c.Request().Context()
	tmpl, err := h.templateRepo.GetByID(ctx, c.Param("id"))
	if err != nil {
		return templateErrorResponse(c, err, "CICD_TEMPLATE_UPDATE_FAILED")
	}

	if req.Name != "" {
		tmpl.Name = req.Name
	}
	if req.Description != nil {
		tmpl.Description = *req.Description
	}
	if req.AppType != "" {
		tmpl.AppType = domain.AppType(req.AppType)
	}
	if req.Stages != nil {
		tmpl.Stages = req.Stages
	}
	if req.GitRepoURL != nil {
		tmpl.GitRepoURL = *req.GitRepoURL
	}
	if req.DockerfilePath != nil {
		tmpl.DockerfilePath = *req.DockerfilePath
	}
	if req.DockerContext != nil {
		tmpl.DockerContext = *req.DockerContext
	}
	if req.EnvVars != nil {
		tmpl.EnvVars = req.EnvVars
	}
	if err := tmpl.Validate(); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	if err := h.templateRepo.Update(ctx, tmpl); err != nil {
		return templateErrorResponse(c, err, "CICD_TEMPLATE_UPDATE_FAILED")
	}

	return c.JSON(http.StatusOK, tmpl)
}

// DeleteTemplate handles DELETE /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) DeleteTemplate(c echo.Context) error {
	id := c.Param("id")

	if err := h.templateRepo.Delete(c.Request().Context(), id); err != nil {
		return templateErrorResponse(c, err, "CICD_TEMPLATE_DELETE_FAILED")
	}

	return c.NoContent(http.StatusNoContent)
}

// templateErrorResponse 는 저장소 에러를 상태 코드로 옮긴다. 없으면 404, 겹치면 409, 그 밖은 500 과 fallbackCode.
func templateErrorResponse(c echo.Context, err error, fallbackCode string) error {
	switch {
	case errors.Is(err, domain.ErrTemplateNotFound):
		return errorResponse(c, http.StatusNotFound, "CICD_TEMPLATE_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrTemplateAlreadyExists):
		return errorResponse(c, http.StatusConflict, "CICD_TEMPLATE_ALREADY_EXISTS", err.Error())
	default:
		return errorResponse(c, http.StatusInternalServerError, fallbackCode, err.Error())
	}
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
