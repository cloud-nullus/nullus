export type Role = "admin" | "devops" | "developer";

export type Theme = "dark" | "light";

export type OrgStatus = "active" | "inactive" | "suspended";

export type MemberRole = Role;

export type MemberStatus = "active" | "pending" | "inactive";

export type ClusterType = "pipeline" | "target";

export type CloudProvider =
  | "aws"
  | "azure"
  | "gcp"
  | "oci"
  | "ibm_cloud"
  | "alibaba_cloud"
  | "tencent_cloud"
  | "naver_cloud"
  | "kt_cloud"
  | "nhn_cloud"
  | "on_premise";

export type ClusterStatus =
  | "connected"
  | "pending"
  | "error"
  | "inactive"
  | "unreachable"
  | "auth_failed";

export type DeploymentState =
  | "running"
  | "success"
  | "failed"
  | "terminating"
  | "pending"
  | "cancelled"
  | "validating"
  | "installing"
  | "configuring"
  | "health_check"
  | "completed"
  | "rolling_back"
  | "rolled_back"
  | "deleted";

export type StackStatus = DeploymentState;

export type PipelineStatus = DeploymentState | "active" | "inactive";

export type AppType =
  | "web"
  | "backend"
  | "batch"
  | "web-backend"
  | "web-frontend"
  | "batch-job";

export type AppTemplate =
  | "go-web-api"
  | "react-vite"
  | "react-spa"
  | "next-app"
  | "express-api"
  | "spring-boot"
  | "python-fastapi";

export type AlertSeverity = "critical" | "warning" | "info";

export type AlertChannel = "slack" | "email";

export type ToolHealthStatus = "running" | "warning" | "error";

export interface User {
  id: string;
  name: string;
  email: string;
  role: Role;
  orgId?: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  domain: string;
  status: OrgStatus;
  clusterAccessScope: string[];
  createdAt: string;
}

export interface OrgResourceProfile {
  id: string;
  name: string;
  orgId: string;
  baseProfile: "local" | "startup" | "standard" | "enterprise";
  optionOverrides: Record<string, Record<string, number>>;
  appliedResourceOverrides?: Record<
    string,
    {
      cpuRequest: number;
      cpuLimit: number;
      memoryRequestGi: number;
      memoryLimitGi: number;
      storageRequestGi: number;
      storageLimitGi: number;
    }
  >;
  rowUnits?: Record<string, { memory: "Gi" | "Mi"; storage: "Gi" | "Mi" }>;
  createdAt?: string;
}

export interface Member {
  id: string;
  name: string;
  email: string;
  role: MemberRole;
  status: MemberStatus;
  joinedAt: string;
}

export interface Cluster {
  id: string;
  name: string;
  type: ClusterType;
  types: ClusterType[];
  cloudProvider: CloudProvider;
  endpoint: string;
  status: ClusterStatus;
  organizationIds: string[];
  // organizationNames 는 organizationIds 를 사람이 읽는 이름으로 옮긴 것이다.
  // 서버가 이름을 못 찾으면 해당 항목이 비고, 화면은 ID 로 되돌아간다.
  organizationNames?: Record<string, string>;
  kubeconfig?: string;
  createdAt: string;
  // nodeArchitectures is the sorted, de-duplicated set of
  // node.status.nodeInfo.architecture values from the cluster. Populated by
  // admin discovery flows (POST /clusters/:id/refresh-discovery) and
  // consumed by the Stack Pre-Deploy Gate. Empty array means "not yet
  // discovered" — treated as unknown, not as "no nodes."
  nodeArchitectures: string[];
}

export interface ToolSelection {
  tool: string;
  version: string;
}

export interface TemplateToolDetail {
  category: string;
  name: string;
  helm_version: string;
  app_version: string;
}

export interface StackResourcesInput {
  developerCount: number;
  concurrentRunners: number;
  commitsPerDay: number;
  buildFrequency: string;
  currency: string;
}

export type StorageMode = "existing" | "create";

export type StoragePlanMode = "existing-all" | "integrated-create" | "none";

export interface StorageTargetInput {
  mode: StorageMode;
  existingRef: string;
  endpoint: string;
  resourceName: string;
  accessSecretRef: string;
  authId: string;
  authPasswordKey: string;
  providerOrEngine: string;
  version: string;
  size: "small" | "medium" | "large";
}

