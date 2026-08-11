import { Layers, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCompatibilityMatrix, useValidateCompatibility } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import type { CompatibilityMatrix, CompatibilityValidationResult } from '../api/stack-api'
import { cn } from '../../../lib/utils'
import { useState } from 'react'
import { formatDateTime, resolveLocale } from '../../../lib/locale'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'


const STATUS_BADGE: Record<string, { className: string; key: string; defaultLabel: string }> = {
  verified: { className: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]', key: 'stackVersionPage.status.verified', defaultLabel: 'Verified' },
  untested: { className: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]', key: 'stackVersionPage.status.partial', defaultLabel: 'Partial' },
  unsupported: { className: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]', key: 'stackVersionPage.status.notSupported', defaultLabel: 'Not Supported' },
}

const toolVersion = (matrix: CompatibilityMatrix, keyword: string): string => {
  const lower = keyword.toLowerCase()
  const tool = matrix.tools.find((item) => item.name.toLowerCase().includes(lower))
  return tool ? tool.appVersion : '-'
}

const rowClassName = 'border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm'

export function StackVersionPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [search, setSearch] = useState('')
  const [validationOpen, setValidationOpen] = useState(false)
  const [validating, setValidating] = useState(false)
  const [validationResult, setValidationResult] = useState<CompatibilityValidationResult | null>(null)
  const { data: matrixData } = useCompatibilityMatrix()
  const matrix = Array.isArray(matrixData) ? matrixData : []
  const validateMutation = useValidateCompatibility('current')

  const q = search.trim().toLowerCase()
  const gitlabRows = matrix.filter(
    (item) =>
      item.name.toLowerCase().includes('gitlab') &&
      (!q || item.name.toLowerCase().includes(q) || item.tools.some((t) => t.name.toLowerCase().includes(q) || t.appVersion.toLowerCase().includes(q)))
  )

  const handleValidate = () => {
    setValidationOpen(true)
    setValidating(true)
    validateMutation.mutate(undefined, {
      onSuccess: (result) => {
        setValidationResult(result)
        setValidating(false)
      },
      onError: () => setValidating(false),
    })
  }

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('sidebar.stackVersion', 'Stack Version') }]}
        icon={<Layers size={16} />}
        tone="success"
        title={t('stackVersionPage.title', 'Stack Version')}
        subtitle={t('stackVersionPage.description', 'Manage compatibility based on validated version combinations.')}
        actions={
          <Button variant="primary" size="md" onClick={handleValidate}>
            <ShieldCheck size={15} />
            {t('stackVersionPage.actions.validateCurrentStack', 'Validate Current Stack')}
          </Button>
        }
      />

      <div className="mb-5 rounded-lg border border-[color-mix(in_srgb,_var(--color-info)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-info)_8%,_transparent)] px-4 py-3 text-sm text-[var(--color-text-primary)]">
        {t('stackVersionPage.notice', 'Only validated version combinations are shown. Unverified combinations will display warnings.')}
      </div>

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3">
          <span className="text-sm font-bold text-[var(--color-text-primary)]">{t('stackVersionPage.verifiedCombinations', 'Verified Combinations')}</span>
          <SearchInput
            wrapperClassName="w-[220px]"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('stackVersionPage.searchPlaceholder', 'Search stacks or tools...')}
          />
        </div>
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]">
              {[
                t('stackVersionPage.table.gitlab', 'GitLab'),
                t('stackVersionPage.table.argocd', 'Argo CD'),
                t('stackVersionPage.table.prometheus', 'Prometheus'),
                t('stackVersionPage.table.grafana', 'Grafana'),
                t('stackVersionPage.table.opentelemetry', 'OpenTelemetry'),
                t('stackVersionPage.table.k8s', 'K8s'),
                t('stackVersionPage.table.status', 'Status'),
              ].map((header) => (
                <th key={header} className="px-[14px] py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {gitlabRows.map((item) => {
              const badge = STATUS_BADGE[item.status] ?? STATUS_BADGE.untested
              const recommended = item.id.includes('gitlab-argocd')
              return (
                <tr key={item.id}>
                  <td className={cn(rowClassName, 'font-semibold text-[var(--color-text-primary)]')}>{toolVersion(item, 'gitlab')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'argo')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'prometheus')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'grafana')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'opentelemetry')}</td>
                  <td className={cn(rowClassName, 'font-mono text-[13px] text-[var(--color-text-secondary)]')}>{item.k8sRange}</td>
                  <td className={rowClassName}>
                    <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', badge.className)}>{t(badge.key, badge.defaultLabel)}</span>
                    {recommended && (
                      <span className="ml-1.5 rounded-md bg-[color-mix(in_srgb,_var(--color-accent-alt)_15%,_transparent)] px-[7px] py-[3px] text-[11px] font-semibold text-[var(--color-accent-alt)]">{t('stackVersionPage.recommended', 'Recommended')}</span>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      <Modal
        open={validationOpen}
        onClose={() => {
          setValidationOpen(false)
          setValidationResult(null)
        }}
        title={t('stackVersionPage.validation.title', 'Compatibility Validation Result')}
      >
        {validating && (
          <div className="py-8 text-center text-[var(--color-text-secondary)]">
            <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-[3px] border-[color-mix(in_srgb,_var(--color-text-primary)_10%,_transparent)] border-t-[var(--color-primary)]" />
            {t('stackVersionPage.validation.running', 'Validating...')}
          </div>
        )}
        {!validating && validationResult && (
          <div className="flex flex-col gap-4">
            <div
              className={cn(
                'flex items-center gap-2.5 rounded-lg border px-4 py-3',
                validationResult.compatible
                  ? 'border-[color-mix(in_srgb,_var(--color-success)_30%,_transparent)] bg-[color-mix(in_srgb,_var(--color-success)_10%,_transparent)]'
                  : 'border-[color-mix(in_srgb,_var(--color-error)_30%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_10%,_transparent)]'
              )}
            >
              <ShieldCheck size={20} color={validationResult.compatible ? 'var(--color-success)' : 'var(--color-error)'} />
              <span className={cn('text-sm font-bold', validationResult.compatible ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]')}>
                {validationResult.compatible
                  ? t('stackVersionPage.validation.pass', 'Compatibility validation passed')
                  : t('stackVersionPage.validation.fail', 'Compatibility issues found')}
              </span>
            </div>

            <p className="m-0 text-xs text-[var(--color-text-secondary)]">{t('stackVersionPage.validation.checkedAt', 'Checked at')}: {formatDateTime(validationResult.checkedAt, locale)}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}
