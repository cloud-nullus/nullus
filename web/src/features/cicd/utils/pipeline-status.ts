type Translate = (key: string, defaultValue?: string) => string

export type PipelineStatusKey =
  | 'active'
  | 'running'
  | 'success'
  | 'failed'
  | 'pending'
  | 'cancelled'

export const PIPELINE_STATUS_STYLES: Record<PipelineStatusKey, { bg: string; color: string }> = {
  active: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' },
  running: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' },
  failed: { bg: 'rgba(239,68,68,0.15)', color: '#ef4444' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b' },
  cancelled: { bg: 'rgba(100,116,139,0.15)', color: '#64748b' },
}

export function getPipelineStatusStyle(status: string) {
  return PIPELINE_STATUS_STYLES[(status as PipelineStatusKey)] ?? PIPELINE_STATUS_STYLES.pending
}

export function getPipelineStatusLabel(t: Translate, status: string) {
  if (status === 'active') return t('cicdListPage.status.active', 'Active')
  if (status === 'running') return t('cicd.status.running', 'Running')
  if (status === 'success') return t('cicd.status.success', 'Success')
  if (status === 'failed') return t('cicd.status.failed', 'Failed')
  if (status === 'pending') return t('cicd.status.pending', 'Pending')
  if (status === 'cancelled') return t('cicd.status.cancelled', 'Cancelled')
  return status
}
