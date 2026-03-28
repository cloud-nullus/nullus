package handler

import (
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
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
	stepTracker    *kube.StepTracker
}

// NewPipelineHandler constructs a PipelineHandler.
func NewPipelineHandler(
	createPipeline *usecase.CreatePipeline,
	listPipelines *usecase.ListPipelines,
	deployPipeline *usecase.DeployPipeline,
	pipelineRepo port.PipelineRepository,
	deploymentRepo port.DeploymentRepository,
	stepTracker *kube.StepTracker,
) *PipelineHandler {
	return &PipelineHandler{
		createPipeline: createPipeline,
		listPipelines:  listPipelines,
		deployPipeline: deployPipeline,
		pipelineRepo:   pipelineRepo,
		deploymentRepo: deploymentRepo,
		stepTracker:    stepTracker,
	}
}

// RegisterRoutes registers pipeline routes on the given Echo group.
func (h *PipelineHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/pipelines", h.ListPipelines)
	g.POST("/pipelines", h.CreatePipeline)
	g.POST("/pipelines/:id/deploy", h.DeployPipeline)
	g.GET("/deployments", h.ListDeployments)
	g.GET("/deployments/:id", h.GetDeployment)
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

	out, err := h.deployPipeline.Start(c.Request().Context(), usecase.DeployPipelineInput{
		PipelineID: id,
		Version:    req.Version,
		DeployedBy: req.DeployedBy,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_DEPLOY_FAILED", err.Error())
	}

	depID := out.Deployment.ID
	h.stepTracker.Init(depID, []string{"Namespace 생성", "Deployment 생성", "Service 생성"})

	go func() {
		h.deployPipeline.ApplyAsync(depID)
		time.AfterFunc(30*time.Second, func() { h.stepTracker.Remove(depID) })
	}()

	return c.JSON(http.StatusAccepted, map[string]any{"deploymentId": depID})
}

// GetDeployment handles GET /api/v1/deployments/:id.
func (h *PipelineHandler) GetDeployment(c echo.Context) error {
	id := c.Param("id")
	deployment, err := h.deploymentRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "DEPLOYMENT_NOT_FOUND", err.Error())
	}
	if steps := h.stepTracker.Get(id); steps != nil {
		deployment.Steps = steps
	}
	return c.JSON(http.StatusOK, deployment)
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

	deployments := make([]*domain.Deployment, 0)
	for _, pipeline := range pipelines {
		items, err := h.deploymentRepo.ListByPipelineID(c.Request().Context(), pipeline.ID)
		if err != nil {
			return errorResponse(c, http.StatusInternalServerError, "DEPLOYMENT_LIST_FAILED", err.Error())
		}
		deployments = append(deployments, items...)
	}

	return c.JSON(http.StatusOK, map[string]any{"items": deployments, "total": len(deployments)})
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

	return c.JSON(http.StatusOK, map[string]any{
		"deploymentId": "dep_app_" + req.AppName,
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