export interface StackStorageInput {
  planMode: StoragePlanMode;
  database: StorageTargetInput;
  objectStorage: StorageTargetInput;
  /** 스택이 만드는 모든 PVC 가 사용할 StorageClass. 빈 값은 클러스터 기본값 사용 */
  storageClass?: string;
}

export interface AccessDomainTlsInput {
  enabled: boolean;
  secretName: string;
  secretNamespace: string;
  issuerName: string;
}

/**
 * 클러스터 밖 소스 저장소(GitHub)에 붙기 위한 설정.
 *
 * 자격증명(PAT)은 여기 없다 — 배포 요청 본문으로만 보낸다. 이 구조는
 * 스택 구성으로 저장되어 조회 API 로 다시 내려오므로 토큰을 넣으면
 * 스택을 볼 수 있는 누구에게나 노출된다.
 */
export interface SourceControlInput {
  /** 리포지토리가 만들어질 GitHub Organization 또는 사용자 계정. */
  owner: string;
  /** GitHub Enterprise Server 의 API 주소. 비면 github.com. */
  apiBaseUrl: string;
}

export interface StackConfig {
  templateId: string | null;
  clusterId: string | null;
  namespace?: string;
  stackName: string;
  accessDomain?: string;
  accessDomainTls?: AccessDomainTlsInput;
  authentication?: {
    provider?: "" | "openbao";
  };
  sourceControl?: SourceControlInput;
  yamlOverrides?: Record<string, string>;
  artifacts: Record<string, ToolSelection>;
  pipeline: Record<string, ToolSelection>;
  monitoring: Record<string, ToolSelection>;
  logging: Record<string, ToolSelection>;
  resources: StackResourcesInput;
  optionOverrides?: Record<string, Record<string, number>>;
  appliedResourceOverrides?: Record<
    string,
    {
      cpuRequest: number;
      cpuLimit: number;
      memoryRequestGi: number;
      memoryLimitGi: number;
      storageRequestGi: number;
      storageLimitGi: number;
    }
  >;
  rowUnits?: Record<string, { memory: "Gi" | "Mi"; storage: "Gi" | "Mi" }>;
  storage?: StackStorageInput;
}

export interface Stack {
  id: string;
  name: string;
  templateId: string;
  templateName: string;
  clusterId: string;
  clusterName: string;
  namespace?: string;
  status: StackStatus;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
}

/**
 * 설치 규모 프로파일. 설치 마법사의 리소스 계획이 이 값에서 시작한다.
 *
 * 도구 선택만으로는 스택이 몇 Gi 를 먹을지 정해지지 않는다 — 같은
 * Gitea + Jenkins + Argo CD 라도 standard 로 깔면 8Gi 노드에 들어가지 않는다.
 * 그래서 템플릿이 "무엇을 깔지"와 함께 "얼마나 크게 깔지"도 들고 다닌다.
 * 서버의 domain.PlanningProfile* 과 같은 어휘다.
 */
export type PlanningProfile = "local" | "startup" | "standard" | "enterprise";

export const PLANNING_PROFILE_VALUES: PlanningProfile[] = [
  "local",
  "startup",
  "standard",
  "enterprise",
];

export const DEFAULT_PLANNING_PROFILE: PlanningProfile = "standard";

export interface Template {
  id: string;
  name: string;
  description: string;
  tools: string[];
  toolDetails?: TemplateToolDetail[];
  estimatedMinutes: number;
  category: string;
  createdBy?: string;
  recommendedUseCase?: string;
  minResources?: string;
  planningProfile: PlanningProfile;
}

export interface StackHistoryEntry {
  id: string;
  stackId: string;
  version: number;
  changedBy: string;
  changedAt: string;
  reason: string;
  snapshot: Record<string, unknown>;
}

export interface StackVersionDiff {
  added: Record<string, unknown>;
  removed: Record<string, unknown>;
  changed: Record<string, [unknown, unknown]>;
}

export type CompatibilityTier = "stable" | "beta" | "deprecated";

