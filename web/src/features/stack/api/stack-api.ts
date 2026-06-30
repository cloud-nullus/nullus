import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../../lib/api";
import {
  normalizeTemplate,
  normalizeCompatibilityMatrix,
  normalizeStackItem,
  normalizeStackHistoryEntry,
  normalizeCompatibilityValidationResult,
  matrixInputToPayload,
  toCreateStackBody,
} from "./stack-normalizers";
import type {
  RawTemplate,
  RawCompatibilityMatrix,
  RawStackItem,
  RawStackHistoryEntry,
  RawCompatibilityValidationResult,
} from "./stack-normalizers";
import type {
  TemplateMutationRequest,
  ValidateCompatibilityInput,
  MatrixInput,
  DeployStackInput,
  StackIntegrationsResponse,
  StackMonitoringSnapshot,
} from "./stack-api-types";
import type {
  ClusterStatus,
  StackResourceDefault,
  StackWorkloads,
} from "../../../types";
import {
  parseContentDispositionFilename,
  type StackExportFormat,
} from "../utils/export-utils";

export * from "./stack-api-types";
export { toCreateStackBody } from "./stack-normalizers";

interface RawClusterSummary {
  id: string;
  name: string;
  connection_status?: ClusterStatus;
  status?: ClusterStatus;
}

interface RawClusterVerifyResult {
  status?: string;
  version?: string;
}

interface StorageTestRequest {
  target: "database" | "object_storage";
  endpoint: string;
  provider_or_engine?: string;
  auth_id?: string;
  auth_password?: string;
  resource_name?: string;
}

interface StorageTestResult {
  ok: boolean;
  message: string;
}

const queryKeys = {
  templates: () => ["stacks", "templates"] as const,
  template: (id: string) => ["stacks", "templates", id] as const,
  list: (filters?: Record<string, unknown>) =>
    ["stacks", "list", filters] as const,
  history: (stackId: string) => ["stacks", "history", stackId] as const,
  monitoring: (stackId: string) => ["stacks", "monitoring", stackId] as const,
  versionDiff: (stackId: string, from: number, to: number) =>
    ["stacks", "diff", stackId, from, to] as const,
  compatibilityMatrix: () => ["stacks", "compatibility"] as const,
  clusters: () => ["clusters"] as const,
  workloads: (stackId: string) => ["stacks", "workloads", stackId] as const,
  integrations: (stackId: string) =>
    ["stacks", "integrations", stackId] as const,
  resourceDefaults: () => ["stacks", "resource-defaults"] as const,
};

const ACTIVE_DEPLOYMENT_STATES = new Set([
  "pending",
  "validating",
  "installing",
  "configuring",
  "health_check",
  "rolling_back",
]);

export function stackListRefetchInterval(
  data: { items?: Array<{ status?: string }> } | undefined,
  forcedIntervalMs = 0,
): number | false {
  if (forcedIntervalMs > 0) {
    return forcedIntervalMs;
  }

  return data?.items?.some((stack) =>
    ACTIVE_DEPLOYMENT_STATES.has(stack.status ?? ""),
  )
    ? 3000
    : false;
}

