import { useState, Fragment } from 'react'
import { Layers, ChevronDown, ChevronRight, ShieldCheck } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useCompatibilityMatrix, useValidateCompatibility } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import type { CompatibilityMatrix, CompatibilityValidationResult } from '../api/stack-api'
import { cn } from '../../../lib/utils'

const STATUS_BADGE: Record<string, { className: string; label: string }> = {
  verified: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Verified' },
  untested: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Untested' },
}

export function StackVersionPage() {
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [validationOpen, setValidationOpen] = useState(false)
  const [validating, setValidating] = useState(false)
  const [validationResult, setValidationResult] = useState<CompatibilityValidationResult | null>(null)
  const { data: matrixData } = useCompatibilityMatrix()
  const matrix: CompatibilityMatrix[] = Array.isArray(matrixData) ? matrixData : []
  const validateMutation = useValidateCompatibility('current')

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
      <Breadcrumb items={[{ label: 'Stack Version' }]} />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(34,197,94,0.15)] text-[#4ade80]"
          >
            <Layers size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Stack Version
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              스택 버전 호환성 매트릭스
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={handleValidate}>
          <ShieldCheck size={15} />
          Validate Current Stack
        </Button>
      </div>

      {/* Table */}
      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              <th className="w-8 whitespace-nowrap px-[14px] py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]" />
              {['매트릭스 이름', '상태', 'K8s 호환 범위'].map((h) => (
                <th
                  key={h}
                  className="whitespace-nowrap px-[14px] py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {matrix.map((matrix) => {
              const isExpanded = expandedId === matrix.id
              const badge = STATUS_BADGE[matrix.status] ?? STATUS_BADGE.untested
              return (
                <Fragment key={matrix.id}>
                  <tr
                    className="cursor-pointer transition-colors duration-150 hover:bg-[rgba(255,255,255,0.02)]"
                    onClick={() => setExpandedId(isExpanded ? null : matrix.id)}
                  >
                    <td className="border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm text-[var(--color-text-secondary)]">
                      {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                    </td>
                    <td className="border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm text-[var(--color-text-primary)]">
                      <span className="font-semibold">{matrix.name}</span>
                    </td>
                    <td className="border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm text-[var(--color-text-primary)]">
                      <span
                        className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', badge.className)}
                      >
                        {badge.label}
                      </span>
                    </td>
                    <td className="border-t border-[var(--color-border-default)] px-[14px] py-3 font-mono text-[13px] text-[var(--color-text-secondary)]">
                      {matrix.k8sRange}
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr>
                      <td colSpan={4} className="border-t border-[var(--color-border-default)] p-0">
                        <div className="bg-[rgba(0,0,0,0.2)] px-5 py-4">
                          <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                            도구별 버전
                          </p>
                          <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2.5">
                            {matrix.tools.map((tool) => (
                              <div
                                key={tool.name}
                                className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-[14px] py-3"
                              >
                                <p className="mb-1.5 mt-0 text-[13px] font-bold text-[var(--color-text-primary)]">
                                  {tool.name}
                                </p>
                                <div className="flex gap-[14px] font-mono text-xs">
                                  <span>
                                    <span className="text-[var(--color-text-secondary)]">helm: </span>
                                    <span className="text-[#a5b4fc]">{tool.helmVersion}</span>
                                  </span>
                                  <span>
                                    <span className="text-[var(--color-text-secondary)]">app: </span>
                                    <span className="text-[#4ade80]">{tool.appVersion}</span>
                                  </span>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Validation Result Modal */}
      <Modal
        open={validationOpen}
        onClose={() => {
          setValidationOpen(false)
          setValidationResult(null)
        }}
        title="호환성 검증 결과"
      >
        {validating && (
          <div className="py-8 text-center text-[var(--color-text-secondary)]">
            <div
              className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-[3px] border-[rgba(255,255,255,0.1)] border-t-[#a5b4fc]"
            />
            검증 중...
          </div>
        )}
        {!validating && validationResult && (
          <div className="flex flex-col gap-4">
            <div
              className={cn(
                'flex items-center gap-2.5 rounded-lg border px-4 py-3',
                validationResult.compatible
                  ? 'border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.1)]'
                  : 'border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.1)]'
              )}
            >
              <ShieldCheck size={20} color={validationResult.compatible ? '#22c55e' : '#ef4444'} />
              <span
                className={cn(
                  'text-sm font-bold',
                  validationResult.compatible ? 'text-[#22c55e]' : 'text-[#ef4444]'
                )}
              >
                {validationResult.compatible ? '호환성 검증 통과' : '호환성 문제 발견'}
              </span>
            </div>

            {validationResult.issues.length > 0 && (
              <div>
                <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                  이슈
                </p>
                <div className="flex flex-col gap-2">
                  {validationResult.issues.map((issue) => (
                    <div
                      key={`${issue.tool}-${issue.message}`}
                      className={cn(
                        'rounded-lg border px-[14px] py-2.5',
                        issue.severity === 'error'
                          ? 'border-[rgba(239,68,68,0.25)] bg-[rgba(239,68,68,0.08)]'
                          : 'border-[rgba(245,158,11,0.25)] bg-[rgba(245,158,11,0.08)]'
                      )}
                    >
                      <span
                        className={cn(
                          'mr-2 text-xs font-bold',
                          issue.severity === 'error' ? 'text-[#f87171]' : 'text-[#fbbf24]'
                        )}
                      >
                        [{issue.tool}]
                      </span>
                      <span className="text-[13px] text-[var(--color-text-primary)]">{issue.message}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <p className="m-0 text-xs text-[var(--color-text-secondary)]">
              검증 시각: {new Date(validationResult.checkedAt).toLocaleString('ko-KR')}
            </p>
          </div>
        )}
      </Modal>
    </div>
  )
}