export interface CompatibilityTool {
  name: string;
  helmVersion: string;
  appVersion: string;
  // archSupport lists the CPU architectures the tool publishes images for
  // (F8 Task 1). Empty array is interpreted as "amd64-only" for backward
  // compatibility with v1 matrices that predate the field.
  archSupport: string[];
  // minK8sVersion is the per-tool minimum Kubernetes version (F8 Task 1).
  // Empty string means "inherit from matrix k8sRange min."
  minK8sVersion: string;
  // tier is the maturity of this tool inside the matrix (F8 Task 1).
  // Distinct from the matrix-level status.
  tier: CompatibilityTier;
}

export interface CompatibilityMatrix {
  id: string;
  name: string;
  status: "verified" | "untested" | "unsupported";
  k8sRange: string;
  tools: CompatibilityTool[];
}

export interface CompatibilityIssue {
  tool: string;
  message: string;
  severity: "error" | "warning";
  code?: string;
}

export interface CompatibilityValidationResult {
  compatible: boolean;
  overall: {
    state: "pass" | "warn" | "fail";
    score: number;
  };
  issues: CompatibilityIssue[];
  // nodeArchitectures reflects the Pre-Deploy Gate's view of the target
  // cluster's fleet (F8 Task 3). Always normalized/sorted server-side.
  nodeArchitectures: string[];
  // matrix is the server's view of the matched matrix row, if any.
  matrix?: CompatibilityMatrix;
  // message is a human-readable summary returned by the server.
  message?: string;
  checkedAt: string;
}

export interface ResourceEstimate {
  cpu: string;
  memory: string;
  storage: string;
  estimatedCostMonthly: number;
  currency: string;
}

export interface StackResourceDefault {
  tool_key: string;
  display_name: string;
  cpu_request: number;
  cpu_limit: number;
  memory_request_gi: number;
  memory_limit_gi: number;
  storage_request_gi: number;
  storage_limit_gi: number;
  is_default: boolean;
  updated_at: string;
}

export interface Pipeline {
  id: string;
  name: string;
  mode: "ci" | "cd" | "ci_cd";
  appType: AppType;
  templateId: string;
  stackId: string;
  executionMode: string;
  gitRepoUrl: string;
  clusterId: string;
  clusterName: string;
  namespace: string;
  dockerfilePath: string;
  dockerContext: string;
  envVars: Record<string, string>;
  status: PipelineStatus;
  lastDeployedAt: string | null;
  createdAt: string;
}

export interface Deployment {
  id: string;
  pipelineId: string;
  pipelineName: string;
  version: string;
  status: PipelineStatus;
  triggeredBy: string;
  startedAt: string;
  completedAt: string | null;
}

export interface PipelineResource {
  kind: string;
  name: string;
  namespace: string;
  stage: "ingress" | "service" | "workload" | "pod" | "job" | string;
  status: string;
  labelSelector?: string;
  serviceUrls?: string[];
}

export interface CICDTemplate {
  id: string;
  name: string;
  description: string;
  appType: AppType;
  stages: string[];
  createdBy?: string;
  gitRepoUrl?: string;
  dockerfilePath?: string;
  dockerContext?: string;
  envVars?: Record<string, string>;
}

export interface AppTemplateInfo {
  id: AppTemplate;
  name: string;
  description?: string;
  language?: string;
  port?: number;
  runtime?: string;
}

export interface DashboardMetrics {
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
  podCount: number;
  podRunning: number;
}

export interface PipelineMetrics {
  successRate: number;
  totalRuns: number;
  avgBuildSeconds: number;
}

export interface ToolHealth {
  name: string;
  version: string;
  status: ToolHealthStatus;
}

export interface MonitoringDashboard {
  kpi: DashboardMetrics;
  pipeline: PipelineMetrics;
  tools: ToolHealth[];
}

export interface AlertRule {
  id: string;
  name: string;
  metric_name: string;
  condition: string;
  warning_threshold: number;
  critical_threshold: number;
  threshold: number;
  channel: AlertChannel;
  enabled: boolean;
  createdAt?: string;
}

export interface AlertHistory {
  id: string;
  ruleName: string;
  severity: AlertSeverity;
  message: string;
  firedAt: string;
  resolvedAt: string | null;
}

export interface UpdateOrgRequest {
  name?: string;
  slug?: string;
  domain?: string;
  status?: OrgStatus;
  clusterAccessScope?: string[];
}

