package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

const resourceStatusRunning = "running"

// PipelineHandler handles HTTP requests for pipeline operations.
type PipelineHandler struct {
	createPipeline *usecase.CreatePipeline
	listPipelines  *usecase.ListPipelines
	deployPipeline *usecase.DeployPipeline
	pipelineRepo   port.PipelineRepository
	deploymentRepo port.DeploymentRepository
	kubeconfig     port.KubeconfigProvider
	stepTracker    *kube.StepTracker
	pool           *pgxpool.Pool
}

// NewPipelineHandler constructs a PipelineHandler.
func NewPipelineHandler(
	createPipeline *usecase.CreatePipeline,
	listPipelines *usecase.ListPipelines,
	deployPipeline *usecase.DeployPipeline,
	pipelineRepo port.PipelineRepository,
	deploymentRepo port.DeploymentRepository,
	kubeconfigProvider port.KubeconfigProvider,
	stepTracker *kube.StepTracker,
	pool *pgxpool.Pool,
) *PipelineHandler {
	return &PipelineHandler{
		createPipeline: createPipeline,
		listPipelines:  listPipelines,
		deployPipeline: deployPipeline,
		pipelineRepo:   pipelineRepo,
		deploymentRepo: deploymentRepo,
		kubeconfig:     kubeconfigProvider,
		stepTracker:    stepTracker,
		pool:           pool,
	}
}

// RegisterRoutes registers pipeline routes on the given Echo group.
func (h *PipelineHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/pipelines", h.ListPipelines)
	g.POST("/pipelines", h.CreatePipeline)
	g.DELETE("/pipelines/:id", h.DeletePipeline)
	g.POST("/pipelines/:id/deploy", h.DeployPipeline)
	g.GET("/pipelines/:id/resources", h.GetPipelineResources)
	g.GET("/deployments", h.ListDeployments)
	g.GET("/deployments/:id", h.GetDeployment)
	g.GET("/app-templates", h.ListAppTemplates)
	g.POST("/deploy-app", h.DeployApp)
}

type pipelineResourceResponse struct {
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Stage         string   `json:"stage"`
	Status        string   `json:"status"`
	LabelSelector string   `json:"label_selector,omitempty"`
	ServiceURLs   []string `json:"service_urls,omitempty"`
}

// RegisterStackRoutes registers pipeline routes under the /stacks group.
// This allows GET /api/v1/stacks/:stackId/pipelines without the Stack
// module importing the CI/CD module.
func (h *PipelineHandler) RegisterStackRoutes(g *echo.Group) {
	g.GET("/:stackId/pipelines", h.ListPipelinesByStack)
}

func (h *PipelineHandler) StreamDeployLogs(c echo.Context) error {
	return StreamCicdLogs(c, h.stepTracker)
}

// createPipelineRequest is the request body for POST /pipelines.
type createPipelineRequest struct {
	Name           string            `json:"name"`
	TemplateID     string            `json:"template_id"`
	ClusterID      string            `json:"cluster_id"`
	StackID        string            `json:"stack_id,omitempty"` // optional — links to a stack
	Namespace      string            `json:"namespace"`
	AppType        string            `json:"app_type"`
	GitRepoURL     string            `json:"git_repo_url"`
	DockerfilePath string            `json:"dockerfile_path"`
	DockerContext  string            `json:"docker_context"`
	EnvVars        map[string]string `json:"env_vars"`
}

