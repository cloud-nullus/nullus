import type { ClusterStatus } from '../../../types'

export interface TemplateMutationRequest {
  id: string
  name: string
  description: string
  tools: unknown[]
  estimated_install_time: number
  recommended_use_case: string
  min_resources: string
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
} from '../../../types'

export interface ClusterSummary {
  id: string
  name: string
  connection_status: ClusterStatus
}

export interface PodMonitoringStatus {
  name: string
  phase: string
  ready: boolean
  restart_count: number
  node_name: string
  cpu_request_millicores: number
  cpu_limit_millicores: number
  cpu_usage_millicores: number
  memory_request_mib: number
  memory_limit_mib: number
  memory_usage_mib: number
  storage_request_gib?: number
  storage_limit_gib?: number
  storage_usage_gib?: number
  status: 'running' | 'warning' | 'error'
}

export interface OSSMonitoringStatus {
  key: string
  name: string
  version: string
  enabled: boolean
  status: 'running' | 'warning' | 'error'
  pod_count: number
  ready_pods: number
  pods: PodMonitoringStatus[]
}

export interface StackMonitoringSummary {
  total_pods: number
  ready_pods: number
  cpu_request_millicores: number
  cpu_limit_millicores: number
  cpu_usage_millicores: number
  memory_request_mib: number
  memory_limit_mib: number
  memory_usage_mib: number
  storage_request_gib: number
  storage_limit_gib: number
  storage_usage_gib: number
  storage_usage_available?: boolean
  usage_available: boolean
}

export interface InstalledResourceStatus {
  kind: string
  name: string
  desired_replicas: number
  ready_replicas: number
  available_replicas: number
  status: 'running' | 'warning' | 'error'
}

export interface StackMonitoringSnapshot {
  stack_id: string
  namespace: string
  timestamp: string
  summary: StackMonitoringSummary
  pod_status_counts: Array<{ name: string; count: number }>
  installed_resources: InstalledResourceStatus[]
  oss_statuses: OSSMonitoringStatus[]
}

export interface StackIntegration {
  id: string
  stack_id: string
  component_type: string
  provider: string
  endpoint: string
  api_endpoint: string
  credential_ref?: string
  credential_ready: boolean
  health_status: string
  provisioning_capabilities: string[]
  metadata?: Record<string, unknown>
}

export interface StackIntegrationsResponse {
  stack_id: string
  state: string
  integrations: StackIntegration[]
  total: number
}

export interface ValidateCompatibilityInput {
  stackId: string
  // clusterId tells the backend to resolve node architectures from the
  // admin module's cluster record (F8 Task 3). Takes precedence over
  // nodeArchitectures when both are set server-side.
  clusterId?: string
  // nodeArchitectures is the explicit override — useful in the wizard
  // before a stack row exists or when the caller already has the fleet
  // layout in hand.
  nodeArchitectures?: string[]
  // tools map is forwarded to the server's tool-based matrix matcher. If
  // omitted, the server falls back to its default Validate flow.
  tools?: Record<string, string>
}

// F8-Phase5 matrix CRUD input type. Mirrors the backend matrixPayload but
// uses camelCase on the TS side; `matrixInputToPayload` flips to snake_case
// for the wire.
export interface MatrixInput {
  id: string
  name: string
  status: 'verified' | 'untested' | 'unsupported'
  kubernetes: { min: string; max: string; recommended: string }
  tools: Record<string, {
    name: string
    helmVersion: string
    appVersion: string
    minK8sVersion?: string
    archSupport?: string[]
    tier?: 'stable' | 'beta' | 'deprecated'
  }>
}

export interface DeployStackInput {
  stackId: string
  // acknowledgeWarnings opts in to proceeding when the server-side
  // Pre-Deploy Gate (F8-F3) returns overall.state == "warn". Defaults to
  // false so legacy clients that pass a bare stackId are blocked on warn
  // instead of silently installing.
  acknowledgeWarnings?: boolean
}
