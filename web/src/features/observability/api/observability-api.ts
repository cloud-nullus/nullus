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
