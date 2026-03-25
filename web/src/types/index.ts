export type Role = 'admin' | 'devops' | 'developer'

export type Theme = 'dark' | 'light'

export type OrgStatus = 'active' | 'inactive' | 'suspended'

export type MemberRole = Role

export type MemberStatus = 'active' | 'pending' | 'inactive'

export type ClusterType = 'kubernetes' | 'eks' | 'gke' | 'aks' | 'k3s' | 'pipeline' | 'target'

export type ClusterStatus = 'connected' | 'pending' | 'error' | 'inactive' | 'unreachable' | 'auth_failed'

export type DeploymentState =
  | 'running'
  | 'success'
  | 'failed'
  | 'pending'
  | 'cancelled'
  | 'validating'
  | 'installing'
  | 'configuring'
  | 'health_check'
  | 'completed'
  | 'rolling_back'
  | 'rolled_back'

export type StackStatus = DeploymentState

export type PipelineStatus = DeploymentState

export type AppType = 'web-backend' | 'web-frontend' | 'batch-job'

export type AppTemplate = 'react-spa' | 'next-app' | 'express-api' | 'spring-boot' | 'python-fastapi'

export type AlertSeverity = 'critical' | 'warning' | 'info'

export type AlertChannel = 'slack' | 'email'

export type ToolHealthStatus = 'running' | 'warning' | 'error'

export interface User {
  id: string
  name: string
  email: string
  role: Role
  orgId?: string
}

export interface Organization {
  id: string
  name: string
  slug: string
  domain: string
  status: OrgStatus
  clusterAccessScope: string[]
  createdAt: string
}

export interface Member {
  id: string
  name: string
  email: string
  role: MemberRole
  status: MemberStatus
  joinedAt: string
}

export interface Cluster {
  id: string
  name: string
  type: ClusterType
  endpoint: string
  status: ClusterStatus
  organizationIds: string[]
  createdAt: string
}

export interface ToolSelection {
  tool: string
  version: string
}

export interface TemplateToolDetail {
  category: string
  name: string
  helm_version: string
  app_version: string
}

export interface StackResourcesInput {
  developerCount: number
  concurrentRunners: number
  commitsPerDay: number
  buildFrequency: string
  currency: string
}

export type StorageMode = 'existing' | 'create'

export type StoragePlanMode = 'existing-all' | 'integrated-create'

export interface StorageTargetInput {
  mode: StorageMode
  existingRef: string
  endpoint: string
  resourceName: string
  accessSecretRef: string
  authId: string
  authPasswordKey: string
  providerOrEngine: string
  version: string
  size: 'small' | 'medium' | 'large'
}

export interface StackStorageInput {
  planMode: StoragePlanMode
  database: StorageTargetInput
  objectStorage: StorageTargetInput
}

export interface AccessDomainTlsInput {
  enabled: boolean
  secretName: string
  secretNamespace: string
}

export interface StackConfig {
  templateId: string | null
  clusterId: string | null
  namespace?: string
  stackName: string
  accessDomain?: string
  accessDomainTls?: AccessDomainTlsInput
  artifacts: Record<string, ToolSelection>
  pipeline: Record<string, ToolSelection>
  monitoring: Record<string, ToolSelection>
  logging: Record<string, ToolSelection>
  resources: StackResourcesInput
  storage?: StackStorageInput
}

export interface Stack {
  id: string
  name: string
  templateId: string
  templateName: string
  clusterId: string
  clusterName: string
  status: StackStatus
  createdAt: string
  updatedAt: string
}

export interface Template {
  id: string
  name: string
  description: string
  tools: string[]
  toolDetails?: TemplateToolDetail[]
  estimatedMinutes: number
  category: string
  createdBy?: string
  recommendedUseCase?: string
  minResources?: string
}

export interface StackHistoryEntry {
  id: string
  stackId: string
  version: number
  changedBy: string
  changedAt: string
  reason: string
  snapshot: Record<string, unknown>
}

export interface StackVersionDiff {
  added: Record<string, unknown>
  removed: Record<string, unknown>
  changed: Record<string, [unknown, unknown]>
}

export interface CompatibilityTool {
  name: string
  helmVersion: string
  appVersion: string
}