// CreatePipeline handles POST /api/v1/pipelines.
func (h *PipelineHandler) CreatePipeline(c echo.Context) error {
	var req createPipelineRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_CONFIG_INVALID", err.Error())
	}

	orgID := h.validatedOrgID(c.Request().Context(), c.Request().Header.Get("X-Org-ID"))

	out, err := h.createPipeline.Execute(c.Request().Context(), usecase.CreatePipelineInput{
		Name:           req.Name,
		TemplateID:     req.TemplateID,
		OrgID:          orgID,
		ClusterID:      req.ClusterID,
		StackID:        req.StackID,
		Namespace:      req.Namespace,
		AppType:        domain.AppType(req.AppType),
		GitRepoURL:     req.GitRepoURL,
		DockerfilePath: req.DockerfilePath,
		DockerContext:  req.DockerContext,
		EnvVars:        req.EnvVars,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrStackNotFound) {
			return errorResponse(c, http.StatusBadRequest, "STACK_NOT_FOUND", err.Error())
		}
		if errors.Is(err, usecase.ErrStackOrgMismatch) {
			return errorResponse(c, http.StatusForbidden, "STACK_ORG_MISMATCH", err.Error())
		}
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_CONFIG_INVALID", err.Error())
	}

	resp := map[string]any{"pipeline": out.Pipeline}
	if out.StackWarning != "" {
		resp["warning"] = out.StackWarning
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListPipelines handles GET /api/v1/pipelines.
// Supports optional ?stack_id= query parameter to filter by stack.
func (h *PipelineHandler) ListPipelines(c echo.Context) error {
	orgID := h.validatedOrgID(c.Request().Context(), c.Request().Header.Get("X-Org-ID"))

	out, err := h.listPipelines.Execute(c.Request().Context(), usecase.ListPipelinesInput{
		OrgID:   orgID,
		StackID: c.QueryParam("stack_id"),
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": out.Pipelines, "total": len(out.Pipelines)})
}

// ListPipelinesByStack handles GET /api/v1/stacks/:stackId/pipelines.
// Returns all pipelines linked to the given stack.
func (h *PipelineHandler) ListPipelinesByStack(c echo.Context) error {
	stackID := c.Param("stackId")
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id path parameter is required")
	}

	pipelines, err := h.pipelineRepo.ListByStackID(c.Request().Context(), stackID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": pipelines, "total": len(pipelines)})
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
	steps := []string{"Namespace 생성", "Deployment 생성", "Service 생성"}
	if pipeline, getErr := h.pipelineRepo.GetByID(c.Request().Context(), id); getErr == nil {
		steps = usecase.BuildStepPlan(pipeline)
	}
	h.stepTracker.Init(depID, steps)

	go func() {
		h.deployPipeline.ApplyAsync(depID)
		time.AfterFunc(5*time.Minute, func() { h.stepTracker.Remove(depID) })
	}()

	return c.JSON(http.StatusAccepted, map[string]any{"deploymentId": depID})
}

// DeletePipeline handles DELETE /api/v1/pipelines/:id.
func (h *PipelineHandler) DeletePipeline(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_ID_REQUIRED", "pipeline id is required")
	}

	if err := h.pipelineRepo.Delete(c.Request().Context(), id); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not found") || strings.Contains(msg, "no rows") {
			return errorResponse(c, http.StatusNotFound, "PIPELINE_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "PIPELINE_DELETE_FAILED", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
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
	ctx := c.Request().Context()
	orgID := h.validatedOrgID(ctx, c.Request().Header.Get("X-Org-ID"))

	pipelines, err := h.pipelineRepo.List(ctx, orgID)
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

func (h *PipelineHandler) GetPipelineResources(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return errorResponse(c, http.StatusBadRequest, "PIPELINE_ID_REQUIRED", "pipeline id is required")
	}

	pipeline, err := h.pipelineRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "PIPELINE_NOT_FOUND", err.Error())
	}

	if h.kubeconfig == nil {
		return errorResponse(c, http.StatusInternalServerError, "KUBECONFIG_PROVIDER_NOT_CONFIGURED", "kubeconfig provider not configured")
	}

	kubeconfig, err := h.kubeconfig.GetKubeconfig(c.Request().Context(), pipeline.ClusterID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "KUBECONFIG_LOAD_FAILED", err.Error())
	}
	if len(kubeconfig) == 0 {
		return errorResponse(c, http.StatusBadRequest, "KUBECONFIG_NOT_REGISTERED", "kubeconfig is not registered for this cluster")
	}

	items, err := collectPipelineResources(c.Request().Context(), pipeline, kubeconfig)
	if err != nil {
		return errorResponse(c, http.StatusBadGateway, "PIPELINE_RESOURCE_DISCOVERY_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func collectPipelineResources(ctx context.Context, pipeline *domain.Pipeline, kubeconfig []byte) ([]pipelineResourceResponse, error) {
	ns := strings.TrimSpace(pipeline.Namespace)
	if ns == "" {
		ns = "default"
	}

	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	restCfg.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	selectors := []string{
		fmt.Sprintf("nullus.io/pipeline-id=%s,app.kubernetes.io/managed-by=nullus-cicd", pipeline.ID),
		fmt.Sprintf("nullus.io/pipeline-id=%s", pipeline.ID),
	}

	deployments := listDeploymentsBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	statefulSets := listStatefulSetsBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	jobs := listJobsBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	cronJobs := listCronJobsBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	services := listServicesBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	ingresses := listIngressBySelectors(ctx, clientset, ns, pipeline.Name, selectors)
	workloadNames := make(map[string]struct{}, len(deployments)+len(statefulSets)+len(jobs))
	for _, dep := range deployments {
		workloadNames[dep.Name] = struct{}{}
	}
	for _, sts := range statefulSets {
		workloadNames[sts.Name] = struct{}{}
	}
	for _, job := range jobs {
		workloadNames[job.Name] = struct{}{}
	}
	pods := listPodsBySelectors(ctx, clientset, ns, pipeline.Name, pipeline.ID, selectors, workloadNames)

	ingressHostsByService := mapIngressHostsByService(ingresses)

	items := make([]pipelineResourceResponse, 0, len(deployments)+len(statefulSets)+len(jobs)+len(cronJobs)+len(services)+len(ingresses)+len(pods))

	for _, ing := range ingresses {
		items = append(items, pipelineResourceResponse{
			Kind:          "Ingress",
			Name:          ing.Name,
			Namespace:     ns,
			Stage:         "ingress",
			Status:        ingressStatus(ing),
			LabelSelector: formatLabelSelector(ing.Labels),
			ServiceURLs:   ingressURLs(ing),
		})
	}

	for _, svc := range services {
		items = append(items, pipelineResourceResponse{
			Kind:          "Service",
			Name:          svc.Name,
			Namespace:     ns,
			Stage:         "service",
			Status:        resourceStatusRunning,
			LabelSelector: formatLabelSelector(svc.Spec.Selector),
			ServiceURLs:   serviceURLs(svc, ns, ingressHostsByService[svc.Name]),
		})
	}

	for _, dep := range deployments {
		items = append(items, pipelineResourceResponse{
			Kind:          "Deployment",
			Name:          dep.Name,
			Namespace:     ns,
			Stage:         "workload",
			Status:        deploymentStatus(dep),
			LabelSelector: formatMatchLabels(dep.Spec.Selector.MatchLabels),
		})
	}

	for _, sts := range statefulSets {
		items = append(items, pipelineResourceResponse{
			Kind:          "StatefulSet",
			Name:          sts.Name,
			Namespace:     ns,
			Stage:         "workload",
			Status:        statefulSetStatus(sts),
			LabelSelector: formatMatchLabels(sts.Spec.Selector.MatchLabels),
		})
	}

	for _, job := range jobs {
		selector := ""
		if job.Spec.Selector != nil {
			selector = formatLabelSelector(job.Spec.Selector.MatchLabels)
		}
		items = append(items, pipelineResourceResponse{
			Kind:          "Job",
			Name:          job.Name,
			Namespace:     ns,
			Stage:         "job",
			Status:        jobStatus(job),
			LabelSelector: selector,
		})
	}

	for _, cron := range cronJobs {
		selector := ""
		if cron.Spec.JobTemplate.Spec.Selector != nil {
			selector = formatLabelSelector(cron.Spec.JobTemplate.Spec.Selector.MatchLabels)
		}
		items = append(items, pipelineResourceResponse{
			Kind:          "CronJob",
			Name:          cron.Name,
			Namespace:     ns,
			Stage:         "job",
			Status:        cronJobStatus(cron),
			LabelSelector: selector,
		})
	}

	for _, pod := range pods {
		items = append(items, pipelineResourceResponse{
			Kind:          "Pod",
			Name:          pod.Name,
			Namespace:     ns,
			Stage:         "pod",
			Status:        podStatus(pod),
			LabelSelector: formatLabelSelector(pod.Labels),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Stage == items[j].Stage {
			if items[i].Kind == items[j].Kind {
				return items[i].Name < items[j].Name
			}
			return items[i].Kind < items[j].Kind
		}
		return items[i].Stage < items[j].Stage
	})

	return items, nil
}

func listDeploymentsBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []appsv1.Deployment {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && dep != nil {
		return []appsv1.Deployment{*dep}
	}
	return nil
}

func listStatefulSetsBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []appsv1.StatefulSet {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && sts != nil {
		return []appsv1.StatefulSet{*sts}
	}
	return nil
}

func listJobsBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []batchv1.Job {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && job != nil {
		return []batchv1.Job{*job}
	}
	return nil
}

func listCronJobsBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []batchv1.CronJob {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if cron, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && cron != nil {
		return []batchv1.CronJob{*cron}
	}
	return nil
}

func listServicesBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []corev1.Service {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if svc, err := clientset.CoreV1().Services(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && svc != nil {
		return []corev1.Service{*svc}
	}
	return nil
}

func listIngressBySelectors(ctx context.Context, clientset *kubernetes.Clientset, namespace, appName string, selectors []string) []networkingv1.Ingress {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if ing, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, appName, metav1.GetOptions{}); err == nil && ing != nil {
		return []networkingv1.Ingress{*ing}
	}
	return nil
}

func listPodsBySelectors(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace, appName, pipelineID string,
	selectors []string,
	workloadNames map[string]struct{},
) []corev1.Pod {
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		if list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil && len(list.Items) > 0 {
			return list.Items
		}
	}
	if list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		matched := make([]corev1.Pod, 0)
		prefix := appName + "-"
		for _, pod := range list.Items {
			if pod.Labels["nullus.io/pipeline-id"] == pipelineID {
				matched = append(matched, pod)
				continue
			}
			if ownerMatchesWorkload(pod.OwnerReferences, workloadNames) {
				matched = append(matched, pod)
				continue
			}
			if strings.HasPrefix(pod.Name, prefix) && pod.Labels["app.kubernetes.io/managed-by"] == "nullus-cicd" {
				matched = append(matched, pod)
			}
		}
		if len(matched) > 0 {
			return matched
		}
	}
	return nil
}

func ownerMatchesWorkload(owners []metav1.OwnerReference, workloadNames map[string]struct{}) bool {
	if len(workloadNames) == 0 || len(owners) == 0 {
		return false
	}
	for _, owner := range owners {
		if _, ok := workloadNames[owner.Name]; ok {
			return true
		}
	}
	return false
}

func deploymentStatus(dep appsv1.Deployment) string {
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	if dep.Status.ReadyReplicas == desired && desired > 0 {
		return resourceStatusRunning
	}
	if dep.Status.UnavailableReplicas > 0 {
		return "degraded"
	}
	if dep.Status.UpdatedReplicas < desired {
		return "updating"
	}
	return "progressing"
}

func statefulSetStatus(sts appsv1.StatefulSet) string {
	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	if sts.Status.ReadyReplicas == desired && desired > 0 {
		return resourceStatusRunning
	}
	if sts.Status.CurrentReplicas == 0 {
		return "pending"
	}
	return "progressing"
}

func jobStatus(job batchv1.Job) string {
	if job.Status.Failed > 0 {
		return "failed"
	}
	if job.Status.Succeeded > 0 {
		return "completed"
	}
	if job.Status.Active > 0 {
		return resourceStatusRunning
	}
	return "pending"
}

func cronJobStatus(job batchv1.CronJob) string {
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		return "suspended"
	}
	if len(job.Status.Active) > 0 {
		return resourceStatusRunning
	}
	return "scheduled"
}

