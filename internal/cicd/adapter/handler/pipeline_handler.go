package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
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
	g.GET("/pipelines", h.ListPipelines)
	g.POST("/pipelines", h.CreatePipeline)
	g.POST("/pipelines/:id/deploy", h.DeployPipeline)
	g.GET("/deployments", h.ListDeployments)
	g.GET("/app-templates", h.ListAppTemplates)
	g.POST("/deploy-app", h.DeployApp)
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
		orgID = "11111111-1111-1111-1111-111111111111"
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

	return c.JSON(http.StatusCreated, out.Pipeline)
}

// ListPipelines handles GET /api/v1/pipelines.
func (h *PipelineHandler) ListPipelines(c echo.Context) error {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "11111111-1111-1111-1111-111111111111"
	}

	out, err := h.listPipelines.Execute(c.Request().Context(), usecase.ListPipelinesInput{OrgID: orgID})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": out.Pipelines, "total": len(out.Pipelines)})
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

	return c.JSON(http.StatusOK, map[string]any{"deploymentId": out.Deployment.ID})
}

type deploymentResponse struct {
	ID           string  `json:"id"`
	PipelineID   string  `json:"pipelineId"`
	PipelineName string  `json:"pipelineName"`
	Version      string  `json:"version"`
	Status       string  `json:"status"`
	TriggeredBy  string  `json:"triggeredBy"`
	StartedAt    string  `json:"startedAt"`
	CompletedAt  *string `json:"completedAt"`
}

func (h *PipelineHandler) ListDeployments(c echo.Context) error {
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "11111111-1111-1111-1111-111111111111"
	}

	pipelines, err := h.pipelineRepo.List(c.Request().Context(), orgID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_LIST_FAILED", err.Error())
	}

	pipelineNames := make(map[string]string, len(pipelines))
	for _, p := range pipelines {
		pipelineNames[p.ID] = p.Name
	}

	results := make([]deploymentResponse, 0)
	for _, pipeline := range pipelines {
		items, err := h.deploymentRepo.ListByPipelineID(c.Request().Context(), pipeline.ID)
		if err != nil {
			return errorResponse(c, http.StatusInternalServerError, "DEPLOYMENT_LIST_FAILED", err.Error())
		}
		for _, d := range items {
			resp := deploymentResponse{
				ID:           d.ID,
				PipelineID:   d.PipelineID,
				PipelineName: pipelineNames[d.PipelineID],
				Version:      d.Version,
				Status:       string(d.Status),
				TriggeredBy:  d.DeployedBy,
				StartedAt:    d.StartedAt.Format(time.RFC3339),
			}
			if d.CompletedAt != nil {
				formatted := d.CompletedAt.Format(time.RFC3339)
				resp.CompletedAt = &formatted
			}
			results = append(results, resp)
		}
	}

	return c.JSON(http.StatusOK, map[string]any{"items": results, "total": len(results)})
}

func (h *PipelineHandler) ListAppTemplates(c echo.Context) error {
	appTemplates := []map[string]any{
		{"id": "go-web-api", "name": "Go Web API", "runtime": "go1.24", "port": 8080},
		{"id": "react-vite", "name": "React Vite App", "runtime": "node22", "port": 5173},
		{"id": "spring-boot", "name": "Spring Boot Service", "runtime": "java21", "port": 8080},
	}
	return c.JSON(http.StatusOK, appTemplates)
}

type deployAppRequest struct {
	TemplateID string `json:"templateId"`
	AppName    string `json:"appName"`
	ClusterID  string `json:"clusterId"`
	Namespace  string `json:"namespace"`
	GitURL     string `json:"gitUrl"`
	Replicas   int32  `json:"replicas"`
	Port       int32  `json:"port"`
	Resources  struct {
		CPULimit   string `json:"cpuLimit"`
		MemLimit   string `json:"memLimit"`
		CPURequest string `json:"cpuRequest"`
		MemRequest string `json:"memRequest"`
	} `json:"resources"`
	EnvVars map[string]string `json:"envVars"`
}

func (h *PipelineHandler) DeployApp(c echo.Context) error {
	var req deployAppRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_APP_INVALID", err.Error())
	}

	generated, err := manifests.Generate(manifests.DeployAppRequest{
		AppName:   req.AppName,
		GitURL:    req.GitURL,
		Namespace: req.Namespace,
		Template:  req.TemplateID,
		Replicas:  req.Replicas,
		Port:      req.Port,
		Resources: manifests.ResourceSpec{
			CPULimit:   req.Resources.CPULimit,
			MemLimit:   req.Resources.MemLimit,
			CPURequest: req.Resources.CPURequest,
			MemRequest: req.Resources.MemRequest,
		},
		EnvVars: req.EnvVars,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_APP_INVALID", err.Error())
	}

	ctx := c.Request().Context()
	orgID := c.Request().Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "11111111-1111-1111-1111-111111111111"
	}

	// Pipeline 저장 (이미 존재하면 무시)
	pipelineID := "pip_app_" + req.AppName + "_" + req.Namespace
	pipeline := &domain.Pipeline{
		ID:         pipelineID,
		Name:       req.AppName,
		TemplateID: req.TemplateID,
		OrgID:      orgID,
		ClusterID:  req.ClusterID,
		Namespace:  req.Namespace,
		AppType:    domain.AppTypeBackend,
		GitRepoURL: req.GitURL,
		Status:     domain.PipelineStatusActive,
		CreatedAt:  time.Now(),
	}
	if err := h.pipelineRepo.Create(ctx, pipeline); err != nil {
		slog.Warn("pipeline create failed (may already exist)", "id", pipelineID, "error", err)
	}

	// Deployment 기록 저장
	now := time.Now()
	completed := now.Add(3 * time.Second)
	deploymentID := "dep_app_" + req.AppName + "_" + now.Format("20060102150405")
	deployment := &domain.Deployment{
		ID:          deploymentID,
		PipelineID:  pipelineID,
		Version:     "latest",
		Status:      domain.DeploymentStatusSuccess,
		StartedAt:   now,
		CompletedAt: &completed,
		DeployedBy:  orgID,
	}
	if err := h.deploymentRepo.Create(ctx, deployment); err != nil {
		slog.Error("deployment create failed", "id", deploymentID, "error", err)
		return errorResponse(c, http.StatusInternalServerError, "DEPLOY_SAVE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"deploymentId": deploymentID,
		"templateId":   req.TemplateID,
		"namespace":    req.Namespace,
		"clusterId":    req.ClusterID,
		"manifests": map[string]string{
			"namespace":  generated.Namespace,
			"deployment": generated.Deployment,
			"service":    generated.Service,
			"ingress":    generated.Ingress,
		},
	})
}
