import type { TFunction } from 'i18next'

export type PipelineStatusKey =
  | 'active'
  | 'running'
  | 'success'
  | 'failed'
  | 'pending'
  | 'cancelled'

export const PIPELINE_STATUS_STYLES: Record<PipelineStatusKey, { bg: string; color: string }> = {
  active: { bg: 'color-mix(in srgb, var(--color-success) 15%, transparent)', color: 'var(--color-success)' },
  running: { bg: 'color-mix(in srgb, var(--color-info) 15%, transparent)', color: 'var(--color-info)' },
  success: { bg: 'color-mix(in srgb, var(--color-success) 15%, transparent)', color: 'var(--color-success)' },
  failed: { bg: 'color-mix(in srgb, var(--color-error) 15%, transparent)', color: 'var(--color-error)' },
  pending: { bg: 'color-mix(in srgb, var(--color-warning) 15%, transparent)', color: 'var(--color-warning)' },
  cancelled: { bg: 'color-mix(in srgb, var(--color-text-muted) 15%, transparent)', color: 'var(--color-text-muted)' },
}

export function getPipelineStatusStyle(status: string) {
  return PIPELINE_STATUS_STYLES[(status as PipelineStatusKey)] ?? PIPELINE_STATUS_STYLES.pending
}

export function getPipelineStatusLabel(t: TFunction, status: string) {
  if (status === 'active') return t('cicdListPage.status.active', 'Active')
  if (status === 'running') return t('cicd.status.running', 'Running')
  if (status === 'success') return t('cicd.status.success', 'Success')
  if (status === 'failed') return t('cicd.status.failed', 'Failed')
  if (status === 'pending') return t('cicd.status.pending', 'Pending')
  if (status === 'cancelled') return t('cicd.status.cancelled', 'Cancelled')
  return status
}
