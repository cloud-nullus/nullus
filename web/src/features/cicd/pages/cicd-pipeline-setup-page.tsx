import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  GitBranch,
  Loader2,
  Rocket,
  Server,
  Settings2,
  Zap,
} from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { NativeSelect } from '../../../components/ui/native-select'
import {
  useCicdTemplates,
  useCreatePipeline,
  useDeployPipeline,
  useProvisionPipeline,
} from '../api/cicd-api'
import { useClusters } from '../../admin/api/admin-api'
import { useStacks, useStackIntegrations } from '../../stack/api/stack-api'
import type { CicdTemplate } from '../api/cicd-api'
import type { AppType } from '../../../types'
import { cn } from '../../../lib/utils'

// ── Types ──────────────────────────────────────────────────────────────────

type ExecutionMode = 'stack_integrated' | 'emergency_direct'

interface IntegrationStatus {
  componentType: string
  provider: string
  health: string
  ready: boolean
}

// ── Constants ──────────────────────────────────────────────────────────────

const DEFAULT_TEMPLATES: CicdTemplate[] = [
  {
    id: 'web-backend',
    name: 'Backend API',
    description: 'REST API 백엔드 서비스 템플릿',
    appType: 'web-backend',
    stages: ['Build', 'Test', 'Security', 'Docker Build', 'ArgoCD Deploy'],
    createdBy: 'admin',
  },
  {
    id: 'web-frontend',
    name: 'Web Frontend',
    description: 'React/Next.js 웹 프론트엔드 앱 템플릿',
    appType: 'web-frontend',
    stages: ['Build', 'Test', 'Docker Build', 'ArgoCD Deploy'],
    createdBy: 'admin',
  },
  {
    id: 'batch-job',
    name: 'Batch Job',
    description: '배치 잡 템플릿',
    appType: 'batch-job',
    stages: ['Build', 'Test', 'Docker Build', 'CronJob Deploy'],
    createdBy: 'admin',
  },
]

const COMPONENT_LABELS: Record<string, string> = {
  code_repository: 'Code Repository',
  image_registry: 'Image Registry',
  ci_platform: 'CI Platform',
  cd_tool: 'CD Tool',
}

// ── Sub-components ─────────────────────────────────────────────────────────

function ModeCard({
  mode,
  selected,
  onClick,
}: {
  mode: ExecutionMode
  selected: boolean
  onClick: () => void
}) {
  const isIntegrated = mode === 'stack_integrated'

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'relative flex w-full cursor-pointer flex-col gap-2 rounded-xl border-2 p-5 text-left transition-all duration-150',
        selected
          ? isIntegrated
            ? 'border-[#6366f1] bg-[rgba(99,102,241,0.1)]'
            : 'border-[#f59e0b] bg-[rgba(245,158,11,0.08)]'
          : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] hover:border-[rgba(255,255,255,0.2)]',
      )}
    >
      <div className="flex items-center gap-2.5">
        <div
          className={cn(
            'flex h-9 w-9 items-center justify-center rounded-lg',
            selected
              ? isIntegrated
                ? 'bg-[rgba(99,102,241,0.2)] text-[#818cf8]'
                : 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]'
              : 'bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]',
          )}
        >
          {isIntegrated ? <GitBranch size={18} /> : <Zap size={18} />}
        </div>
        <div>
          <div
            className={cn(
              'text-sm font-bold',
              selected
                ? isIntegrated
                  ? 'text-[#a5b4fc]'
                  : 'text-[#fbbf24]'
                : 'text-[var(--color-text-primary)]',
            )}
          >
            {isIntegrated ? 'Stack Integrated' : 'Emergency Direct'}
          </div>
          <div className="text-[11px] text-[var(--color-text-secondary)]">
            {isIntegrated ? 'GitLab CI + ArgoCD GitOps' : 'Direct kubectl apply'}
          </div>
        </div>
        {selected && (
          <div className="ml-auto">
            <CheckCircle2
              size={18}
              className={isIntegrated ? 'text-[#6366f1]' : 'text-[#f59e0b]'}
            />
          </div>
        )}
      </div>
      <p className="m-0 text-[12px] leading-relaxed text-[var(--color-text-secondary)]">
        {isIntegrated
          ? 'Stack에 배포된 GitLab + ArgoCD를 연동해 GitOps 파이프라인을 구성합니다. 프로비저닝 후 git push만으로 자동 배포됩니다.'
          : 'Stack 없이 Nullus가 직접 git clone → docker build → kubectl apply를 수행합니다. 즉시 사용 가능하며 테스트/긴급 복구에 적합합니다.'}
      </p>
    </button>
  )
}