export interface CreateOrgRequest {
  name: string;
  slug: string;
  domain?: string;
  status: OrgStatus;
}

export interface InviteMemberRequest {
  name: string;
  email: string;
  role: MemberRole;
}

export interface CreateClusterRequest {
  name: string;
  type?: ClusterType;
  types: ClusterType[];
  cloudProvider: CloudProvider;
  endpoint?: string;
  kubeconfig: string;
}

export type KnownIssueSeverity = "high" | "medium" | "low";

export type KnownIssueStatus = "open" | "acknowledged" | "planned";

export interface KnownIssue {
  id: string;
  severity: KnownIssueSeverity;
  title: string;
  description: string;
  workaround: string;
  status: KnownIssueStatus;
}

export type CreateStackRequest = StackConfig;

export interface CreatePipelineRequest {
  name: string;
  appType: AppType;
  clusterId: string;
  namespace?: string;
  templateId?: string;
  stackId?: string;
  gitRepoUrl?: string;
  dockerfilePath?: string;
  dockerContext?: string;
  executionMode?: string;
  envVars?: Record<string, string>;
}

export interface CreateCicdTemplateRequest {
  id: string;
  name: string;
  description: string;
  appType: AppType;
  stages: string[];
}

export interface DeployAppRequest {
  appName: string;
  gitUrl: string;
  clusterId: string;
  namespace: string;
  templateId: AppTemplate;
  stackId?: string;
  replicas?: number;
  port?: number;
  resources: {
    cpuRequest: string;
    cpuLimit: string;
    memRequest: string;
    memLimit: string;
  };
  envVars: Record<string, string>;
}

export interface DeployAppResult {
  deploymentId: string;
  appName: string;
  status: PipelineStatus;
}

export interface CreateAlertRuleRequest {
  name: string;
  metric_name: string;
  warning_threshold: number;
  critical_threshold: number;
  channel: AlertChannel;
  enabled?: boolean;
}

export interface RetryHistoryEntry {
  id: string;
  timestamp: string;
  actor: string;
  previousState?: string;
  acknowledgeWarnings: boolean;
  verdict?: string;
  issueCodes?: string[];
}

// K8s Workload monitoring types for Stack → CI/CD integration
export interface K8sObject {
  kind: string;
  name: string;
  namespace: string;
  status: string;
  replicas?: number;
  port?: number;
  host?: string;
  node?: string;
  /** 이 워크로드를 만든 CI/CD 템플릿. 스캐폴딩 이전 배포에는 없다. */
  templateId?: string;
  /**
   * 파드 실사용량. metrics-server 가 없으면 null 이다 —
   * 0 과 구분해야 한다. 0 은 "안 쓰고 있다" 로 읽힌다.
   */
  cpuMillicores?: number | null;
  memoryMib?: number | null;
}

export interface WorkloadDeployment {
  id: string;
  status: string;
  startedAt: string;
  version: string;
}

export interface StackWorkloadPipeline {
  id: string;
  name: string;
  namespace: string;
  status: string;
  lastDeployment: WorkloadDeployment | null;
  k8sObjects: K8sObject[];
}

export interface StackWorkloadSummary {
  totalPipelines: number;
  totalDeployments: number;
  runningPods: number;
  pendingPods: number;
  failedPods: number;
}

export interface StackWorkloads {
  pipelines: StackWorkloadPipeline[];
  summary: StackWorkloadSummary;
}

/** 배포된 앱 컨테이너가 stdout 으로 뱉은 한 줄. 파드들을 시간순으로 섞어 준다. */
export interface StackWorkloadLogLine {
  pod: string;
  app: string;
  timestamp: string;
  message: string;
}

export interface StackWorkloadLogs {
  lines: StackWorkloadLogLine[];
  /** 실제로 읽은 파드. 화면이 "어디서 온 로그인지" 를 말할 수 있어야 한다. */
  pods: string[];
  /** 파드가 상한보다 많아 일부만 읽었다. */
  truncated: boolean;
}

export type StackTemplate = Template;
export type CicdTemplate = CICDTemplate;
export type KpiMetrics = DashboardMetrics;
export type PipelineStats = PipelineMetrics;
export type ToolHealthState = ToolHealthStatus;
export type AlertHistoryEntry = AlertHistory;
export type OrgMember = Member;
