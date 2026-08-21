import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../../lib/api";
import { useAuthStore } from "../../../stores/auth-store";
import type {
  AppTemplateInfo,
  CicdTemplate,
  CreateCicdTemplateRequest,
  CreatePipelineRequest,
  DeployAppRequest,
  DeployAppResult,
  Deployment,
  Pipeline,
  PipelineResource,
} from "../../../types";

export type {
  AppTemplate,
  AppTemplateInfo,
  AppType,
  CicdTemplate,
  CreateCicdTemplateRequest,
  CreatePipelineRequest,
  DeployAppRequest,
  DeployAppResult,
  Deployment,
  Pipeline,
  PipelineResource,
  PipelineStatus,
} from "../../../types";

// --- Types ---

/**
 * 파이프라인 삭제 시 함께 지울 대상. 셋 다 선택이며 기본은 지우지 않는다.
 *
 * 소스 저장소와 이미지는 되돌릴 수 없고, 클러스터 리소스를 지우면 돌던 앱이
 * 멈춘다. 그래서 파이프라인 삭제의 기본 동작에 포함하지 않는다.
 */
export interface DeletePipelineOptions {
  id: string;
  deleteClusterResources?: boolean;
  deleteRepository?: boolean;
  deleteImages?: boolean;
}

/** 실제로 무엇이 지워졌는지. 요청과 다를 수 있다(예: 미지원 레지스트리). */
export interface DeletePipelineResult {
  cluster_resources_deleted: boolean;
  repository_deleted: boolean;
  images_deleted: boolean;
  warnings?: string[] | null;
}

/**
 * 파이프라인 생성 시 플랫폼이 준비한 것과, 준비하지 못한 것.
 *
 * 서버는 저장소·CI job·Argo CD Application 을 준비하며 문제를 warnings 로
 * 보고한다. 예전에는 이 값을 화면이 통째로 버려서, 저장소가 비어 있는데도
 * 파이프라인은 "정상" 으로 보였다.
 */
export interface PipelineProvisioning {
  warnings: string[];
  /** 이미 있던 저장소라 스캐폴딩을 쓰지 않았다 — 리포에 Jenkinsfile 이 없을 수 있다. */
  scaffoldSkipped: boolean;
  /** CI 가 필요로 하는데 등록되지 못한 변수들. */
  missingVariables: string[];
  repositoryPath?: string;
  argoApplicationCreated?: boolean;
}

export interface CICDTool {
  category: string;
  name: string;
  helm_version: string;
  app_version: string;
}

export interface CICDGoldenPath {
  id: string;
  name: string;
  description: string;
  tools: CICDTool[];
  estimated_install_time: number;
  recommended_use_case: string;
  min_resources: string;
  created_at?: string;
}

// --- Query keys ---

const queryKeys = {
  templates: () => ["cicd", "templates"] as const,
  goldenPaths: () => ["cicd", "golden-paths"] as const,
  pipelines: (filters?: Record<string, unknown>) =>
    ["cicd", "pipelines", filters] as const,
  deployments: (filters?: Record<string, unknown>) =>
    ["cicd", "deployments", filters] as const,
  appTemplates: () => ["cicd", "appTemplates"] as const,
  pipelineResources: (pipelineId: string) =>
    ["cicd", "pipelineResources", pipelineId] as const,
};

// --- API functions ---