export interface CompatibilityMatrix {
  id: string
  name: string
  status: 'verified' | 'untested'
  k8sRange: string
  tools: CompatibilityTool[]
}

export interface CompatibilityIssue {
  tool: string
  message: string
  severity: 'error' | 'warning'
}

export interface CompatibilityValidationResult {
  compatible: boolean
  issues: CompatibilityIssue[]
  checkedAt: string
}

export interface ResourceEstimate {
  cpu: string
  memory: string
  storage: string
  estimatedCostMonthly: number
  currency: string
}

export interface StackResourceDefault {
  tool_key: string
  display_name: string
  cpu_request: number
  cpu_limit: number
  memory_request_gi: number
  memory_limit_gi: number
  storage_request_gi: number
  storage_limit_gi: number
  is_default: boolean
  updated_at: string
}

export interface Pipeline {
  id: string
  name: string
  appType: AppType
  clusterId: string
  clusterName: string
  status: PipelineStatus
  lastDeployedAt: string | null
  createdAt: string
}

export interface Deployment {
  id: string
  pipelineId: string
  pipelineName: string
  version: string
  status: PipelineStatus
  triggeredBy: string
  startedAt: string
  completedAt: string | null
}

export interface CICDTemplate {
  id: string
  name: string
  description: string
  appType: AppType
  stages: string[]
  createdBy?: string
}

export interface AppTemplateInfo {
  id: AppTemplate
  name: string
  description?: string
  language?: string
  port?: number
  runtime?: string
}

export interface DashboardMetrics {
  cpuUsage: number
  memoryUsage: number
  storageUsage: number
  podCount: number
  podRunning: number
}

export interface PipelineMetrics {
  successRate: number
  totalRuns: number
  avgBuildSeconds: number
}

export interface ToolHealth {
  name: string
  version: string
  status: ToolHealthStatus
}

export interface MonitoringDashboard {
  kpi: DashboardMetrics
  pipeline: PipelineMetrics
  tools: ToolHealth[]
}

export interface AlertRule {
  id: string
  name: string
  condition: string
  threshold: string
  channel: AlertChannel
  enabled: boolean
  createdAt: string
}

export interface AlertHistory {
  id: string
  ruleName: string
  severity: AlertSeverity
  message: string
  firedAt: string
  resolvedAt: string | null
}

export interface UpdateOrgRequest {
  name?: string
  slug?: string
  domain?: string
  status?: OrgStatus
  clusterAccessScope?: string[]
}

export interface CreateOrgRequest {
  name: string
  slug: string
  domain?: string
  status: OrgStatus
}

export interface InviteMemberRequest {
  name: string
  email: string
  role: MemberRole
}

export interface CreateClusterRequest {
  name: string
  type: ClusterType
  kubeconfig: string
}

export type KnownIssueSeverity = 'high' | 'medium' | 'low'

export type KnownIssueStatus = 'open' | 'acknowledged' | 'planned'

export interface KnownIssue {
  id: string
  severity: KnownIssueSeverity
  title: string
  description: string
  workaround: string
  status: KnownIssueStatus
}

export type CreateStackRequest = StackConfig

export interface CreatePipelineRequest {
  name: string
  appType: AppType
  clusterId: string
  templateId?: string
}

export interface CreateCicdTemplateRequest {
  id: string
  name: string
  description: string
  appType: AppType
  stages: string[]
}

export interface DeployAppRequest {
  appName: string
  gitUrl: string
  clusterId: string
  namespace: string
  template: AppTemplate
  resources: {
    cpuRequest: string
    cpuLimit: string
    memoryRequest: string
    memoryLimit: string
  }
  envVars: { key: string; value: string }[]
}

export interface DeployAppResult {
  deploymentId: string
  appName: string
  status: PipelineStatus
}

export interface CreateAlertRuleRequest {
  name: string
  condition: string
  threshold: string
  channel: AlertChannel
}

export type StackTemplate = Template
export type CicdTemplate = CICDTemplate
export type KpiMetrics = DashboardMetrics
export type PipelineStats = PipelineMetrics
export type ToolHealthState = ToolHealthStatus
export type AlertHistoryEntry = AlertHistory
export type OrgMember = Member