const stackApiCalls = {
  getTemplates: () =>
    api
      .get<RawTemplate[]>("/stacks/templates")
      .then((r) => (r.data ?? []).map(normalizeTemplate)),

  getTemplate: (id: string) =>
    api
      .get<import("../../../types").StackTemplate>(`/stacks/templates/${id}`)
      .then((r) => r.data),

  getList: (filters?: {
    status?: string;
    search?: string;
    include_deleted?: boolean;
  }) =>
    api
      .get<{
        items: import("../../../types").Stack[];
        total: number;
      }>("/stacks", { params: filters })
      .then((r) => ({
        ...r.data,
        items: ((r.data.items ?? []) as unknown as RawStackItem[]).map(
          normalizeStackItem,
        ),
      })),

  create: (request: import("../../../types").CreateStackRequest) =>
    api
      .post<{ id: string }>("/stacks", toCreateStackBody(request))
      .then((r) => r.data),

  delete: (stackId: string) =>
    api.delete("/stacks/" + stackId).then((r) => r.data),

  saveDraft: (request: import("../../../types").CreateStackRequest) =>
    api.post<{ draftId: string }>("/stacks/draft", request).then((r) => r.data),

  estimateResources: (
    input: import("../../../types").CreateStackRequest["resources"],
  ) =>
    api
      .post<
        import("../../../types").ResourceEstimate
      >("/stacks/estimate", input)
      .then((r) => r.data),

  getResourceDefaults: () =>
    api
      .get<{
        items: StackResourceDefault[];
        total: number;
      }>("/stacks/resource-defaults")
      .then((r) => r.data),

  upsertResourceDefault: (payload: Omit<StackResourceDefault, "updated_at">) =>
    api
      .post<StackResourceDefault>("/stacks/resource-defaults", payload)
      .then((r) => r.data),

  getHistory: (stackId: string) =>
    api
      .get<RawStackHistoryEntry[]>(`/stacks/${stackId}/history`)
      .then((r) => (r.data ?? []).map(normalizeStackHistoryEntry)),

  getMonitoring: (stackId: string) =>
    api
      .get<StackMonitoringSnapshot>(`/stacks/${stackId}/monitoring`)
      .then((r) => r.data),

  getVersionDiff: (stackId: string, from: number, to: number) =>
    api
      .get<
        import("../../../types").StackVersionDiff
      >(`/stacks/${stackId}/history/diff`, { params: { versionA: from, versionB: to } })
      .then((r) => r.data),

  rollbackStack: (stackId: string, version: number, preservePVC: boolean) =>
    api
      .post<{
        id: string;
      }>(`/stacks/${stackId}/rollback`, { version, preservePVC })
      .then((r) => r.data),

  getCompatibilityMatrix: () =>
    api
      .get<RawCompatibilityMatrix[]>("/stacks/compatibility")
      .then((r) => (r.data ?? []).map(normalizeCompatibilityMatrix)),

  // F8-Phase5 admin CRUD — create/update/delete compatibility matrices.
  // Wire body in snake_case to match the backend matrixPayload struct.
  createMatrix: (input: MatrixInput) =>
    api
      .post<RawCompatibilityMatrix>(
        "/admin/compatibility/matrices",
        matrixInputToPayload(input),
      )
      .then((r) => normalizeCompatibilityMatrix(r.data)),

  updateMatrix: (input: MatrixInput) =>
    api
      .put<RawCompatibilityMatrix>(
        `/admin/compatibility/matrices/${input.id}`,
        matrixInputToPayload(input),
      )
      .then((r) => normalizeCompatibilityMatrix(r.data)),

  deleteMatrix: (id: string) =>
    api
      .delete<void>(`/admin/compatibility/matrices/${id}`)
      .then(() => undefined),

  validateCompatibility: (input: ValidateCompatibilityInput) => {
    const { stackId, tools, clusterId, nodeArchitectures } = input;
    const body: Record<string, unknown> = {};
    if (tools && Object.keys(tools).length > 0) {
      body.tools = tools;
    }
    if (clusterId) {
      body.cluster_id = clusterId;
    }
    if (nodeArchitectures && nodeArchitectures.length > 0) {
      body.node_architectures = nodeArchitectures;
    }
    return api
      .post<RawCompatibilityValidationResult>(
        `/stacks/${stackId}/validate`,
        body,
      )
      .then((r) => normalizeCompatibilityValidationResult(r.data ?? {}));
  },

  createTemplate: (request: TemplateMutationRequest) =>
    api
      .post<
        import("../../../types").StackTemplate
      >("/stacks/templates", request)
      .then((r) => r.data),

  updateTemplate: (request: TemplateMutationRequest) =>
    api
      .put<
        import("../../../types").StackTemplate
      >(`/stacks/templates/${request.id}`, request)
      .then((r) => r.data),

  deleteTemplate: (id: string) =>
    api.delete<void>(`/stacks/templates/${id}`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: RawClusterSummary[] }>("/admin/clusters").then((r) =>
      (r.data?.items ?? []).map((cluster) => ({
        id: cluster.id,
        name: cluster.name,
        connection_status:
          cluster.connection_status ?? cluster.status ?? "pending",
      })),
    ),

  getClusterK8sVersion: (clusterId: string) =>
    api
      .post<RawClusterVerifyResult>(`/admin/clusters/${clusterId}/verify`)
      .then((r) => (r.data?.version ?? "").trim()),

  deployStack: (input: DeployStackInput | string) => {
    const { stackId, acknowledgeWarnings } =
      typeof input === "string"
        ? { stackId: input, acknowledgeWarnings: false }
        : input;
    const body = acknowledgeWarnings
      ? { acknowledge_warnings: true }
      : undefined;
    return api
      .post<{
        stack_id: string;
        status: string;
      }>(`/stacks/${stackId}/deploy`, body)
      .then((r) => r.data);
  },

  // retryStack — F8 follow-up Phase 3. Invokes POST /stacks/:id/retry to
  // rewind a failed/rolled_back stack to pending and re-run the install
  // pipeline. Same acknowledge_warnings contract as deployStack.
  retryStack: (input: DeployStackInput) => {
    const body = input.acknowledgeWarnings
      ? { acknowledge_warnings: true }
      : undefined;
    return api
      .post<{
        stack_id: string;
        status: string;
      }>(`/stacks/${input.stackId}/retry`, body)
      .then((r) => r.data);
  },

  continueStack: (input: DeployStackInput) => {
    const body = input.acknowledgeWarnings
      ? { acknowledge_warnings: true }
      : undefined;
    return api
      .post<{
        stack_id: string;
        status: string;
      }>(`/stacks/${input.stackId}/continue`, body)
      .then((r) => r.data);
  },

  getWorkloads: (stackId: string) =>
    api.get<StackWorkloads>(`/stacks/${stackId}/workloads`).then((r) => r.data),

  testStorageConnection: (input: StorageTestRequest) =>
    api
      .post<StorageTestResult>("/stacks/storage/test", input)
      .then((r) => r.data),

  getIntegrations: (stackId: string) =>
    api
      .get<StackIntegrationsResponse>(`/stacks/${stackId}/integrations`)
      .then((r) => r.data),

  exportStackConfig: async (stackId: string, format: StackExportFormat) => {
    const response = await api.get<Blob>(`/stacks/${stackId}/export`, {
      params: { format },
      responseType: "blob",
    });

    return {
      blob: response.data,
      filename:
        parseContentDispositionFilename(
          response.headers?.["content-disposition"],
        ) ?? `stack-${stackId}.${format}`,
      contentType:
        response.headers?.["content-type"] ??
        (format === "yaml" ? "application/x-yaml" : "application/json"),
    };
  },
};