func podStatus(pod corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := strings.ToLower(strings.TrimSpace(cs.State.Waiting.Reason))
			if reason != "" {
				return reason
			}
		}
		if cs.State.Terminated != nil {
			reason := strings.ToLower(strings.TrimSpace(cs.State.Terminated.Reason))
			if reason != "" {
				return reason
			}
		}
	}
	if pod.Status.Phase == corev1.PodRunning {
		ready := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				ready = false
				break
			}
		}
		if ready {
			return resourceStatusRunning
		}
		return "not_ready"
	}
	return strings.ToLower(string(pod.Status.Phase))
}

func ingressStatus(ing networkingv1.Ingress) string {
	if len(ing.Status.LoadBalancer.Ingress) > 0 {
		return resourceStatusRunning
	}
	return "configured"
}

func formatLabelSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, labels[key]))
	}
	return strings.Join(parts, ",")
}

func formatMatchLabels(labels map[string]string) string {
	return formatLabelSelector(labels)
}

func mapIngressHostsByService(ingresses []networkingv1.Ingress) map[string][]string {
	out := map[string][]string{}
	for _, ing := range ingresses {
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			host := strings.TrimSpace(rule.Host)
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					continue
				}
				svcName := strings.TrimSpace(path.Backend.Service.Name)
				if svcName == "" || host == "" {
					continue
				}
				out[svcName] = appendUnique(out[svcName], fmt.Sprintf("http://%s", host))
			}
		}
	}
	return out
}

