import { useMemo, useState } from 'react'
import { Check, Plus, Wrench } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
import type { Stack } from '../../../types'
import { useAddTools, useStacks } from '../api/stack-api'

interface ToolOption {
  id: string
  label: string
  description: string
}

interface ToolSlot {
  id: string
  label: string
  options: ToolOption[]
}

interface ToolCategory {
  id: string
  label: string
  description: string
  slots: ToolSlot[]
}

interface InstalledTool {
  category: string
  tool: string
  version: string
}

type StackWithTools = Stack & {
  tools?: InstalledTool[]
}

const TOOL_CATEGORIES: ToolCategory[] = [
  {
    id: 'artifacts',
    label: 'Artifacts',
    description: 'Package/Source/Container/Storage 도구를 추가합니다.',
    slots: [
      {
        id: 'package_registry',
        label: 'Package Registry',
        options: [
          { id: 'gitlab', label: 'GitLab Package Registry', description: 'GitLab 내장 패키지 레지스트리' },
          { id: 'nexus', label: 'Nexus Repository', description: '범용 아티팩트 저장소' },
          { id: 'jfrog', label: 'JFrog Artifactory', description: '엔터프라이즈급 아티팩트 관리' },
        ],
      },
      {
        id: 'source_repository',
        label: 'Source Repository',
        options: [
          { id: 'gitlab', label: 'GitLab', description: 'GitLab 소스 코드 관리' },
          { id: 'github', label: 'GitHub', description: 'GitHub 소스 코드 관리' },
          { id: 'gitea', label: 'Gitea', description: '경량 셀프호스팅 Git 서비스' },
        ],
      },
      {
        id: 'container_registry',
        label: 'Container Registry',
        options: [
          { id: 'gitlab-registry', label: 'GitLab Container Registry', description: 'GitLab 내장 컨테이너 레지스트리' },
          { id: 'harbor', label: 'Harbor', description: '엔터프라이즈 컨테이너 레지스트리' },
          { id: 'docker-hub', label: 'Docker Hub', description: 'Docker 공식 레지스트리' },
        ],
      },
      {
        id: 'storage_backend',
        label: 'Storage Backend',
        options: [
          { id: 'minio', label: 'MinIO', description: 'S3 호환 오브젝트 스토리지' },
          { id: 's3', label: 'AWS S3', description: 'Amazon S3 오브젝트 스토리지' },
          { id: 'gcs', label: 'Google Cloud Storage', description: 'GCP 오브젝트 스토리지' },
        ],
      },
    ],
  },
  {
    id: 'pipeline',
    label: 'Pipeline',
    description: 'CI/CD 파이프라인 도구를 추가합니다.',
    slots: [
      {
        id: 'ci_platform',
        label: 'CI Platform',
        options: [
          { id: 'gitlab-ci', label: 'GitLab CI/CD', description: 'GitLab 내장 CI/CD 파이프라인' },
          { id: 'github-actions', label: 'GitHub Actions', description: 'GitHub 워크플로우 기반 CI/CD' },
          { id: 'jenkins', label: 'Jenkins', description: '전통적인 오픈소스 CI 서버' },
        ],
      },
      {
        id: 'cd_tool',
        label: 'CD Tool',
        options: [
          { id: 'argocd', label: 'ArgoCD', description: 'GitOps 기반 쿠버네티스 CD' },
          { id: 'flux', label: 'Flux CD', description: 'GitOps 툴킷' },
          { id: 'spinnaker', label: 'Spinnaker', description: '멀티 클라우드 CD 플랫폼' },
        ],
      },
    ],
  },
  {
    id: 'observability',
    label: 'Observability',
    description: '메트릭/시각화/트레이싱 도구를 추가합니다.',
    slots: [
      {
        id: 'metrics_collection',
        label: 'Metrics Collection',
        options: [
          { id: 'prometheus', label: 'Prometheus', description: '시계열 메트릭 수집' },
          { id: 'thanos', label: 'Thanos', description: '장기 보관 및 글로벌 메트릭 집계' },
          { id: 'victoriametrics', label: 'VictoriaMetrics', description: '고성능 시계열 데이터베이스' },
        ],
      },
      {
        id: 'visualization',
        label: 'Visualization',
        options: [
          { id: 'grafana', label: 'Grafana', description: '오픈소스 메트릭 시각화' },
          { id: 'opensearch-dashboards', label: 'OpenSearch Dashboards', description: 'OpenSearch 시각화 대시보드' },
        ],
      },
      {
        id: 'trace_layer',
        label: 'Trace Layer',
        options: [
          { id: 'tempo', label: 'Tempo', description: '분산 추적 백엔드' },
          { id: 'jaeger', label: 'Jaeger', description: '분산 추적 및 트레이스 분석' },
        ],
      },
    ],
  },
  {
    id: 'logging',
    label: 'Logging',
    description: '로그 검색/분석 도구를 추가합니다.',
    slots: [
      {
        id: 'log_search',
        label: 'Log Search',
        options: [
          { id: 'opensearch', label: 'OpenSearch', description: 'Elasticsearch 호환 검색/분석' },
          { id: 'loki', label: 'Grafana Loki', description: 'Prometheus 스타일 로그 집계' },
        ],
      },
    ],
  },
]