// --- Hooks ---

export function useTemplates() {
  return useQuery({
    queryKey: queryKeys.templates(),
    queryFn: stackApiCalls.getTemplates,
  });
}

export function useClusters() {
  return useQuery({
    queryKey: queryKeys.clusters(),
    queryFn: stackApiCalls.getClusters,
  });
}

export function useClusterK8sVersion() {
  return useMutation({
    mutationFn: (clusterId: string) =>
      stackApiCalls.getClusterK8sVersion(clusterId),
  });
}

export function useTestStorageConnection() {
  return useMutation({
    mutationFn: (input: StorageTestRequest) =>
      stackApiCalls.testStorageConnection(input),
  });
}

export function useStacks(
  filters?: { status?: string; search?: string; include_deleted?: boolean },
  options?: { refetchIntervalMs?: number },
) {
  return useQuery({
    queryKey: queryKeys.list(filters),
    queryFn: () => stackApiCalls.getList(filters),
    refetchInterval: (query) =>
      stackListRefetchInterval(query.state.data, options?.refetchIntervalMs),
  });
}

export function useStackIntegrations(stackId: string) {
  return useQuery({
    queryKey: queryKeys.integrations(stackId),
    queryFn: () => stackApiCalls.getIntegrations(stackId),
    enabled: !!stackId,
  });
}

export function useCreateStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.create,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

export function useAddTools() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      stackId,
      tools,
    }: {
      stackId: string;
      tools: Array<{ category: string; tool: string; version: string }>;
    }) => api.patch(`/stacks/${stackId}/tools`, { tools }).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

export function useDeleteStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (stackId: string) => stackApiCalls.delete(stackId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

export function useSaveDraft() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.saveDraft,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

export function useEstimateResources() {
  return useMutation({
    mutationFn: stackApiCalls.estimateResources,
  });
}

export function useResourceDefaults() {
  return useQuery({
    queryKey: queryKeys.resourceDefaults(),
    queryFn: stackApiCalls.getResourceDefaults,
  });
}

export function useUpsertResourceDefault() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.upsertResourceDefault,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.resourceDefaults() });
    },
  });
}

export function useStackHistory(stackId: string) {
  return useQuery({
    queryKey: queryKeys.history(stackId),
    queryFn: () => stackApiCalls.getHistory(stackId),
    enabled: !!stackId,
  });
}

// F8-UIUX-RetryAuditSurface-Frontend — load retry audit entries for the
// deployment logs page. staleTime 30s keeps the panel quiet while still
// surfacing brand-new retries without requiring a hard refresh.
export function useStackRetryHistory(stackId: string | undefined) {
  return useQuery<{ items: import("../../../types").RetryHistoryEntry[] }>({
    queryKey: ["stack-retry-history", stackId],
    queryFn: async () => {
      const res = await api.get<{
        items: import("../../../types").RetryHistoryEntry[];
      }>(`/stacks/${stackId}/retry-history`);
      return res.data;
    },
    enabled: Boolean(stackId),
    staleTime: 30_000,
  });
}