func ingressURLs(ing networkingv1.Ingress) []string {
	urls := make([]string, 0)
	for _, rule := range ing.Spec.Rules {
		host := strings.TrimSpace(rule.Host)
		if host == "" {
			continue
		}
		urls = appendUnique(urls, fmt.Sprintf("http://%s", host))
	}
	return urls
}

func serviceURLs(svc corev1.Service, namespace string, ingressURLs []string) []string {
	urls := make([]string, 0)
	urls = append(urls, ingressURLs...)
	for _, port := range svc.Spec.Ports {
		if port.Port > 0 {
			urls = appendUnique(urls, fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, namespace, port.Port))
			if svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != "None" {
				urls = appendUnique(urls, fmt.Sprintf("http://%s:%d", svc.Spec.ClusterIP, port.Port))
			}
		}
		if svc.Spec.Type == corev1.ServiceTypeNodePort && port.NodePort > 0 {
			urls = appendUnique(urls, fmt.Sprintf("http://<node-ip>:%d", port.NodePort))
		}
	}
	return urls
}

func appendUnique(items []string, v string) []string {
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
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
	orgID, err := h.resolveOrgID(ctx, c.Request().Header.Get("X-Org-ID"), req.ClusterID)
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "DEPLOY_APP_INVALID", "cannot resolve organization: "+err.Error())
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

