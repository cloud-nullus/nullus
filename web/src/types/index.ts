export type Role = 'admin' | 'devops' | 'developer'

export type Theme = 'dark' | 'light'

export type OrgStatus = 'active' | 'inactive' | 'suspended'

export type MemberRole = Role

export type MemberStatus = 'active' | 'pending' | 'inactive'

export type ClusterType = 'kubernetes' | 'eks' | 'gke' | 'aks' | 'k3s'

export type ClusterStatus = 'connected' | 'pending' | 'error' | 'inactive'

export type DeploymentState = 'running' | 'success' | 'failed' | 'pending' | 'cancelled'

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

export interface StackResourcesInput {
  developerCount: number
  concurrentRunners: number
  commitsPerDay: number
  buildFrequency: string
  currency: string
}

export interface StackConfig {
  templateId: string | null
  clusterId: string | null
  stackName: string
  artifacts: Record<string, ToolSelection>
  pipeline: Record<string, ToolSelection>
  monitoring: Record<string, ToolSelection>
  logging: Record<string, ToolSelection>
  resources: StackResourcesInput
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
  estimatedMinutes: number
  category: string
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

export interface StackVersionDiffEntry {
  key: string
  value: string
}

export interface StackVersionDiff {
  fromVersion: number
  toVersion: number
  added: StackVersionDiffEntry[]
  removed: StackVersionDiffEntry[]
  changed: { key: string; from: string; to: string }[]
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
}

export interface AppTemplateInfo {
  id: AppTemplate
  name: string
  description: string
  language: string
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

export interface InviteMemberRequest {
  email: string
  role: MemberRole
}

export interface CreateClusterRequest {
  name: string
  type: ClusterType
  kubeconfig: string
}

export type CreateStackRequest = StackConfig

export interface CreatePipelineRequest {
  name: string
  appType: AppType
  clusterId: string
  templateId?: string
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