// 테스트에서 매퍼를 직접 검증할 수 있도록 내보낸다. 훅을 거치면 react-query 목까지
// 끼워야 해서 응답 표기 대응 같은 순수 변환 로직을 확인하기 어렵다.
export const cicdApiCalls = {
  resolvePipelineMode: (stages: string[] | undefined): Pipeline["mode"] => {
    const normalized = (stages ?? []).map((stage) =>
      String(stage).toLowerCase(),
    );
    const hasCI = normalized.some(
      (stage) =>
        stage.includes("build") ||
        stage.includes("test") ||
        stage.includes("lint") ||
        stage.includes("scan") ||
        stage.includes("package"),
    );
    const hasCD = normalized.some(
      (stage) =>
        stage.includes("deploy") ||
        stage.includes("release") ||
        stage.includes("rollout") ||
        stage.includes("sync") ||
        stage.includes("apply"),
    );
    if (hasCI && hasCD) return "ci_cd";
    if (hasCI) return "ci";
    if (hasCD) return "cd";
    return "ci_cd";
  },

  getTemplates: async () => {
    const raw = await api.get<any[]>("/cicd/templates").then((r) => r.data);

    return (raw ?? []).map((t: any) => ({
      id: t.id,
      name: t.name,
      description: t.description ?? "",
      appType: (t.app_type ?? "") as CicdTemplate["appType"],
      stages: t.stages ?? [],
      createdBy: t.created_by,
      gitRepoUrl: t.git_repo_url ?? "",
      dockerfilePath: t.dockerfile_path ?? "",
      dockerContext: t.docker_context ?? "",
      envVars: t.env_vars ?? {},
    })) as CicdTemplate[];
  },

  createTemplate: (data: CreateCicdTemplateRequest) =>
    api.post<CicdTemplate>("/cicd/templates", data).then((r) => r.data),

  updateTemplate: (data: CreateCicdTemplateRequest) =>
    api
      .put<CicdTemplate>(`/cicd/templates/${data.id}`, data)
      .then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/cicd/templates/${id}`).then((r) => r.data),

  getGoldenPaths: () =>
    api.get<CICDGoldenPath[]>("/cicd/golden-paths").then((r) => r.data),

  getGoldenPathById: (id: string) =>
    api.get<CICDGoldenPath>(`/cicd/golden-paths/${id}`).then((r) => r.data),

  createGoldenPath: (data: CICDGoldenPath) =>
    api.post<CICDGoldenPath>("/cicd/golden-paths", data).then((r) => r.data),

  updateGoldenPath: (id: string, data: CICDGoldenPath) =>
    api
      .put<CICDGoldenPath>(`/cicd/golden-paths/${id}`, data)
      .then((r) => r.data),

  deleteGoldenPath: (id: string) =>
    api.delete<void>(`/cicd/golden-paths/${id}`).then((r) => r.data),

  // 부수 리소스 삭제는 각각 명시적으로 켠다. 기본값(옵션 없음)은 파이프라인
  // 레코드만 지우는 종전 동작이다 — 되돌릴 수 없는 일은 요청에 드러나야 한다.
  deletePipeline: (input: string | DeletePipelineOptions) => {
    const { id, ...options } =
      typeof input === "string" ? { id: input } : input;
    const params: Record<string, string> = {};
    if (options.deleteClusterResources) params.cluster_resources = "true";
    if (options.deleteRepository) params.repository = "true";
    if (options.deleteImages) params.images = "true";
    return api
      .delete<DeletePipelineResult | void>(`/cicd/pipelines/${id}`, { params })
      .then((r) => r.data);
  },

  getPipelines: async (filters?: { status?: string; search?: string }) => {
    const raw = await api
      .get<any>("/cicd/pipelines", { params: filters })
      .then((r) => r.data);
    const clustersRes = await api
      .get<any>("/admin/clusters")
      .then((r) => r.data);
    const templatesRes = await api
      .get<any[]>("/cicd/templates")
      .then((r) => r.data);
    const clusterMap = new Map(
      (clustersRes.items ?? []).map((c: any) => [c.id, c.name]),
    );
    const templateStagesMap = new Map(
      (templatesRes ?? []).map((tpl: any) => [tpl.id, tpl.stages ?? []]),
    );

    const deploymentsRes = await api
      .get<any>("/cicd/deployments")
      .then((r) => r.data);
    const latestDeployByPipeline = new Map<string, string>();

    // getDeployments 와 같은 표기 문제. 여기서도 snake_case 만 읽어서
    // CI/CD 목록의 "마지막 배포" 가 항상 비어 있었다.
    for (const d of deploymentsRes.items ?? []) {
      const pid = d.pipelineId ?? d.pipeline_id;
      const startedAt = d.startedAt ?? d.started_at;
      if (!pid || !startedAt) continue;
      const existing = latestDeployByPipeline.get(pid);
      if (!existing || startedAt > existing) {
        latestDeployByPipeline.set(pid, startedAt);
      }
    }

    const items: Pipeline[] = (raw.items ?? []).map((p: any) => ({
      id: p.id,
      name: p.name,
      // 배포된 앱이 열리는 주소. 스택에 접근 도메인이 없으면 비어 있다 —
      // 서버가 계산해 내려준다(도메인이 바뀌면 저장된 값은 그 순간 거짓이 된다).
      accessUrl: p.access_url ?? "",
      mode: cicdApiCalls.resolvePipelineMode(
        templateStagesMap.get(p.template_id),
      ),
      appType: (p.app_type ?? "") as Pipeline["appType"],
      templateId: p.template_id ?? "",
      stackId: p.stack_id ?? "",
      executionMode: p.execution_mode ?? "",
      gitRepoUrl: p.git_repo_url ?? "",
      clusterId: p.cluster_id ?? "",
      clusterName: clusterMap.get(p.cluster_id) ?? "",
      namespace: p.namespace ?? "default",
      dockerfilePath: p.dockerfile_path ?? "",
      dockerContext: p.docker_context ?? "",
      envVars: p.env_vars ?? {},
      status: (p.status ?? "pending") as Pipeline["status"],
      lastDeployedAt: latestDeployByPipeline.get(p.id) ?? null,
      createdAt: p.created_at ?? "",
    }));

    return { items, total: raw.total ?? items.length };
  },

  createPipeline: async (data: CreatePipelineRequest) => {
    const res: any = await api
      .post("/cicd/pipelines", {
        name: data.name,
        app_type: data.appType,
        cluster_id: data.clusterId,
        namespace: data.namespace ?? "default",
        template_id: data.templateId ?? "",
        stack_id: data.stackId ?? "",
        git_repo_url: data.gitRepoUrl ?? "",
        dockerfile_path: data.dockerfilePath ?? "",
        docker_context: data.dockerContext ?? "",
        execution_mode: data.executionMode ?? "ci_cd",
        env_vars: data.envVars ?? {},
      })
      .then((r) => r.data);

    const raw = res.pipeline ?? res;
    const provisioning: PipelineProvisioning = {
      warnings: res.warnings ?? [],
      scaffoldSkipped: res.scaffold_skipped ?? false,
      missingVariables: res.missing_variables ?? [],
      repositoryPath: res.repository_path ?? undefined,
      argoApplicationCreated: res.argo_application_created ?? undefined,
    };

    return {
      provisioning,
      id: raw.id,
      name: raw.name,
      mode: "ci_cd",
      appType: (raw.app_type ?? "") as Pipeline["appType"],
      templateId: raw.template_id ?? "",
      stackId: raw.stack_id ?? "",
      executionMode: raw.execution_mode ?? "ci_cd",
      gitRepoUrl: raw.git_repo_url ?? "",
      clusterId: raw.cluster_id ?? "",
      clusterName: "",
      namespace: raw.namespace ?? "default",
      dockerfilePath: raw.dockerfile_path ?? "",
      dockerContext: raw.docker_context ?? "",
      envVars: raw.env_vars ?? {},
      status: (raw.status ?? "active") as Pipeline["status"],
      lastDeployedAt: null,
      createdAt: raw.created_at ?? "",
    } as Pipeline & { provisioning: PipelineProvisioning };
  },

  deployPipeline: async ({
    pipelineId,
    manifestTypes,
  }: {
    pipelineId: string;
    manifestTypes?: string[];
  }) => {
    const user = useAuthStore.getState().user;
    const response = await api.post<{ deploymentId: string }>(
      `/cicd/pipelines/${pipelineId}/deploy`,
      {
        version: `v0.1.${Date.now() % 1000}`,
        deployed_by: user?.email ?? "",
        manifest_types: manifestTypes,
      },
    );
    return response.data;
  },

  getDeployment: async (deploymentId: string) => {
    const raw: any = await api
      .get(`/cicd/deployments/${deploymentId}`)
      .then((r) => r.data);
    const rawSteps = (raw.steps ?? raw.Steps ?? []) as any[];
    const normalizedSteps = rawSteps.map((step, index) => {
      const rawLogs = step.logs ?? step.Logs ?? [];
      const normalizedLogs = Array.isArray(rawLogs)
        ? rawLogs.map((line: unknown) => String(line))
        : [];
      const message = step.message ?? step.Message;
      const name = step.name ?? step.Name ?? `Step ${index + 1}`;

      // Some environments only return step-level message without explicit logs.
      const logs =
        normalizedLogs.length > 0
          ? normalizedLogs
          : typeof message === "string" && message
            ? [message]
            : [];

      return {
        name: String(name),
        status: String(step.status ?? step.Status ?? ""),
        kind: String(step.kind ?? step.Kind ?? ""),
        message: typeof message === "string" ? message : undefined,
        applied_at: String(step.applied_at ?? step.AppliedAt ?? ""),
        logs,
      };
    });

    const topLevelLogs = raw.logs ?? raw.Logs;
    const fallbackStep =
      normalizedSteps.length === 0 &&
      Array.isArray(topLevelLogs) &&
      topLevelLogs.length > 0
        ? [
            {
              name: "Output",
              status: "completed",
              kind: "log",
              message: "",
              applied_at: "",
              logs: topLevelLogs.map((line: unknown) => String(line)),
            },
          ]
        : [];

    return {
      id: raw.ID ?? raw.id ?? "",
      status: (raw.Status ?? raw.status ?? "running") as string,
      steps: [...normalizedSteps, ...fallbackStep] as Array<{
        name: string;
        status: string;
        kind: string;
        message?: string;
        applied_at?: string;
        logs?: string[];
      }>,
      startedAt: raw.StartedAt ?? raw.started_at ?? "",
      completedAt: raw.CompletedAt ?? raw.completed_at ?? null,
    };
  },

  getDeployments: async (filters?: {
    pipelineId?: string;
    status?: string;
  }) => {
    const [raw, pipelinesRaw] = await Promise.all([
      api
        .get<any>("/cicd/deployments", { params: filters })
        .then((r) => r.data),
      api.get<any>("/cicd/pipelines").then((r) => r.data),
    ]);

    const pipelineNameMap = new Map<string, string>();
    for (const p of pipelinesRaw.items ?? []) {
      pipelineNameMap.set(p.id, p.name);
    }

    // GET /cicd/deployments 는 camelCase 로 응답한다(pipelineId, startedAt …).
    // 이 매퍼는 snake_case 만 읽고 있어서 CI/CD 이력 화면의 6개 컬럼 중 4개가
    // 항상 비어 보였다 — 파이프라인·배포자·시작·완료 시각. 실데이터로 앱을
    // 돌려 보고 발견했다. 두 표기를 모두 받아 응답 표기가 갈려도 견디게 한다.
    const mappedItems: Deployment[] = (raw.items ?? []).map((d: any) => {
      const pipelineId = d.pipelineId ?? d.pipeline_id ?? "";
      return {
        id: d.id,
        pipelineId,
        // 목록 응답이 pipelineName 을 이미 주면 그걸 쓰고, 없으면 파이프라인 목록으로 채운다.
        pipelineName: d.pipelineName ?? pipelineNameMap.get(pipelineId) ?? "",
        version: d.version ?? "",
        status: (d.status ?? "pending") as Deployment["status"],
        triggeredBy: d.triggeredBy ?? d.deployed_by ?? "",
        startedAt: d.startedAt ?? d.started_at ?? "",
        completedAt: d.completedAt ?? d.completed_at ?? null,
      };
    });

    const items = mappedItems
      .filter(
        (item) =>
          !filters?.pipelineId || item.pipelineId === filters.pipelineId,
      )
      .filter((item) => !filters?.status || item.status === filters.status)
      .sort((a, b) => {
        const aTime = a.startedAt ? new Date(a.startedAt).getTime() : 0;
        const bTime = b.startedAt ? new Date(b.startedAt).getTime() : 0;
        return bTime - aTime;
      });

    return { items, total: items.length };
  },

  getAppTemplates: () =>
    api.get<AppTemplateInfo[]>("/cicd/app-templates").then((r) => r.data),

  deployApp: (request: DeployAppRequest) =>
    api.post<DeployAppResult>("/cicd/deploy-app", request).then((r) => r.data),

  getPipelineResources: async (pipelineId: string) => {
    const raw = await api
      .get<any>(`/cicd/pipelines/${pipelineId}/resources`)
      .then((r) => r.data);
    const items = (raw.items ?? []).map((item: any) => ({
      kind: String(item.kind ?? ""),
      name: String(item.name ?? ""),
      namespace: String(item.namespace ?? ""),
      stage: String(item.stage ?? ""),
      status: String(item.status ?? ""),
      labelSelector: item.label_selector
        ? String(item.label_selector)
        : undefined,
      serviceUrls: Array.isArray(item.service_urls)
        ? item.service_urls.map((url: unknown) => String(url))
        : [],
    })) as PipelineResource[];
    return { items, total: raw.total ?? items.length };
  },
};

// --- Hooks ---

export function useCicdTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: cicdApiCalls.getTemplates,
  });
}

export function useCreateCicdTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.createTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "templates"] });
    },
  });
}

export function useUpdateCicdTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.updateTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "templates"] });
    },
  });
}

export function useDeleteCicdTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.deleteTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "templates"] });
    },
  });
}

export function usePipelines(filters?: { status?: string; search?: string }) {
  return useQuery({
    queryKey: queryKeys.pipelines(filters),
    queryFn: () => cicdApiCalls.getPipelines(filters),
  });
}

export function useCreatePipeline() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.createPipeline,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "pipelines"] });
    },
  });
}

export function useDeletePipeline() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.deletePipeline,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "pipelines"] });
      void qc.invalidateQueries({ queryKey: ["cicd", "deployments"] });
    },
  });
}

export function useDeployPipeline() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.deployPipeline,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "pipelines"] });
      void qc.invalidateQueries({ queryKey: ["cicd", "deployments"] });
    },
  });
}

export function useDeployments(filters?: {
  pipelineId?: string;
  status?: string;
}) {
  return useQuery({
    queryKey: queryKeys.deployments(filters),
    queryFn: () => cicdApiCalls.getDeployments(filters),
  });
}

export function useDeploymentStatus(deploymentId: string | null) {
  return useQuery({
    queryKey: ["cicd-deployment-status", deploymentId],
    queryFn: () => cicdApiCalls.getDeployment(deploymentId!),
    enabled: !!deploymentId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status === "success" || status === "failed") return false;
      return 2000;
    },
    staleTime: 0,
  });
}

export function useTemplateById(templateId: string) {
  return useQuery({
    queryKey: ["cicd-template", templateId],
    queryFn: () =>
      api.get<any>(`/cicd/templates/${templateId}`).then((r) => {
        const t = r.data;
        return {
          id: t.id,
          name: t.name,
          stages: t.stages ?? [],
          appType: t.app_type ?? "",
        };
      }),
    enabled: !!templateId,
  });
}

export function usePipelineDeployments(pipelineId: string) {
  return useQuery({
    queryKey: ["cicd-pipeline-deployments", pipelineId],
    queryFn: () => cicdApiCalls.getDeployments({ pipelineId }),
    enabled: !!pipelineId,
  });
}

export function useAppTemplates() {
  return useQuery({
    queryKey: queryKeys.appTemplates(),
    queryFn: cicdApiCalls.getAppTemplates,
  });
}

export function useDeployApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.deployApp,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "pipelines"] });
      void qc.invalidateQueries({ queryKey: ["cicd", "deployments"] });
    },
  });
}

export function usePipelineResources(pipelineId: string) {
  return useQuery({
    queryKey: queryKeys.pipelineResources(pipelineId),
    queryFn: () => cicdApiCalls.getPipelineResources(pipelineId),
    enabled: !!pipelineId,
    staleTime: 3000,
  });
}

export function useRollbackDeployment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      pipelineId,
      deploymentId,
      preservePVC,
    }: {
      pipelineId: string;
      deploymentId: string;
      preservePVC: boolean;
    }) =>
      api
        .post(`/api/v1/cicd/pipelines/${pipelineId}/rollback/${deploymentId}`, {
          preservePVC,
        })
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["cicd", "deployments"] });
      void qc.invalidateQueries({ queryKey: ["deployments"] });
    },
  });
}

// --- Golden Path Hooks ---

export function useGoldenPaths() {
  return useQuery({
    queryKey: queryKeys.goldenPaths(),
    queryFn: cicdApiCalls.getGoldenPaths,
  });
}

export function useGoldenPathById(id: string) {
  return useQuery({
    queryKey: ["cicd", "golden-paths", id] as const,
    queryFn: () => cicdApiCalls.getGoldenPathById(id),
    enabled: !!id,
  });
}

export function useCreateGoldenPath() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.createGoldenPath,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.goldenPaths() });
    },
  });
}

export function useUpdateGoldenPath() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: CICDGoldenPath }) =>
      cicdApiCalls.updateGoldenPath(id, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.goldenPaths() });
    },
  });
}

export function useDeleteGoldenPath() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cicdApiCalls.deleteGoldenPath,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.goldenPaths() });
    },
  });
}