function IntegrationBadge({ status }: { status: IntegrationStatus }) {
  const ready = status.health === 'ready'
  return (
    <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2">
      <div className="flex items-center gap-2">
        <div
          className={cn(
            'h-2 w-2 rounded-full',
            ready ? 'bg-emerald-400' : 'bg-amber-400',
          )}
        />
        <span className="text-xs text-[var(--color-text-secondary)]">
          {COMPONENT_LABELS[status.componentType] ?? status.componentType}
        </span>
      </div>
      <span
        className={cn(
          'text-xs font-semibold',
          ready ? 'text-emerald-400' : 'text-amber-400',
        )}
      >
        {status.provider || '—'}{' '}
        <span className="font-normal opacity-60">{ready ? 'Ready' : 'Not ready'}</span>
      </span>
    </div>
  )
}

// ── Main Page ──────────────────────────────────────────────────────────────

export function CicdPipelineSetupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const selectedTemplateId = searchParams.get('template')

  // API hooks
  const { data: templatesData } = useCicdTemplates()
  const { data: clustersData } = useClusters()
  const { data: stacksData } = useStacks({ status: 'completed' })
  const createPipeline = useCreatePipeline()
  const deployPipeline = useDeployPipeline()
  const provisionPipeline = useProvisionPipeline()

  const templates = useMemo(
    () =>
      Array.isArray(templatesData) && templatesData.length > 0
        ? templatesData
        : DEFAULT_TEMPLATES,
    [templatesData],
  )

  const template =
    templates.find((t) => t.id === selectedTemplateId) ?? templates[0]

  const clusterList = clustersData?.items ?? []
  const targetClusters = clusterList.filter((c) => {
    const types = (
      Array.isArray((c as any).types) ? (c as any).types : [(c as any).type ?? '']
    )
      .flatMap((t: string) => t.split(','))
      .map((t: string) => t.trim().toLowerCase())
    return types.includes('target')
  })
  const clusterOptions =
    targetClusters.length > 0
      ? targetClusters.map((c) => ({ id: c.id, name: c.name }))
      : [{ id: 'c1', name: 'dev-k8s' }]

  const completedStacks = stacksData?.items ?? []

  // ── Form state ───────────────────────────────────────────────────────────

  const [executionMode, setExecutionMode] = useState<ExecutionMode>('stack_integrated')
  const [pipelineName, setPipelineName] = useState(
    template ? `${template.name.toLowerCase().replace(/\s+/g, '-')}-pipeline` : '',
  )
  const [clusterId, setClusterId] = useState(clusterOptions[0]?.id ?? '')
  const [stackId, setStackId] = useState(completedStacks[0]?.id ?? '')
  const [gitRepoUrl, setGitRepoUrl] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [envRepoUrl, setEnvRepoUrl] = useState('')
  const [selectedTemplate, setSelectedTemplate] = useState(template?.id ?? '')
  const [error, setError] = useState<string | null>(null)

  // Post-creation state
  const [createdPipelineId, setCreatedPipelineId] = useState<string | null>(null)
  const [step, setStep] = useState<'form' | 'provisioning' | 'done'>('form')
  const [provisionResult, setProvisionResult] = useState<{
    gitlabProjectUrl: string
    argocdAppName: string
    argocdSyncUrl: string
  } | null>(null)

  // Stack integrations (only loaded when stack_integrated + stack selected)
  const { data: integrationsData } = useStackIntegrations(
    executionMode === 'stack_integrated' ? stackId : '',
  )

  const integrations: IntegrationStatus[] = useMemo(() => {
    if (!integrationsData?.integrations) return []
    return integrationsData.integrations.map((i: any) => ({
      componentType: i.component_type,
      provider: i.provider,
      health: i.health_status,
      ready: i.health_status === 'ready',
    }))
  }, [integrationsData])

  const allIntegrationsReady =
    integrations.length > 0 && integrations.every((i) => i.ready)

  useEffect(() => {
    if (clusterOptions.some((c) => c.id === clusterId)) return
    setClusterId(clusterOptions[0]?.id ?? '')
  }, [clusterOptions, clusterId])

  useEffect(() => {
    if (completedStacks.length > 0 && !stackId) {
      setStackId(completedStacks[0].id)
    }
  }, [completedStacks, stackId])

  // ── Handlers ─────────────────────────────────────────────────────────────

  const handleCreate = () => {
    if (!pipelineName.trim()) {
      setError('Pipeline 이름을 입력해주세요.')
      return
    }
    if (!clusterId) {
      setError('클러스터를 선택해주세요.')
      return
    }
    if (executionMode === 'stack_integrated' && !stackId) {
      setError('Stack을 선택해주세요.')
      return
    }
    setError(null)

    const currentTemplate = templates.find((t) => t.id === selectedTemplate) ?? template

    createPipeline.mutate(
      {
        name: pipelineName.trim(),
        appType: (currentTemplate?.appType ?? 'web-backend') as AppType,
        clusterId,
        namespace,
        templateId: currentTemplate?.id,
        stackId: executionMode === 'stack_integrated' ? stackId : undefined,
        gitRepoUrl: gitRepoUrl || undefined,
        executionMode,
      },
      {
        onSuccess: (pipeline) => {
          if (executionMode === 'emergency_direct') {
            // 긴급모드: 바로 배포 트리거
            deployPipeline.mutate(
              { pipelineId: pipeline.id },
              {
                onSuccess: (dep) => {
                  navigate(`/cicd/logs/${dep.deploymentId}`)
                },
                onError: (e) => setError(String(e)),
              },
            )
          } else {
            // stack_integrated: provision 단계로 이동
            setCreatedPipelineId(pipeline.id)
            setStep('provisioning')
          }
        },
        onError: (e) => setError(String(e)),
      },
    )
  }

  const handleProvision = () => {
    if (!createdPipelineId) return
    setError(null)

    provisionPipeline.mutate(
      {
        pipelineId: createdPipelineId,
        envRepoUrl: envRepoUrl || undefined,
      },
      {
        onSuccess: (result) => {
          setProvisionResult({
            gitlabProjectUrl: result.gitlab_project_url,
            argocdAppName: result.argocd_app_name,
            argocdSyncUrl: result.argocd_sync_url,
          })
          setStep('done')
        },
        onError: (e) => setError(String(e)),
      },
    )
  }

  // ── Render: Done state ───────────────────────────────────────────────────

  if (step === 'done' && provisionResult) {
    return (
      <div>
        <Breadcrumb
          items={[
            { label: 'CI/CD List', path: '/cicd/list' },
            { label: 'Pipeline Setup' },
          ]}
        />
        <div className="mx-auto max-w-xl py-12 text-center">
          <div className="mb-4 flex justify-center">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-400">
              <CheckCircle2 size={28} />
            </div>
          </div>
          <h2 className="mb-2 text-xl font-bold text-[var(--color-text-primary)]">
            프로비저닝 완료
          </h2>
          <p className="mb-8 text-sm text-[var(--color-text-secondary)]">
            GitLab 프로젝트와 ArgoCD Application이 등록됐습니다.
            <br />
            git push 하면 자동으로 배포됩니다.
          </p>
          <div className="mb-8 flex flex-col gap-3 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5 text-left">
            {[
              ['GitLab 프로젝트', provisionResult.gitlabProjectUrl],
              ['ArgoCD App', provisionResult.argocdAppName],
              ['ArgoCD 대시보드', provisionResult.argocdSyncUrl],
            ].map(([label, value]) => (
              <div key={label} className="flex flex-col gap-0.5">
                <span className="text-[11px] text-[var(--color-text-secondary)]">{label}</span>
                <a
                  href={value.startsWith('http') ? value : undefined}
                  target="_blank"
                  rel="noreferrer"
                  className="break-all text-sm font-semibold text-[#a5b4fc] hover:underline"
                >
                  {value}
                </a>
              </div>
            ))}
          </div>
          <div className="flex justify-center gap-3">
            <Button variant="outline" size="md" onClick={() => navigate('/cicd/list')}>
              목록으로
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => {
                if (createdPipelineId) {
                  deployPipeline.mutate(
                    { pipelineId: createdPipelineId },
                    { onSuccess: (dep) => navigate(`/cicd/logs/${dep.deploymentId}`) },
                  )
                }
              }}
              loading={deployPipeline.isPending}
            >
              <Rocket size={14} />
              지금 Sync 트리거
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // ── Render: Provisioning step ────────────────────────────────────────────

  if (step === 'provisioning') {
    return (
      <div>
        <Breadcrumb
          items={[
            { label: 'CI/CD List', path: '/cicd/list' },
            { label: 'Pipeline Setup' },
          ]}
        />
        <div className="mx-auto max-w-xl py-10">
          <div className="mb-6 flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
              <GitBranch size={18} />
            </div>
            <div>
              <h2 className="m-0 text-lg font-bold text-[var(--color-text-primary)]">
                GitLab + ArgoCD 프로비저닝
              </h2>
              <p className="m-0 text-xs text-[var(--color-text-secondary)]">
                GitLab 프로젝트를 생성하고 ArgoCD Application을 등록합니다.
              </p>
            </div>
          </div>

          <div className="mb-4 flex flex-col gap-4 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            <Input
              label="Environment Repo URL (선택)'
              placeholder="https://gitlab.example.com/team/app-env (비우면 소스 레포 사용)"
              value={envRepoUrl}
              onChange={(e) => setEnvRepoUrl(e.target.value)}
            />
            <div className="rounded-lg border border-[rgba(99,102,241,0.25)] bg-[rgba(99,102,241,0.05)] p-3 text-xs text-[var(--color-text-secondary)]">
              <strong className="text-[#a5b4fc]">GitOps 흐름</strong>
              <br />
              git push → GitLab CI 빌드 → 이미지 push → Environment Repo 업데이트 → ArgoCD sync → 클러스터 배포
            </div>
          </div>

          {error && (
            <div className="mb-4 flex items-start gap-2 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-xs text-red-400">
              <AlertTriangle size={14} className="mt-0.5 shrink-0" />
              {error}
            </div>
          )}

          <div className="flex justify-between gap-3">
            <Button
              variant="outline"
              size="md"
              onClick={() => {
                setStep('form')
                setCreatedPipelineId(null)
                setError(null)
              }}
            >
              이전으로
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={handleProvision}
              loading={provisionPipeline.isPending}
            >
              <GitBranch size={14} />
              프로비저닝 실행
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // ── Render: Main form ────────────────────────────────────────────────────

  const selectedClusterName =
    clusterOptions.find((c) => c.id === clusterId)?.name ?? '—'
  const selectedStackName =
    completedStacks.find((s) => s.id === stackId)?.name ?? '—'
  const currentTemplateName =
    templates.find((t) => t.id === selectedTemplate)?.name ??
    template?.name ??
    '—'

  return (
    <div>
      <Breadcrumb
        items={[
          { label: 'CI/CD List', path: '/cicd/list' },
          { label: 'CI/CD Template', path: '/cicd/templates' },
          { label: 'Pipeline Setup' },
        ]}
      />

      {/* Header */}
      <div className="mb-6 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <Settings2 size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('cicdPipelineSetupPage.title', 'CI/CD Pipeline Setup')}
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              배포 모드를 선택하고 파이프라인을 구성하세요.
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          size="md"
          type="button"
          onClick={() => navigate('/cicd/templates')}
        >
          <GitBranch size={14} />
          템플릿 변경
        </Button>
      </div>

      <div className="flex items-start gap-6">
        {/* Left: form */}
        <div className="min-w-0 flex-1 flex flex-col gap-5">

          {/* Step 1: Execution mode */}
          <section className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            <h2 className="mb-4 mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
              Step 1 — 배포 모드 선택
            </h2>
            <div className="grid grid-cols-2 gap-3">
              <ModeCard
                mode="stack_integrated"
                selected={executionMode === 'stack_integrated'}
                onClick={() => setExecutionMode('stack_integrated')}
              />
              <ModeCard
                mode="emergency_direct"
                selected={executionMode === 'emergency_direct'}
                onClick={() => setExecutionMode('emergency_direct')}
              />
            </div>
          </section>

          {/* Step 2: Basic config */}
          <section className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            <h2 className="mb-4 mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
              Step 2 — 기본 설정
            </h2>
            <div className="grid grid-cols-2 gap-4">
              <Input
                label="Pipeline 이름"
                placeholder="e.g. orders-api-prod"
                value={pipelineName}
                onChange={(e) => setPipelineName(e.target.value)}
              />
              <NativeSelect
                label="템플릿"
                value={selectedTemplate || template?.id || ''}
                onChange={(e) => setSelectedTemplate(e.target.value)}
                className="w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
              >
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </NativeSelect>
              <NativeSelect
                label="Target 클러스터"
                value={clusterId}
                onChange={(e) => setClusterId(e.target.value)}
                className="w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
              >
                {clusterOptions.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </NativeSelect>
              <Input
                label="Namespace"
                placeholder="default"
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
              />
              <div className="col-span-2">
                <Input
                  label="Git Repository URL"
                  placeholder="https://gitlab.example.com/team/app.git"
                  value={gitRepoUrl}
                  onChange={(e) => setGitRepoUrl(e.target.value)}
                />
              </div>
            </div>
          </section>

          {/* Step 3: Stack selection (stack_integrated only) */}
          {executionMode === 'stack_integrated' && (
            <section className="rounded-xl border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.04)] p-5">
              <h2 className="mb-4 mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[#a5b4fc]">
                Step 3 — Stack 연동
              </h2>

              {completedStacks.length === 0 ? (
                <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/08 p-3 text-xs text-amber-400">
                  <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                  <span>
                    completed 상태의 Stack이 없습니다. Stack을 먼저 배포하거나,{' '}
                    <button
                      type="button"
                      className="underline"
                      onClick={() => setExecutionMode('emergency_direct')}
                    >
                      Emergency Direct 모드
                    </button>
                    를 사용하세요.
                  </span>
                </div>
              ) : (
                <>
                  <NativeSelect
                    label="Stack 선택 (completed 상태만 표시)"
                    value={stackId}
                    onChange={(e) => setStackId(e.target.value)}
                    className="mb-4 w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                  >
                    {completedStacks.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </NativeSelect>

                  {/* Integration status */}
                  {integrations.length > 0 ? (
                    <div className="flex flex-col gap-2">
                      <p className="m-0 mb-1 text-[11px] text-[var(--color-text-secondary)]">
                        Stack Integration 상태
                      </p>
                      {integrations.map((i) => (
                        <IntegrationBadge key={i.componentType} status={i} />
                      ))}
                      {!allIntegrationsReady && (
                        <div className="mt-1 flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/08 p-2.5 text-[11px] text-amber-400">
                          <AlertTriangle size={12} className="mt-0.5 shrink-0" />
                          일부 integration이 준비되지 않았습니다. Stack config의 credentials.gitlab_token / argocd_token을 확인하세요.
                        </div>
                      )}
                    </div>
                  ) : stackId ? (
                    <div className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
                      <Loader2 size={12} className="animate-spin" />
                      Integration 상태 조회 중...
                    </div>
                  ) : null}
                </>
              )}
            </section>
          )}

          {/* Emergency mode info */}
          {executionMode === 'emergency_direct' && (
            <section className="rounded-xl border border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.05)] p-5">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[rgba(245,158,11,0.15)] text-[#f59e0b]">
                  <Zap size={16} />
                </div>
                <div>
                  <p className="m-0 mb-1 text-sm font-bold text-[#fbbf24]">긴급 직접 배포 모드</p>
                  <p className="m-0 text-xs text-[var(--color-text-secondary)] leading-relaxed">
                    파이프라인 생성 즉시 Nullus가 직접 배포를 실행합니다.
                  </p>
                  <ul className="m-0 mt-2 list-none p-0 text-[11px] text-[var(--color-text-secondary)]">
                    <li className="flex items-center gap-1.5 py-0.5">
                      <ChevronRight size={10} className="text-[#f59e0b]" />
                      Git Clone → Docker Build → kubectl apply
                    </li>
                    <li className="flex items-center gap-1.5 py-0.5">
                      <ChevronRight size={10} className="text-[#f59e0b]" />
                      Stack 불필요 — 클러스터 kubeconfig만 있으면 동작
                    </li>
                    <li className="flex items-center gap-1.5 py-0.5">
                      <ChevronRight size={10} className="text-[#f59e0b]" />
                      Git URL이 없으면 기본 nginx 이미지로 Namespace + Deployment 생성
                    </li>
                  </ul>
                </div>
              </div>
            </section>
          )}

          {error && (
            <div className="flex items-start gap-2 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-xs text-red-400">
              <AlertTriangle size={14} className="mt-0.5 shrink-0" />
              {error}
            </div>
          )}

          {/* CTA */}
          <div className="flex justify-end gap-3">
            <Button
              variant="outline"
              size="md"
              type="button"
              onClick={() => navigate('/cicd/list')}
            >
              취소
            </Button>
            <Button
              variant="primary"
              size="md"
              type="button"
              onClick={handleCreate}
              loading={createPipeline.isPending || deployPipeline.isPending}
            >
              {executionMode === 'stack_integrated' ? (
                <>
                  <GitBranch size={14} />
                  파이프라인 생성 &amp; 프로비저닝
                </>
              ) : (
                <>
                  <Zap size={14} />
                  생성 &amp; 즉시 배포
                </>
              )}
            </Button>
          </div>
        </div>

        {/* Right: summary sidebar */}
        <div className="sticky top-6 w-[260px] shrink-0 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h3 className="mb-3 mt-0 text-[12px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            Summary
          </h3>
          <div className="flex flex-col gap-0">
            {[
              ['Mode', executionMode === 'stack_integrated' ? 'Stack Integrated' : 'Emergency Direct'],
              ['Template', currentTemplateName],
              ['Pipeline', pipelineName || '—'],
              ['Cluster', selectedClusterName],
              ['Namespace', namespace || 'default'],
              ...(executionMode === 'stack_integrated'
                ? [['Stack', selectedStackName]]
                : []),
              ...(gitRepoUrl ? [['Git URL', gitRepoUrl]] : []),
            ].map(([label, value]) => (
              <div
                key={label}
                className="flex items-baseline justify-between gap-2 border-b border-[rgba(255,255,255,0.04)] py-1.5 last:border-0"
              >
                <span className="shrink-0 text-[11px] text-[var(--color-text-secondary)]">
                  {label}
                </span>
                <span
                  className={cn(
                    'overflow-hidden text-ellipsis whitespace-nowrap text-right text-xs font-semibold',
                    label === 'Mode' && executionMode === 'stack_integrated'
                      ? 'text-[#a5b4fc]'
                      : label === 'Mode'
                        ? 'text-[#fbbf24]'
                        : 'text-[var(--color-text-primary)]',
                  )}
                  title={value}
                >
                  {value}
                </span>
              </div>
            ))}
          </div>

          {/* Integration summary dot */}
          {executionMode === 'stack_integrated' && integrations.length > 0 && (
            <div className="mt-3 flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] p-2">
              <div
                className={cn(
                  'h-2 w-2 rounded-full',
                  allIntegrationsReady ? 'bg-emerald-400' : 'bg-amber-400',
                )}
              />
              <span className="text-[11px] text-[var(--color-text-secondary)]">
                Integration{' '}
                <span
                  className={cn(
                    'font-semibold',
                    allIntegrationsReady ? 'text-emerald-400' : 'text-amber-400',
                  )}
                >
                  {allIntegrationsReady ? 'Ready' : 'Not ready'}
                </span>
              </span>
            </div>
          )}

          <div className="mt-4">
            <div
              className={cn(
                'rounded-lg border px-3 py-2 text-center text-[11px] font-semibold',
                executionMode === 'stack_integrated'
                  ? 'border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.08)] text-[#a5b4fc]'
                  : 'border-[rgba(245,158,11,0.3)] bg-[rgba(245,158,11,0.08)] text-[#fbbf24]',
              )}
            >
              {executionMode === 'stack_integrated'
                ? '생성 → 프로비저닝 → GitOps'
                : '생성 → 즉시 kubectl apply'}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
