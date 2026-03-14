package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
	"github.com/labstack/echo/v4"
)

// PipelineHandler handles HTTP requests for pipeline operations.
type PipelineHandler struct {
	createPipeline *usecase.CreatePipeline
	listPipelines  *usecase.ListPipelines
	deployPipeline *usecase.DeployPipeline
	pipelineRepo   port.PipelineRepository
	deploymentRepo port.DeploymentRepository
}

// NewPipelineHandler constructs a PipelineHandler.
func NewPipelineHandler(
	createPipeline *usecase.CreatePipeline,
	listPipelines *usecase.ListPipelines,
	deployPipeline *usecase.DeployPipeline,
	pipelineRepo port.PipelineRepository,
	deploymentRepo port.DeploymentRepository,
) *PipelineHandler {
	return &PipelineHandler{
		createPipeline: createPipeline,
		listPipelines:  listPipelines,
		deployPipeline: deployPipeline,
		pipelineRepo:   pipelineRepo,
		deploymentRepo: deploymentRepo,
	}
}

// RegisterRoutes registers pipeline routes on the given Echo group.
func (h *PipelineHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/pipelines", h.CreatePipeline)
	g.GET("/pipelines", h.ListPipelines)
	g.GET("/pipelines/:id", h.GetPipeline)
	g.POST("/pipelines/:id/deploy", h.DeployPipeline)
	g.GET("/pipelines/:id/deployments", h.ListDeployments)
}

// createPipelineRequest is the request body for POST /pipelines.
type createPipelineRequest struct {
	Name       string `json:"name"`
	TemplateID string `json:"template_id"`
	ClusterID  string `json:"cluster_id"`
	Namespace  string `json:"namespace"`
	AppType    string `json:"app_type"`
	GitRepoURL string `json:"git_repo_url"`
}

// CreatePipeline handles POST /api/v1/pipelines.
func (h *PipelineHandler) CreatePipeline(c echo.Context) error {
	var req createPipelineRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_CONFIG_INVALID", err.Error())
	}

	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "org_default"
	}

	out, err := h.createPipeline.Execute(c.Request().Context(), usecase.CreatePipelineInput{
		Name:       req.Name,
		TemplateID: req.TemplateID,
		OrgID:      orgID,
		ClusterID:  req.ClusterID,
		Namespace:  req.Namespace,
		AppType:    domain.AppType(req.AppType),
		GitRepoURL: req.GitRepoURL,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_CONFIG_INVALID", err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{"data": out.Pipeline})
}

// ListPipelines handles GET /api/v1/pipelines.
func (h *PipelineHandler) ListPipelines(c echo.Context) error {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "org_default"
	}

	out, err := h.listPipelines.Execute(c.Request().Context(), usecase.ListPipelinesInput{OrgID: orgID})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Pipelines})
}

// GetPipeline handles GET /api/v1/pipelines/:id.
func (h *PipelineHandler) GetPipeline(c echo.Context) error {
	id := c.Param("id")

	pipeline, err := h.pipelineRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "PIPELINE_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": pipeline})
}

// deployRequest is the request body for POST /pipelines/:id/deploy.
type deployRequest struct {
	Version    string `json:"version"`
	DeployedBy string `json:"deployed_by"`
}

// DeployPipeline handles POST /api/v1/pipelines/:id/deploy.
func (h *PipelineHandler) DeployPipeline(c echo.Context) error {
	id := c.Param("id")

	var req deployRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_DEPLOY_INVALID", err.Error())
	}

	out, err := h.deployPipeline.Execute(c.Request().Context(), usecase.DeployPipelineInput{
		PipelineID: id,
		Version:    req.Version,
		DeployedBy: req.DeployedBy,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_DEPLOY_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{"data": out.Deployment})
}

// ListDeployments handles GET /api/v1/pipelines/:id/deployments.
func (h *PipelineHandler) ListDeployments(c echo.Context) error {
	id := c.Param("id")

	deployments, err := h.deploymentRepo.ListByPipelineID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DEPLOYMENT_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": deployments})
}
