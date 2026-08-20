import type { ClusterStatus, PlanningProfile } from "../../../types";

export interface TemplateMutationRequest {
  id: string;
  name: string;
  description: string;
  tools: unknown[];
  estimated_install_time: number;
  recommended_use_case: string;
  min_resources: string;
  planning_profile: PlanningProfile;
}

export type {
  CompatibilityMatrix,
  CompatibilityValidationResult,
  CreateStackRequest,
  ResourceEstimate,
  RetryHistoryEntry,
  Stack,
  StackHistoryEntry,
  StackTemplate,
  StackVersionDiff,
} from "../../../types";

export interface ClusterSummary {
  id: string;
  name: string;
  connection_status: ClusterStatus;
}

export interface PodMonitoringStatus {
  name: string;
  phase: string;
  ready: boolean;
  restart_count: number;
  node_name: string;
  cpu_request_millicores: number;
  cpu_limit_millicores: number;
  cpu_usage_millicores: number;
  memory_request_mib: number;
  memory_limit_mib: number;
  memory_usage_mib: number;
  storage_request_gib?: number;
  storage_limit_gib?: number;
  storage_usage_gib?: number;
  status: "running" | "warning" | "error";
}

export interface OSSMonitoringStatus {
  key: string;
  name: string;
  version: string;
  enabled: boolean;
  status: "running" | "warning" | "error";
  /**
   * 이 도구의 웹 주소. 서버가 스택의 접속 도메인에서 만들어 내려준다.
   * 접속 도메인이 없거나 주소 규칙을 모르는 도구는 비어 있다 —
   * 화면이 도메인으로 주소를 다시 조립하면 서버 규칙과 갈라진다.
   */
  url?: string;
  pod_count: number;
  ready_pods: number;
  pods: PodMonitoringStatus[];
}

export interface StackMonitoringSummary {
  total_pods: number;
  ready_pods: number;
  cpu_request_millicores: number;
  cpu_limit_millicores: number;
  cpu_usage_millicores: number;
  memory_request_mib: number;
  memory_limit_mib: number;
  memory_usage_mib: number;
  storage_request_gib: number;
  storage_limit_gib: number;
  storage_usage_gib: number;
  storage_usage_available?: boolean;
  usage_available: boolean;
}

export interface InstalledResourceStatus {
  kind: string;
  name: string;
  desired_replicas: number;
  ready_replicas: number;
  available_replicas: number;
  status: "running" | "warning" | "error";
}

export interface StackMonitoringSnapshot {
  stack_id: string;
  namespace: string;
  timestamp: string;
  summary: StackMonitoringSummary;
  pod_status_counts: Array<{ name: string; count: number }>;
  installed_resources: InstalledResourceStatus[];
  oss_statuses: OSSMonitoringStatus[];
}

export interface StackIntegration {
  id: string;
  stack_id: string;
  component_type: string;
  provider: string;
  endpoint: string;
  api_endpoint: string;
  credential_ref?: string;
  credential_ready: boolean;
  health_status: string;
  provisioning_capabilities: string[];
  metadata?: Record<string, unknown>;
}

export interface StackIntegrationsResponse {
  stack_id: string;
  state: string;
  integrations: StackIntegration[];
  total: number;
}

export interface ValidateCompatibilityInput {
  stackId: string;
  // clusterId tells the backend to resolve node architectures from the
  // admin module's cluster record (F8 Task 3). Takes precedence over
  // nodeArchitectures when both are set server-side.
  clusterId?: string;
  // nodeArchitectures is the explicit override — useful in the wizard
  // before a stack row exists or when the caller already has the fleet
  // layout in hand.
  nodeArchitectures?: string[];
  // tools map is forwarded to the server's tool-based matrix matcher. If
  // omitted, the server falls back to its default Validate flow.
  tools?: Record<string, string>;
}

