import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import type {
  AlertHistoryEntry,
  AlertRule,
  AlertSeverity,
  CreateAlertRuleRequest,
  MonitoringDashboard,
} from '../../../types'

export type {
  AlertChannel,
  AlertHistoryEntry,
  AlertRule,
  AlertSeverity,
  CreateAlertRuleRequest,
  MonitoringDashboard,
  ToolHealthStatus,
} from '../../../types'

// --- Query keys ---

const queryKeys = {
  dashboard: () => ['observability', 'dashboard'] as const,
  alertRules: () => ['observability', 'alert-rules'] as const,
  alertRule: (id: string) => ['observability', 'alert-rules', id] as const,
  alertHistory: (filters?: Record<string, unknown>) => ['observability', 'alert-history', filters] as const,
  stackAlerts: (state: StackAlertState) => ['observability', 'stack-alerts', state] as const,
}

/**
 * 스택 심각 경보 (nullus-plan#87). 스택마다 Prometheus 가 기본 규칙으로 평가하고, 플랫폼이 Alertmanager 에서
 * 가져와 기록한다. 발생 · 해제는 조직의 알림 채널로도 간다.
 */
export type StackAlertState = 'open' | 'all'

export interface StackAlert {
  id: string
  stack_id: string
  stack_name: string
  alertname: string
  severity: string
  summary: string
  description?: string
  runbook_url?: string
  target?: string
  labels: Record<string, string>
  started_at: string
  resolved_at?: string
  last_notified_at?: string
  active: boolean
}

// --- API functions ---

const observabilityApiCalls = {
  getDashboard: () =>
    api.get<MonitoringDashboard>('/observability/dashboard').then((r) => r.data),

  getAlertRules: () =>
    api.get<{ items: AlertRule[]; total: number }>('/observability/alert-rules').then((r) => r.data),

  getAlertRule: (id: string) =>
    api.get<AlertRule>(`/observability/alert-rules/${id}`).then((r) => r.data),

  createAlertRule: (data: CreateAlertRuleRequest) =>
    api.post<AlertRule>('/observability/alert-rules', data).then((r) => r.data),

  updateAlertRule: (id: string, data: Partial<AlertRule>) =>
    api.patch<AlertRule>(`/observability/alert-rules/${id}`, data).then((r) => r.data),

  deleteAlertRule: (id: string) =>
    api.delete(`/observability/alert-rules/${id}`).then((r) => r.data),

  getStackAlerts: (state: StackAlertState) =>
    api
      .get<{ items: StackAlert[]; total: number }>('/observability/stack-alerts', { params: { state, limit: 200 } })
      .then((r) => r.data.items ?? []),

  getAlertHistory: (filters?: { severity?: AlertSeverity }) =>
    api
      .get<{ items: AlertHistoryEntry[]; total: number }>('/observability/alert-history', { params: filters })
      .then((r) => r.data),
}

// --- Hooks ---

export function useDashboard(refetchInterval = 5000) {
  return useQuery({
    queryKey: queryKeys.dashboard(),
    queryFn: observabilityApiCalls.getDashboard,
    refetchInterval,
  })
}

export function useAlertRules() {
  return useQuery({
    queryKey: queryKeys.alertRules(),
    queryFn: observabilityApiCalls.getAlertRules,
  })
}

export function useAlertRule(id: string | null, enabled = true) {
  return useQuery({
    queryKey: queryKeys.alertRule(id ?? ''),
    queryFn: () => observabilityApiCalls.getAlertRule(id ?? ''),
    enabled: enabled && Boolean(id),
  })
}

export function useCreateAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: observabilityApiCalls.createAlertRule,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.alertRules() })
      void qc.invalidateQueries({ queryKey: ['observability', 'alert-history'] })
    },
  })
}

export function useUpdateAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<AlertRule> }) =>
      observabilityApiCalls.updateAlertRule(id, data),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: queryKeys.alertRules() })
      void qc.invalidateQueries({ queryKey: queryKeys.alertRule(variables.id) })
      void qc.invalidateQueries({ queryKey: ['observability', 'alert-history'] })
    },
  })
}

export function useDeleteAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => observabilityApiCalls.deleteAlertRule(id),
    onSuccess: (_data, id) => {
      void qc.invalidateQueries({ queryKey: queryKeys.alertRules() })
      void qc.removeQueries({ queryKey: queryKeys.alertRule(id) })
      void qc.invalidateQueries({ queryKey: ['observability', 'alert-history'] })
    },
  })
}

export function useAlertHistory(filters?: { severity?: AlertSeverity }) {
  return useQuery({
    queryKey: queryKeys.alertHistory(filters),
    queryFn: () => observabilityApiCalls.getAlertHistory(filters),
  })
}

/** 스택 경보 목록. 수집이 1분마다 돌므로 같은 주기로 다시 읽는다. */
export function useStackAlerts(state: StackAlertState) {
  return useQuery({
    queryKey: queryKeys.stackAlerts(state),
    queryFn: () => observabilityApiCalls.getStackAlerts(state),
    refetchInterval: 60_000,
  })
}

export interface DeployedAppPod {
  name: string
  node: string
  status: string
}

export interface DeployedApp {
  id: string
  name: string
  template_id: string
  namespace: string
  cluster_id: string
  cluster_name: string
  git_repo_url: string
  status: string
  state: string
  version: string
  deployed_by: string
  deployed_at: string
  replicas: number
  pods: DeployedAppPod[]
}

export function useDeployedApps() {
  return useQuery({
    queryKey: ['observability', 'deployed-apps'] as const,
    queryFn: () =>
      api.get<{ items: DeployedApp[]; total: number }>('/observability/deployed-apps').then((r) => r.data),
    refetchInterval: 10000,
  })
}