const STEP_TABS = [
  { id: 0, label: '1. Category Selection' },
  { id: 1, label: '2. Tool Configuration' },
  { id: 2, label: '3. Review & Deploy' },
]

const SLOT_TO_CATEGORY = TOOL_CATEGORIES.reduce<Record<string, string>>((acc, category) => {
  category.slots.forEach((slot) => {
    acc[slot.id] = category.id
  })
  return acc
}, {})

function firstSelectableOption(slot: ToolSlot, installedToolNames: Set<string>) {
  return slot.options.find((opt) => !installedToolNames.has(opt.id)) ?? slot.options[0]
}

function ToolSelector({
  label,
  options,
  value,
  installedToolNames,
  installedLabel,
  onChange,
}: {
  label: string
  options: ToolOption[]
  value: { tool: string; version: string }
  installedToolNames: Set<string>
  installedLabel: string
  onChange: (v: { tool: string; version: string }) => void
}) {
  return (
    <div className="mb-5">
      <div className="mb-2.5 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
        {label}
      </div>
      <div className="flex flex-col gap-2">
        {options.map((opt) => {
          const selected = value.tool === opt.id
          const installed = installedToolNames.has(opt.id)
          return (
            <button
              key={opt.id}
              type="button"
              disabled={installed}
              onClick={() => onChange({ tool: opt.id, version: 'latest' })}
              className={cn(
                'flex w-full items-center gap-3 rounded-lg border px-[14px] py-3 text-left transition-all duration-150',
                installed && 'cursor-not-allowed opacity-55',
                !installed && 'cursor-pointer',
                selected
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                  : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
              )}
            >
              <div
                className={cn(
                  'flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2',
                  selected
                    ? 'border-[#6366f1] bg-[#6366f1]'
                    : 'border-[var(--color-border-hover)] bg-transparent'
                )}
              >
                {selected && <div className="h-1.5 w-1.5 rounded-full bg-white" />}
              </div>
              <div className="flex min-w-0 flex-1 items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                    {opt.label}
                  </div>
                  <div className="text-xs text-[var(--color-text-secondary)]">{opt.description}</div>
                </div>
                {installed && (
                  <span className="shrink-0 rounded bg-[rgba(148,163,184,0.2)] px-2 py-0.5 text-[11px] font-semibold text-[#cbd5e1]">
                    {installedLabel}
                  </span>
                )}
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function StackAddToolsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id: stackId = '' } = useParams()
  const [step, setStep] = useState(0)
  const [selectedCategories, setSelectedCategories] = useState<string[]>([])
  const [selectedTools, setSelectedTools] = useState<Record<string, { tool: string; version: string }>>({})

  const addTools = useAddTools()
  const { data: stackListData, isLoading } = useStacks()

  const stack = useMemo(() => {
    const apiStack = (stackListData?.items as StackWithTools[] | undefined)?.find((item) => item.id === stackId)
    return apiStack ?? null
  }, [stackListData?.items, stackId])

  const installedTools = (stack?.tools ?? []) as InstalledTool[]

  const installedToolsByCategory = useMemo(() => {
    return installedTools.reduce<Record<string, Set<string>>>((acc, tool) => {
      if (!acc[tool.category]) {
        acc[tool.category] = new Set<string>()
      }
      acc[tool.category].add(tool.tool)
      return acc
    }, {})
  }, [installedTools])

  const installedCategorySet = useMemo(() => {
    return new Set(
      Object.keys(installedToolsByCategory)
        .map((slotKey) => SLOT_TO_CATEGORY[slotKey])
        .filter((category): category is string => Boolean(category))
    )
  }, [installedToolsByCategory])

  const reviewItems = useMemo(() => {
    const items: Array<{ category: string; categoryLabel: string; slotLabel: string; tool: string; toolLabel: string; version: string }> = []
    TOOL_CATEGORIES.forEach((category) => {
      if (!selectedCategories.includes(category.id)) return
      category.slots.forEach((slot) => {
        const selected = selectedTools[slot.id]
        if (!selected) return
        const installedForSlot = installedToolsByCategory[slot.id] ?? new Set<string>()
        if (installedForSlot.has(selected.tool)) return
        items.push({
          category: slot.id,
          categoryLabel: t(`stackAddTools.categories.${category.id}.label`, category.label),
          slotLabel: t(`stackAddTools.slots.${slot.id}.label`, slot.label),
          tool: selected.tool,
          toolLabel: t(`stackAddTools.tools.${selected.tool}.label`, selected.tool),
          version: selected.version,
        })
      })
    })
    return items
  }, [installedToolsByCategory, selectedCategories, selectedTools, t])

  const initializeSelections = (category: ToolCategory) => {
    setSelectedTools((prev) => {
      const next = { ...prev }
      category.slots.forEach((slot) => {
        if (next[slot.id]) return
        const installedForSlot = installedToolsByCategory[slot.id] ?? new Set<string>()
        const fallback = firstSelectableOption(slot, installedForSlot)
        next[slot.id] = { tool: fallback.id, version: 'latest' }
      })
      return next
    })
  }

  const handleToggleCategory = (category: ToolCategory) => {
    if (installedCategorySet.has(category.id)) return
    setSelectedCategories((prev) => {
      if (prev.includes(category.id)) {
        return prev.filter((item) => item !== category.id)
      }
      initializeSelections(category)
      return [...prev, category.id]
    })
  }

  const handleAddTools = async () => {
    if (!stackId || reviewItems.length === 0) {
      toast.error(t('stackAddTools.toast.selectTools', 'Please select tools to add.'))
      return
    }

    try {
      await addTools.mutateAsync({
        stackId,
        tools: reviewItems.map((item) => ({
          category: item.category,
          tool: item.tool,
          version: item.version,
        })),
      })
      toast.success(t('stackAddTools.toast.deployStarted', 'Tool addition deployment has started.'))
      navigate('/stack/list')
    } catch {
      toast.error(t('stackAddTools.toast.deployFailed', 'Failed to add tools. Please try again later.'))
    }
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: t('stackAddTools.breadcrumb.stackList', 'Stack List'), path: '/stack/list' },
        { label: stack?.name ?? 'Stack', path: '/stack/list' },
        { label: t('stackAddTools.breadcrumb.current', 'Add Tools') },
      ]} />

      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <Wrench size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">{t('stackAddTools.title', 'Add Tools')}</h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackAddTools.description', 'Safely add required tools to an existing stack.')}
            </p>
          </div>
        </div>
        <Button variant="outline" size="md" type="button" onClick={() => navigate('/stack/list')}>
          {t('stackAddTools.actions.backToList', 'Back to List')}
        </Button>
      </div>

      <div className="mb-5 flex gap-0 border-b border-[var(--color-border-default)]">
        {STEP_TABS.map((tab) => {
          const isActive = step === tab.id
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setStep(tab.id)}
              className={cn(
                '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-[18px] py-2.5 text-sm transition-all duration-150',
                isActive
                  ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                  : 'font-normal text-[var(--color-text-secondary)]'
              )}
            >
              {t(`stackAddTools.steps.${tab.id}`, tab.label)}
            </button>
          )
        })}
      </div>

      <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
        {isLoading && <div className="text-sm text-[var(--color-text-secondary)]">{t('stackAddTools.loading', 'Loading stack information...')}</div>}

        {!isLoading && !stack && (
          <div className="rounded-lg border border-[rgba(239,68,68,0.4)] bg-[rgba(239,68,68,0.08)] p-4 text-sm text-[#fca5a5]">
            {t('stackAddTools.notFound', 'Target stack not found.')}
          </div>
        )}

        {!isLoading && stack && step === 0 && (
          <div>
            <p className="mb-4 mt-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackAddTools.step0.description', 'Check installed status and select categories that are not added yet.')}
            </p>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-3">
              {TOOL_CATEGORIES.map((category) => {
                const selected = selectedCategories.includes(category.id)
                const installed = installedCategorySet.has(category.id)
                return (
                  <button
                    key={category.id}
                    type="button"
                    disabled={installed}
                    onClick={() => handleToggleCategory(category)}
                    className={cn(
                      'flex h-full cursor-pointer flex-col items-start gap-2 rounded-lg border p-4 text-left transition-all duration-150',
                      selected && 'border-[rgba(99,102,241,0.45)] bg-[rgba(99,102,241,0.08)]',
                      !selected && !installed && 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]',
                      installed && 'cursor-not-allowed border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.08)]'
                    )}
                  >
                    <div className="flex w-full items-center justify-between gap-3">
                      <span className="text-sm font-semibold text-[var(--color-text-primary)]">
                        {t(`stackAddTools.categories.${category.id}.label`, category.label)}
                      </span>
                      <span className="flex items-center gap-1.5">
                        {installed && (
                          <span className="inline-flex items-center gap-1 rounded bg-[rgba(34,197,94,0.18)] px-2 py-0.5 text-[11px] font-semibold text-[#86efac]">
                            <Check size={11} /> {t('stackAddTools.badge.installed', 'Installed')}
                          </span>
                        )}
                        {!installed && selected && (
                          <span className="inline-flex items-center gap-1 rounded bg-[rgba(99,102,241,0.2)] px-2 py-0.5 text-[11px] font-semibold text-[#a5b4fc]">
                            <Plus size={11} /> {t('stackAddTools.badge.selected', 'Selected')}
                          </span>
                        )}
                      </span>
                    </div>
                    <span className="text-xs text-[var(--color-text-secondary)]">
                      {t(`stackAddTools.categories.${category.id}.description`, category.description)}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        )}

        {!isLoading && stack && step === 1 && (
          <div>
            {selectedCategories.length === 0 ? (
              <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4 text-sm text-[var(--color-text-secondary)]">
                {t('stackAddTools.step1.selectCategoryFirst', 'Please select categories first in Step 1.')}
              </div>
            ) : (
              selectedCategories.map((categoryId) => {
                const category = TOOL_CATEGORIES.find((item) => item.id === categoryId)
                if (!category) return null
                return (
                  <div key={category.id} className="mb-6 last:mb-0">
                    <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
                      {t(`stackAddTools.categories.${category.id}.label`, category.label)}
                    </h3>
                    {category.slots.map((slot) => (
                      <ToolSelector
                        key={slot.id}
                        label={t(`stackAddTools.slots.${slot.id}.label`, slot.label)}
                        installedLabel={t('stackAddTools.badge.installed', 'Installed')}
                        options={slot.options.map((option) => ({
                          ...option,
                          label: t(`stackAddTools.tools.${option.id}.label`, option.label),
                          description: t(`stackAddTools.tools.${option.id}.description`, option.description),
                        }))}
                        value={selectedTools[slot.id] ?? { tool: slot.options[0].id, version: 'latest' }}
                        installedToolNames={installedToolsByCategory[slot.id] ?? new Set<string>()}
                        onChange={(value) => {
                          setSelectedTools((prev) => ({ ...prev, [slot.id]: value }))
                        }}
                      />
                    ))}
                  </div>
                )
              })
            )}
          </div>
        )}

        {!isLoading && stack && step === 2 && (
          <div>
            <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">{t('stackAddTools.step2.title', 'Review & Deploy')}</h3>
            {reviewItems.length === 0 ? (
              <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4 text-sm text-[var(--color-text-secondary)]">
                {t('stackAddTools.step2.noItems', 'No new tools to add. Please review your category/tool selection.')}
              </div>
            ) : (
              <div className="overflow-hidden rounded-lg border border-[var(--color-border-default)]">
                <table className="w-full border-collapse text-sm">
                  <thead className="bg-[rgba(255,255,255,0.03)] text-left text-[11px] uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                    <tr>
                      <th className="px-4 py-2.5">{t('stackAddTools.table.category', 'Category')}</th>
                      <th className="px-4 py-2.5">{t('stackAddTools.table.slot', 'Slot')}</th>
                      <th className="px-4 py-2.5">{t('stackAddTools.table.tool', 'Tool')}</th>
                      <th className="px-4 py-2.5">{t('stackAddTools.table.version', 'Version')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {reviewItems.map((item) => (
                      <tr key={`${item.category}-${item.tool}`} className="border-t border-[var(--color-border-default)]">
                        <td className="px-4 py-2.5 text-[var(--color-text-primary)]">{item.categoryLabel}</td>
                        <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{item.slotLabel}</td>
                        <td className="px-4 py-2.5 text-[var(--color-text-primary)]">{item.toolLabel}</td>
                        <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{item.version}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="mt-4 flex items-center justify-between gap-2">
        <Button
          variant="outline"
          size="md"
          type="button"
          onClick={() => setStep((prev) => Math.max(0, prev - 1))}
          disabled={step === 0}
        >
          {t('stackAddTools.actions.previous', 'Previous')}
        </Button>
        {step < 2 ? (
          <Button
            variant="primary"
            size="md"
            type="button"
            onClick={() => setStep((prev) => Math.min(2, prev + 1))}
            disabled={(step === 0 && selectedCategories.length === 0) || (step === 1 && selectedCategories.length === 0)}
          >
            {t('stackAddTools.actions.next', 'Next')}
          </Button>
        ) : (
          <Button
            variant="primary"
            size="md"
            type="button"
            loading={addTools.isPending}
            disabled={reviewItems.length === 0}
            onClick={handleAddTools}
          >
            {t('stackAddTools.actions.confirmDeploy', 'Confirm & Deploy')}
          </Button>
        )}
      </div>
    </div>
  )
}