// F8-Phase5 matrix CRUD input type. Mirrors the backend matrixPayload but
// uses camelCase on the TS side; `matrixInputToPayload` flips to snake_case
// for the wire.
export interface MatrixInput {
  id: string;
  name: string;
  status: "verified" | "untested" | "unsupported";
  kubernetes: { min: string; max: string; recommended: string };
  tools: Record<
    string,
    {
      name: string;
      helmVersion: string;
      appVersion: string;
      minK8sVersion?: string;
      archSupport?: string[];
      tier?: "stable" | "beta" | "deprecated";
    }
  >;
}

export interface DeployStackInput {
  stackId: string;
  /**
   * 외부 SCM(GitHub) PAT. 스택 구성이 아니라 이 요청 본문으로만 보낸다 —
   * 구성은 평문으로 저장되고 조회 API 로 다시 내려오기 때문이다.
   * 서버는 값을 스택의 OpenBao 로 옮기고 응답·감사 로그에는 싣지 않는다.
   */
  sourceControlToken?: string;
  // acknowledgeWarnings opts in to proceeding when the server-side
  // Pre-Deploy Gate (F8-F3) returns overall.state == "warn". Defaults to
  // false so legacy clients that pass a bare stackId are blocked on warn
  // instead of silently installing.
  acknowledgeWarnings?: boolean;
}

/** 서버가 확정해 내려주는 스토리지 접속 정보. */
export type StorageConnectionResponse = {
  mode: string;
  engine: string;
  endpoint: string;
  resource_name: string;
  auth_id: string;
  secret_ref?: string;
  secret_key?: string;
};

/**
 * OSS 도구 하나의 접속 안내.
 * secret_ref 가 없으면 조회할 Secret 이 없다는 뜻이고 note 가 설명을 담는다.
 */
export type ToolCredentialResponse = {
  name: string;
  username?: string;
  secret_ref?: string;
  secret_key?: string;
  note?: string;
};

/**
 * 스택 접속 정보. 리소스 이름은 서버가 정한다 —
 * 화면이 다시 조립하면 설치 경로와 규칙이 갈리는 순간 어긋난다.
 */
export type StackConnectionInfoResponse = {
  stack_id: string;
  namespace: string;
  access_domain?: string;
  database: StorageConnectionResponse;
  object_storage: StorageConnectionResponse;
  tools: ToolCredentialResponse[];
};

/**
 * 배포된 스택의 Helm 릴리스 한 건. 설정 편집 화면이 이 목록에서 대상을 고른다.
 * step_name 이 비면 재배포 때 편집이 유지되지 않는다 — 저장할 오버라이드 키를
 * 알 수 없기 때문이다.
 */
export type StackRelease = {
  release_name: string;
  step_name?: string;
  chart_name?: string;
  chart_version?: string;
  app_version?: string;
  namespace: string;
  revision: number;
  status: string;
};

/**
 * 편집 단위.
 * - live: 실제로 배포된 values 전체 (플랫폼이 계산해 넣은 값까지 보인다)
 * - override: 사용자가 얹은 커스텀만
 */
export type ReleaseValuesMode = "live" | "override";

export type ReleaseValuesResponse = {
  release_name: string;
  step_name?: string;
  namespace: string;
  revision: number;
  mode: ReleaseValuesMode;
  yaml: string;
  protected_paths?: string[];
};

/** 플랫폼이 소유한 경로를 건드렸을 때의 경고. 차단이 아니라 안내다. */
export type ProtectedValueWarning = {
  path: string;
  kind: "removed" | "changed";
  message: string;
};

export type ApplyReleaseValuesResponse = {
  release_name: string;
  step_name?: string;
  namespace: string;
  mode: ReleaseValuesMode;
  revision: number;
  status?: string;
  dry_run: boolean;
  warnings?: ProtectedValueWarning[];
  effective_yaml?: string;
  manifest?: string;
  /** 미리보기에서만 채워진다. 차트가 렌더되지 않는 것은 편집 결과이지 서버 오류가 아니다. */
  render_error?: string;
};

export type ApplyReleaseValuesInput = {
  stackId: string;
  releaseName: string;
  mode: ReleaseValuesMode;
  yaml: string;
};
