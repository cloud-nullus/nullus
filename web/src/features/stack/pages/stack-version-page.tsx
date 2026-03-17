import { Layers, ShieldCheck } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useCompatibilityMatrix, useValidateCompatibility } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import type { CompatibilityMatrix, CompatibilityValidationResult } from '../api/stack-api'
import { cn } from '../../../lib/utils'
import { useState } from 'react'

const MOCK_COMPATIBILITY_MATRIX: CompatibilityMatrix[] = [
  {
    id: 'gitlab-allinone-v1',
    name: 'GitLab All-in-One',
    status: 'verified',
    k8sRange: '1.27-1.32',
    tools: [
      { name: 'GitLab CE', helmVersion: '8.7.2', appVersion: '17.7.2' },
      { name: 'GitLab CI', helmVersion: '8.7.2', appVersion: '17.7.2' },
      { name: 'GitLab Registry', helmVersion: '8.7.2', appVersion: '17.7.2' },
      { name: 'Argo CD', helmVersion: '7.7.2', appVersion: '2.13.2' },
      { name: 'Prometheus', helmVersion: '67.0.0', appVersion: '3.1.0' },
      { name: 'Grafana', helmVersion: '8.5.0', appVersion: '11.4.0' },
      { name: 'MinIO', helmVersion: '5.3.0', appVersion: '2024.11.7' },
    ],
  },
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    status: 'verified',
    k8sRange: '1.27-1.32',
    tools: [
      { name: 'GitLab CE', helmVersion: '8.7.2', appVersion: '17.7.2' },
      { name: 'GitLab CI', helmVersion: '8.7.2', appVersion: '17.7.2' },
      { name: 'Harbor', helmVersion: '1.14.0', appVersion: '2.11.0' },
      { name: 'Argo CD', helmVersion: '7.7.2', appVersion: '2.13.2' },
      { name: 'Prometheus', helmVersion: '67.0.0', appVersion: '3.1.0' },
      { name: 'Grafana', helmVersion: '8.5.0', appVersion: '11.4.0' },
      { name: 'MinIO', helmVersion: '5.3.0', appVersion: '2024.11.7' },
    ],
  },
  {
    id: 'github-argocd-v1',
    name: 'GitHub + Argo CD',
    status: 'untested',
    k8sRange: '1.27-1.32',
    tools: [
      { name: 'GitHub', helmVersion: 'external', appVersion: 'external' },
      { name: 'GitHub Actions', helmVersion: 'external', appVersion: 'external' },
      { name: 'Harbor', helmVersion: '1.14.0', appVersion: '2.11.0' },
      { name: 'Argo CD', helmVersion: '7.7.2', appVersion: '2.13.2' },
      { name: 'Prometheus', helmVersion: '67.0.0', appVersion: '3.1.0' },
      { name: 'Grafana', helmVersion: '8.5.0', appVersion: '11.4.0' },
      { name: 'MinIO', helmVersion: '5.3.0', appVersion: '2024.11.7' },
    ],
  },
]

const STATUS_BADGE: Record<string, { className: string; label: string }> = {
  verified: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Verified' },
  untested: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Partial' },
  unsupported: { className: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Not Supported' },
}

const toolVersion = (matrix: CompatibilityMatrix, keyword: string): string => {
  const lower = keyword.toLowerCase()
  const tool = matrix.tools.find((item) => item.name.toLowerCase().includes(lower))
  return tool ? tool.appVersion : '-'
}

const rowClassName = 'border-t border-[var(--color-border-default)] px-[14px] py-3 text-sm'

export function StackVersionPage() {
  const [validationOpen, setValidationOpen] = useState(false)
  const [validating, setValidating] = useState(false)
  const [validationResult, setValidationResult] = useState<CompatibilityValidationResult | null>(null)
  const { data: matrixData } = useCompatibilityMatrix()
  const matrix = Array.isArray(matrixData) && matrixData.length > 0 ? matrixData : MOCK_COMPATIBILITY_MATRIX
  const validateMutation = useValidateCompatibility('current')

  const gitlabRows = matrix.filter((item) => item.name.toLowerCase().includes('gitlab'))
  const githubRows = matrix.filter((item) => item.name.toLowerCase().includes('github'))

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

      <div className="mb-7 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(34,197,94,0.15)] text-[#4ade80]">
            <Layers size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">Stack Version</h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">테스트 완료된 버전 조합을 기반으로 호환성을 관리합니다.</p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={handleValidate}>
          <ShieldCheck size={15} />
          Validate Current Stack
        </Button>
      </div>

      <div className="mb-5 rounded-lg border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.08)] px-4 py-3 text-sm text-[var(--color-text-primary)]">
        테스트 완료된 버전 조합만 표시됩니다. 미검증 조합 사용 시 경고가 표시됩니다.
      </div>

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="border-b border-[var(--color-border-default)] px-5 py-4 text-sm font-bold text-[var(--color-text-primary)]">
          Verified Combinations
        </div>
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              {['GitLab', 'Argo CD', 'Prometheus', 'Grafana', 'OpenTelemetry', 'K8s', 'Status'].map((header) => (
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
                    <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', badge.className)}>{badge.label}</span>
                    {recommended && (
                      <span className="ml-1.5 rounded-md bg-[rgba(139,92,246,0.15)] px-[7px] py-[3px] text-[11px] font-semibold text-[#c4b5fd]">Recommended</span>
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
        title="호환성 검증 결과"
      >
        {validating && (
          <div className="py-8 text-center text-[var(--color-text-secondary)]">
            <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-[3px] border-[rgba(255,255,255,0.1)] border-t-[#a5b4fc]" />
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
              <span className={cn('text-sm font-bold', validationResult.compatible ? 'text-[#22c55e]' : 'text-[#ef4444]')}>
                {validationResult.compatible ? '호환성 검증 통과' : '호환성 문제 발견'}
              </span>
            </div>

            <p className="m-0 text-xs text-[var(--color-text-secondary)]">검증 시각: {new Date(validationResult.checkedAt).toLocaleString('ko-KR')}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}
