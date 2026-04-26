import { AlertTriangle, Layers, Search, ShieldCheck, XCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useClusterK8sVersion, useCompatibilityMatrix, useStacks, useValidateCompatibility } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import type { CompatibilityMatrix, CompatibilityValidationResult, Stack } from '../api/stack-api'
import { cn } from '../../../lib/utils'
import { useMemo, useState } from 'react'
import { formatDateTime, resolveLocale } from '../../../lib/locale'

const STATUS_BADGE: Record<string, { className: string; key: string; defaultLabel: string }> = {
  verified: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', key: 'stackVersionPage.status.verified', defaultLabel: 'Verified' },
  untested: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', key: 'stackVersionPage.status.partial', defaultLabel: 'Partial' },
  unsupported: { className: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', key: 'stackVersionPage.status.notSupported', defaultLabel: 'Not Supported' },
}

const VALIDATION_BADGE = {
  pass: {
    container: 'border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.1)]',
    text: 'text-[#22c55e]',
    icon: '#22c55e',
    key: 'stackVersionPage.validation.pass',
    label: 'Compatibility validation passed',
  },
  warn: {
    container: 'border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.1)]',
    text: 'text-[#f59e0b]',
    icon: '#f59e0b',
    key: 'stackVersionPage.validation.warn',
    label: 'Compatibility warnings found',
  },
  fail: {
    container: 'border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.1)]',
    text: 'text-[#ef4444]',
    icon: '#ef4444',
    key: 'stackVersionPage.validation.fail',
    label: 'Compatibility issues found',
  },
} as const

const toolVersion = (matrix: CompatibilityMatrix, keyword: string): string => {
  const lower = keyword.toLowerCase()
  const tool = matrix.tools.find((item) => item.name.toLowerCase().includes(lower))
  return tool ? tool.appVersion : '-'
}

const matrixSetupType = (matrix: CompatibilityMatrix): 'Helm' | 'Deployment' | 'Mixed' => {
  const helmCount = matrix.tools.filter((tool) => tool.helmVersion && tool.helmVersion !== '-' && tool.helmVersion.toLowerCase() !== 'external').length
  const externalCount = matrix.tools.length - helmCount
  if (helmCount > 0 && externalCount > 0) {
    return 'Mixed'
  }
  if (helmCount > 0) {
    return 'Helm'
  }
  return 'Deployment'
}

const rowClassName = 'border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm'


const setupBreakdownSummary = (matrix: CompatibilityMatrix): string => {
  const labels: string[] = []
  const sampleTools = ['GitLab', 'Argo CD', 'Prometheus', 'Grafana', 'OpenTelemetry']

  sampleTools.forEach((toolName) => {
    const tool = matrix.tools.find((item) => item.name.toLowerCase().includes(toolName.toLowerCase().replace(' ', '')) || item.name.toLowerCase().includes(toolName.toLowerCase()))
    if (!tool) return
    const mode = tool.helmVersion && tool.helmVersion !== '-' && tool.helmVersion.toLowerCase() !== 'external' ? 'Helm' : 'Deployment'
    labels.push(`${toolName}: ${mode}`)
  })

  return labels.join(' · ')
}

const backingVersion = (matrix: CompatibilityMatrix, keyword: string): string => {
  const value = toolVersion(matrix, keyword)
  return value === '-' ? 'N/A' : value
}


const parseK8sMinor = (value: string): number | null => {
  const match = value.trim().match(/^(?:v)?(\d+)\.(\d+)/i)
  if (!match) return null
  return Number(match[1]) * 1000 + Number(match[2])
}

const isK8sInRange = (version: string, range: string): boolean => {
  const v = parseK8sMinor(version)
  if (v === null) return false

  const [minPart, maxPart] = range.split('-').map((part) => part.trim())
  const min = parseK8sMinor(minPart)
  const max = parseK8sMinor(maxPart ?? minPart)
  if (min === null || max === null) return false

  return v >= min && v <= max
}

export function StackVersionPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [search, setSearch] = useState('')
  const [validationOpen, setValidationOpen] = useState(false)
  const [validating, setValidating] = useState(false)
  const [selectedStackId, setSelectedStackId] = useState<string | null>(null)
  const [validationResult, setValidationResult] = useState<CompatibilityValidationResult | null>(null)
  const [validatedK8sVersion, setValidatedK8sVersion] = useState<string | null>(null)

  const { data: matrixData } = useCompatibilityMatrix()
  const { data: stacksData, isLoading: stacksLoading } = useStacks()
  const matrix = Array.isArray(matrixData) ? matrixData : []
  const stacks = stacksData?.items ?? []
  const validateMutation = useValidateCompatibility()
  const clusterVersionMutation = useClusterK8sVersion()

  const q = search.trim().toLowerCase()
  const gitlabRows = useMemo(() => {
    const rows = matrix.filter(
      (item) =>
        item.name.toLowerCase().includes('gitlab') &&
        (!q || item.name.toLowerCase().includes(q) || item.tools.some((it) => it.name.toLowerCase().includes(q) || it.appVersion.toLowerCase().includes(q)))
    )

    if (!validatedK8sVersion) {
      return rows
    }

    return [...rows].sort((a, b) => {
      const aMatch = isK8sInRange(validatedK8sVersion, a.k8sRange) ? 1 : 0
      const bMatch = isK8sInRange(validatedK8sVersion, b.k8sRange) ? 1 : 0
      return bMatch - aMatch
    })
  }, [matrix, q, validatedK8sVersion])

  const selectedStack = useMemo<Stack | null>(
    () => stacks.find((stack) => stack.id === selectedStackId) ?? null,
    [selectedStackId, stacks]
  )

  const openValidateModal = () => {
    setValidationOpen(true)
    setValidating(false)
    setSelectedStackId(null)
    setValidationResult(null)
    setValidatedK8sVersion(null)
  }

  const handleValidateStack = (stack: Stack) => {
    setSelectedStackId(stack.id)
    setValidating(true)

    clusterVersionMutation.mutate(stack.clusterId, {
      onSuccess: (version) => {
        if (version) {
          setValidatedK8sVersion(version)
        }
      },
    })

    validateMutation.mutate(stack.id, {
      onSuccess: (result) => {
        setValidationResult(result)
        setValidating(false)
      },
      onError: () => setValidating(false),
    })
  }

  const validationState = validationResult?.overall.state ?? (validationResult?.compatible ? 'pass' : 'fail')
  const validationBadge = VALIDATION_BADGE[validationState]

  return (
    <div>
      <Breadcrumb items={[{ label: t('sidebar.stackVersion', 'Stack Version') }]} />

      <div className="mb-7 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(34,197,94,0.15)] text-[#4ade80]">
            <Layers size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">{t('stackVersionPage.title', 'Stack Version')}</h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">{t('stackVersionPage.description', 'Manage compatibility based on validated version combinations.')}</p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={openValidateModal} type="button">
          <ShieldCheck size={15} />
          {t('stackVersionPage.actions.validateCurrentStack', 'Validate Current Stack')}
        </Button>
      </div>

      <div className="mb-5 rounded-lg border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.08)] px-4 py-3 text-sm text-[var(--color-text-primary)]">
        {t('stackVersionPage.notice', 'Only validated version combinations are shown. Unverified combinations will display warnings.') } {t('stackVersionPage.noticePostgres', 'Postgres is shown as N/A when the matrix does not define a backing DB version.') }
      </div>

      {validatedK8sVersion && (
        <div className="mb-3 rounded-md border border-[rgba(34,197,94,0.25)] bg-[rgba(34,197,94,0.08)] px-3 py-2 text-xs text-[var(--color-text-primary)]">
          {t('stackVersionPage.validation.k8sContext', 'Updated by selected stack Kubernetes version')}: <span className="font-semibold">{validatedK8sVersion}</span>
        </div>
      )}

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3">
          <span className="text-sm font-bold text-[var(--color-text-primary)]">{t('stackVersionPage.verifiedCombinations', 'Verified Combinations')}</span>
          <div className="relative">
            <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('stackVersionPage.searchPlaceholder', 'Search stacks or tools...')}
              className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
            />
          </div>
        </div>
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              {[
                t('stackVersionPage.table.name', 'Matrix'),
                t('stackVersionPage.table.k8sRange', 'K8s Range'),
                'MinIO',
                'Postgres',
                'Setup',
                t('stackVersionPage.table.gitlab', 'GitLab'),
                t('stackVersionPage.table.argocd', 'Argo CD'),
                t('stackVersionPage.table.prometheus', 'Prometheus'),
                t('stackVersionPage.table.grafana', 'Grafana'),
                t('stackVersionPage.table.opentelemetry', 'OpenTelemetry'),
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
                  <td className={cn(rowClassName, 'font-semibold text-[var(--color-text-primary)]')}>{item.name}</td>
                  <td className={cn(rowClassName, 'font-mono text-[13px] text-[var(--color-text-secondary)]')}>
                    <div className="flex items-center gap-1.5">
                      <span>{item.k8sRange}</span>
                      {validatedK8sVersion && (
                        <span className={cn('rounded px-1.5 py-[1px] text-[10px] font-semibold', isK8sInRange(validatedK8sVersion, item.k8sRange) ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]' : 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]')}>
                          {isK8sInRange(validatedK8sVersion, item.k8sRange) ? t('stackVersionPage.validation.match', 'Match') : t('stackVersionPage.validation.outOfRange', 'Out of range')}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{backingVersion(item, 'minio')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{backingVersion(item, 'postgres')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>
                    <div className="flex flex-col gap-0.5">
                      <span>{matrixSetupType(item)}</span>
                      <span className="text-[10px] text-[var(--color-text-muted)]">{setupBreakdownSummary(item)}</span>
                    </div>
                  </td>
                  <td className={cn(rowClassName, 'font-semibold text-[var(--color-text-primary)]')}>{toolVersion(item, 'gitlab')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'argo')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'prometheus')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'grafana')}</td>
                  <td className={cn(rowClassName, 'text-[var(--color-text-secondary)]')}>{toolVersion(item, 'opentelemetry')}</td>
                  <td className={rowClassName}>
                    <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', badge.className)}>{t(badge.key, badge.defaultLabel)}</span>
                    {recommended && (
                      <span className="ml-1.5 rounded-md bg-[rgba(139,92,246,0.15)] px-[7px] py-[3px] text-[11px] font-semibold text-[#c4b5fd]">{t('stackVersionPage.recommended', 'Recommended')}</span>
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
          setSelectedStackId(null)
          setValidationResult(null)
    setValidatedK8sVersion(null)
          setValidating(false)
        }}
        title={t('stackVersionPage.validation.title', 'Compatibility Validation Result')}
      >
        <div className="mb-4">
          <p className="mb-2 text-xs text-[var(--color-text-secondary)]">{t('stackVersionPage.validation.selectStack', 'Select a stack to validate')}</p>
          <div className="max-h-[180px] overflow-auto rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] p-2">
            {stacksLoading && <p className="m-0 px-2 py-1 text-xs text-[var(--color-text-secondary)]">{t('common.loading', 'Loading...')}</p>}
            {!stacksLoading && stacks.length === 0 && (
              <p className="m-0 px-2 py-1 text-xs text-[var(--color-text-secondary)]">{t('stackVersionPage.validation.noStacks', 'No stacks found')}</p>
            )}
            {!stacksLoading && stacks.map((stack) => (
              <button
                key={stack.id}
                type="button"
                onClick={() => handleValidateStack(stack)}
                className={cn(
                  'mb-1 flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-xs last:mb-0',
                  selectedStackId === stack.id
                    ? 'bg-[rgba(99,102,241,0.2)] text-[#c4b5fd]'
                    : 'bg-transparent text-[var(--color-text-primary)] hover:bg-[rgba(255,255,255,0.06)]'
                )}
              >
                <span>{stack.name}</span>
                <span className="text-[10px] text-[var(--color-text-secondary)]">{stack.templateName}</span>
              </button>
            ))}
          </div>
        </div>

        {validating && (
          <div className="py-8 text-center text-[var(--color-text-secondary)]">
            <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-[3px] border-[rgba(255,255,255,0.1)] border-t-[#a5b4fc]" />
            {t('stackVersionPage.validation.running', 'Validating...')}
          </div>
        )}
        {!validating && validationResult && (
          <div className="flex flex-col gap-4">
            {selectedStack && (
              <p className="m-0 text-xs text-[var(--color-text-secondary)]">
                {t('stackVersionPage.validation.targetStack', 'Target stack')}: <span className="font-semibold text-[var(--color-text-primary)]">{selectedStack.name}</span>
              </p>
            )}

            <div
              className={cn(
                'flex items-center gap-2.5 rounded-lg border px-4 py-3',
                validationBadge.container
              )}
            >
              {validationState === 'warn' ? (
                <AlertTriangle size={20} color={validationBadge.icon} />
              ) : validationState === 'fail' ? (
                <XCircle size={20} color={validationBadge.icon} />
              ) : (
                <ShieldCheck size={20} color={validationBadge.icon} />
              )}
              <div className="flex flex-col gap-0.5">
                <span className={cn('text-sm font-bold', validationBadge.text)}>
                  {t(validationBadge.key, validationBadge.label)}
                </span>
                <span className="text-xs text-[var(--color-text-secondary)]">
                  {t('stackVersionPage.validation.score', 'Score')}: {validationResult.overall.score}
                </span>
              </div>
            </div>

            {validationResult.issues.length > 0 && (
              <ul className="m-0 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-4 py-3 text-xs text-[var(--color-text-secondary)]">
                {validationResult.issues.map((issue, index) => (
                  <li key={`${issue.code ?? 'issue'}-${index}`} className="mb-1 last:mb-0">
                    <span className={cn('mr-1 font-semibold', issue.severity === 'error' ? 'text-[#ef4444]' : 'text-[#f59e0b]')}>
                      [{issue.severity.toUpperCase()}]
                    </span>
                    {issue.tool}: {issue.message}
                  </li>
                ))}
              </ul>
            )}

            <p className="m-0 text-xs text-[var(--color-text-secondary)]">{t('stackVersionPage.validation.checkedAt', 'Checked at')}: {formatDateTime(validationResult.checkedAt, locale)}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}