export function useStackMonitoring(stackId: string, refetchIntervalMs = 5000) {
  return useQuery({
    queryKey: queryKeys.monitoring(stackId),
    queryFn: () => stackApiCalls.getMonitoring(stackId),
    enabled: !!stackId,
    refetchInterval: refetchIntervalMs,
    staleTime: 0,
  });
}

export function useStackVersionDiff(stackId: string, from: number, to: number) {
  return useQuery({
    queryKey: queryKeys.versionDiff(stackId, from, to),
    queryFn: () => stackApiCalls.getVersionDiff(stackId, from, to),
    enabled: !!stackId && from > 0 && to > 0,
  });
}

export function useRollbackStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      stackId,
      version,
      preservePVC,
    }: {
      stackId: string;
      version: number;
      preservePVC: boolean;
    }) => stackApiCalls.rollbackStack(stackId, version, preservePVC),
    onSuccess: (_, variables) => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
      void qc.invalidateQueries({
        queryKey: queryKeys.history(variables.stackId),
      });
    },
  });
}

export function useCompatibilityMatrix() {
  return useQuery({
    queryKey: queryKeys.compatibilityMatrix(),
    queryFn: stackApiCalls.getCompatibilityMatrix,
  });
}

export function useValidateCompatibility(defaultStackId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input?: ValidateCompatibilityInput | string) => {
      const normalized: ValidateCompatibilityInput =
        typeof input === "string"
          ? { stackId: input }
          : (input ?? { stackId: defaultStackId ?? "" });
      return stackApiCalls.validateCompatibility(normalized);
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() });
    },
  });
}

export function useCreateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.createTemplate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() });
    },
  });
}

export function useUpdateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.updateTemplate,
    onSuccess: (_, variables) => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() });
      void qc.invalidateQueries({ queryKey: queryKeys.template(variables.id) });
    },
  });
}

export function useDeleteTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: stackApiCalls.deleteTemplate,
    onSuccess: (_, id) => {
      void qc.invalidateQueries({ queryKey: queryKeys.templates() });
      void qc.invalidateQueries({ queryKey: queryKeys.template(id) });
    },
  });
}

export function useDeployStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: DeployStackInput | string) =>
      stackApiCalls.deployStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

// useRetryStack — F8 follow-up Phase 3. Drives POST /stacks/:id/retry from
// UI. Invalidates the stack list cache so Retry buttons update.
export function useRetryStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: DeployStackInput) => stackApiCalls.retryStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

export function useContinueStack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: DeployStackInput) => stackApiCalls.continueStack(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["stacks", "list"] });
    },
  });
}

// F8-Phase5 (재개) matrix CRUD mutations. Each onSuccess invalidates the
// compatibility cache so the Stack Version Management page refetches.
export function useCreateMatrix() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: MatrixInput) => stackApiCalls.createMatrix(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() });
    },
  });
}

export function useUpdateMatrix() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: MatrixInput) => stackApiCalls.updateMatrix(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() });
    },
  });
}

export function useDeleteMatrix() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => stackApiCalls.deleteMatrix(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.compatibilityMatrix() });
    },
  });
}

export function useStackWorkloads(stackId: string) {
  return useQuery({
    queryKey: queryKeys.workloads(stackId),
    queryFn: () => stackApiCalls.getWorkloads(stackId),
    enabled: !!stackId,
    refetchInterval: 30_000,
  });
}

export function useExportStackConfig() {
  return useMutation({
    mutationFn: ({
      stackId,
      format,
    }: {
      stackId: string;
      format: StackExportFormat;
    }) => stackApiCalls.exportStackConfig(stackId, format),
  });
}

export function useImportStackConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ payload, replaceExisting = false }: { payload: string; replaceExisting?: boolean }) =>
      api
        .post<{ id: string }>(`/stacks/import?replace_existing=${replaceExisting}`, payload, {
          headers: { "Content-Type": "text/plain" },
        })
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.list() });
    },
  });
}

export function usePreviewImportStackConfig() {
  return useMutation({
    mutationFn: (payload: string) =>
      api
        .post<{
          mode: "create" | "update";
          name: string;
          cluster_id: string;
          existing_stack_id?: string;
          existing_state?: string;
          changes?: {
            added: Record<string, unknown>;
            removed: Record<string, unknown>;
            changed: Record<string, [unknown, unknown]>;
          };
        }>("/stacks/import/preview", payload, {
          headers: { "Content-Type": "text/plain" },
        })
        .then((r) => r.data),
  });
}