const defaultOrgID = "11111111-1111-1111-1111-111111111111"

// validatedOrgID checks if the header org ID exists in DB, falls back to default.
func (h *PipelineHandler) validatedOrgID(ctx context.Context, headerOrgID string) string {
	if h.pool == nil {
		if headerOrgID != "" {
			return headerOrgID
		}
		return defaultOrgID
	}
	if headerOrgID != "" {
		var exists bool
		if err := h.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM organizations WHERE id = $1)", headerOrgID).Scan(&exists); err == nil && exists {
			return headerOrgID
		}
	}
	return defaultOrgID
}

// resolveOrgID determines the org ID: validates header, falls back to cluster's org, then default.
func (h *PipelineHandler) resolveOrgID(ctx context.Context, headerOrgID, clusterID string) (string, error) {
	if h.pool == nil {
		if headerOrgID != "" {
			return headerOrgID, nil
		}
		return defaultOrgID, nil
	}
	if headerOrgID != "" {
		var exists bool
		if err := h.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM organizations WHERE id = $1)", headerOrgID).Scan(&exists); err == nil && exists {
			return headerOrgID, nil
		}
	}
	var orgID string
	err := h.pool.QueryRow(ctx, "SELECT org_id FROM clusters WHERE id = $1", clusterID).Scan(&orgID)
	if err != nil {
		return "", fmt.Errorf("cluster %s not found: %w", clusterID, err)
	}
	return orgID, nil
}
