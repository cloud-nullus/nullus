import { useEffect, useRef, useState } from 'react'
import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Rocket, Plus, Trash2, ChevronRight, Check, Loader2, Copy, Terminal } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { CodePreview } from '../../../components/shared/code-preview'
import { Breadcrumb } from '../../../components/shared/breadcrumb'

import { useCreatePipeline, useDeployPipeline } from '../api/cicd-api'
import type { AppType } from '../api/cicd-api'
import { useClusterNamespaces, useClusters } from '../../admin/api/admin-api'
import { useStacks } from '../../stack/api/stack-api'
import { cn } from '../../../lib/utils'
import { useCicdDeployLog, type CicdLogLevel } from '../hooks/use-cicd-deploy-log'

type Step = 1 | 2 | 3 | 4 | 5 | 6

const STEP_LABELS: Record<Step, string> = {
  1: '앱 이름',
  2: 'Git Repository',
  3: '클러스터 / 네임스페이스',
  4: '리소스 설정',
  5: '환경 변수',
  6: '매니페스트 확인',
}

const PHASES = ['Namespace 생성', 'Deployment 생성', 'Service 생성']
const PROGRESS_SEGMENTS = Array.from({ length: 100 }, (_, i) => i + 1)

const LOG_LEVEL_STYLE: Record<CicdLogLevel, string> = {
  info: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  success: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  error: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
}

function PhaseStep({ label, index, progress }: { label: string; index: number; progress: number }) {
  const phaseProgress = 100 / PHASES.length
  const phaseStart = index * phaseProgress
  const isDone = progress >= phaseStart + phaseProgress
  const isActive = progress >= phaseStart && !isDone

  return (
    <div className="flex flex-1 items-center gap-2">
      <div
        className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-all duration-300',
          isDone
            ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]'
            : isActive
              ? 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]'
              : 'bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]'
        )}
      >
        {isDone ? (
          <Check size={14} />
        ) : isActive ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          <span className="text-xs font-bold">{index + 1}</span>
        )}
      </div>
      <span
        className={cn(
          'text-[13px] font-semibold',
          isDone ? 'text-[#22c55e]' : isActive ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]'
        )}
      >
        {label}
      </span>
      {index < PHASES.length - 1 && (
        <div
          className={cn(
            'mx-1 h-px flex-1 transition-colors duration-300',
            isDone ? 'bg-[rgba(34,197,94,0.4)]' : 'bg-[var(--color-border-default)]'
          )}
        />
      )}
    </div>
  )
}

function generateYaml(form: Partial<FormState>, appType: string): string {
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${form.appName ?? 'my-app'}
  namespace: ${form.namespace ?? 'default'}
  labels:
    app: ${form.appName ?? 'my-app'}
    template: ${appType || 'backend'}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${form.appName ?? 'my-app'}
  template:
    metadata:
      labels:
        app: ${form.appName ?? 'my-app'}
    spec:
      containers:
        - name: ${form.appName ?? 'my-app'}
          image: harbor.nullus.io/${form.appName ?? 'my-app'}:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${(form.envVars ?? []).filter((e) => e.key).length > 0
  ? `          env:\n${(form.envVars ?? []).filter((e) => e.key).map((e) => `            - name: ${e.key}\n              value: "${e.value}"`).join('\n')}`
  : ''}
---
apiVersion: v1
kind: Service
metadata:
  name: ${form.appName ?? 'my-app'}-svc
  namespace: ${form.namespace ?? 'default'}
spec:
  selector:
    app: ${form.appName ?? 'my-app'}
  ports:
    - port: 80
      targetPort: 8080`
}

interface EnvVar { key: string; value: string }

interface FormState {
  appName: string
  gitUrl: string
  clusterId: string
  namespace: string
  replicas: number
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  envVars: EnvVar[]
}

const deploySchema = z.object({
  appName: z.string().min(2, 'App name must be at least 2 characters').max(50, 'App name must be 50 characters or less'),
  gitUrl: z.string().min(1, 'Git URL is required'),
  clusterId: z.string().min(1, 'Cluster is required'),
  namespace: z.string().min(1, 'Namespace is required'),
  replicas: z.number().min(1).max(10),
  cpuRequest: z.string().min(1, 'CPU request is required'),
  cpuLimit: z.string().min(1, 'CPU limit is required'),
  memoryRequest: z.string().min(1, 'Memory request is required'),
  memoryLimit: z.string().min(1, 'Memory limit is required'),
  envVars: z
    .array(
      z.object({
        key: z.string(),
        value: z.string(),
      })
    )
    .superRefine((envVars, ctx) => {
      envVars.forEach((env, index) => {
        if (env.value.trim() && !env.key.trim()) {
          ctx.addIssue({
            code: 'custom',
            message: 'Key is required when value exists',
            path: [index, 'key'],
          })
        }
      })
    }),
})

const DEFAULT_FORM: FormState = {
  appName: '',
  gitUrl: '',
  clusterId: '',
  namespace: 'default',
  replicas: 2,
  cpuRequest: '100m',
  cpuLimit: '500m',
  memoryRequest: '128Mi',
  memoryLimit: '512Mi',
  envVars: [{ key: '', value: '' }],
}

export function DeveloperDeployPage() {
  const [step, setStep] = useState<Step>(1)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const appType = searchParams.get('appType') ?? 'backend'
  const [deploymentId, setDeploymentId] = useState<string | null>(null)
  const [customManifest, setCustomManifest] = useState<string | null>(null)
  const [selectedStackId, setSelectedStackId] = useState('')
  const [repoName, setRepoName] = useState('')
  const { logs, status, progress, isConnected } = useCicdDeployLog(deploymentId)
  const terminalRef = useRef<HTMLDivElement>(null)
  const { data: stacksData } = useStacks()
  const stacks = (stacksData?.items ?? []).map((s) => ({
    id: s.id,
    name: s.name,
  }))
  const { data: clustersData } = useClusters()
  const clusters = (clustersData?.items ?? []).map((c) => ({
    id: c.id,
    name: c.name,
  }))
  const {
    register,
    control,
    watch,
    setValue,
    trigger,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormState>({
    resolver: zodResolver(deploySchema),
    defaultValues: DEFAULT_FORM,
    mode: 'onChange',
  })
  const { fields, append, remove } = useFieldArray({
    control,
    name: 'envVars',
  })

  const pipelineIdParam = searchParams.get('pipelineId') ?? ''
  const clusterIdParam = searchParams.get('clusterId') ?? ''
  const namespaceParam = searchParams.get('namespace') ?? ''
  const appNameParam = searchParams.get('appName') ?? ''

  const form = watch()
  const { data: namespacesData } = useClusterNamespaces(form.clusterId)
  const namespaces = (namespacesData ?? []).map((ns) => ns.name)
  const selectedStack = stacks.find((s) => s.id === selectedStackId)
  const stackGitBaseUrl = selectedStack ? `http://${selectedStack.name}.internal/` : ''
  const gitUrl = selectedStackId ? `${stackGitBaseUrl}${repoName}` : form.gitUrl

  const firstClusterId = clusters[0]?.id ?? ''
  useEffect(() => {
    if (firstClusterId && !form.clusterId) {
      setValue('clusterId', firstClusterId, { shouldValidate: true })
    }
  }, [firstClusterId, form.clusterId, setValue])

  useEffect(() => {
    if (logs.length === 0) return
    const el = terminalRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [logs])

  useEffect(() => {
    if (!pipelineIdParam && !clusterIdParam && !namespaceParam && !appNameParam) {
      return
    }

    if (clusterIdParam) {
      setValue('clusterId', clusterIdParam, { shouldValidate: true })
    }
    if (namespaceParam) {
      setValue('namespace', namespaceParam, { shouldValidate: true })
    }
    if (appNameParam) {
      setValue('appName', appNameParam, { shouldValidate: true })
    }
  }, [pipelineIdParam, clusterIdParam, namespaceParam, appNameParam, setValue])

  const firstNamespace = namespaces[0] ?? ''
  useEffect(() => {
    if (firstNamespace) {
      setValue('namespace', firstNamespace, { shouldValidate: true })
    }
  }, [firstNamespace, setValue])

  const createPipelineMutation = useCreatePipeline()
  const deployPipelineMutation = useDeployPipeline()

  const setField = (key: keyof FormState, value: FormState[keyof FormState]) => {
    setValue(key as never, value as never, { shouldValidate: true, shouldDirty: true })
  }

  const selectedCluster = clusters.find((c) => c.id === form.clusterId) ?? clusters[0] ?? { id: '', name: '' }

  const onSubmit = async (data: FormState) => {
    try {
      const parsedAppType = appType as AppType
      const pipeline = await createPipelineMutation.mutateAsync({
        name: data.appName,
        appType: parsedAppType,
        clusterId: data.clusterId,
        namespace: data.namespace,
      })
      const result = await deployPipelineMutation.mutateAsync(pipeline.id)
      setDeploymentId(result.deploymentId)
    } catch { /* react-query handles mutation errors */ }
  }

  const validateCurrentStep = async () => {
    if (step === 1) return trigger('appName')
    if (step === 2) {
      setValue('gitUrl', gitUrl, { shouldValidate: true, shouldDirty: true })
      return trigger('gitUrl')
    }
    if (step === 3) return trigger(['clusterId', 'namespace'])
    if (step === 4) return trigger(['replicas', 'cpuLimit', 'memoryLimit'])
    if (step === 5) return true
    return true
  }

  const canNext: Record<Step, boolean> = {
    1: form.appName.trim().length >= 2,
    2: gitUrl.trim().length > 0 && !errors.gitUrl,
    3: !!form.clusterId && !!form.namespace,
    4: form.replicas >= 1 && !!form.cpuLimit && !!form.memoryLimit,
    5: true,
    6: true,
  }

  if (deploymentId) {
    const isComplete = status === 'success'
    const isFailed = status === 'failed'
    const isDone = isComplete || isFailed

    const deployedResources = logs
      .map((entry) => entry.message)
      .filter((line) => !line.startsWith('$') && !line.startsWith('error'))
      .map((line) => {
        const match = line.match(/^(\w+)\/(\S+)\s+(\w+)$/)
        return match ? { kind: match[1], name: match[2], action: match[3] } : null
      })
      .filter((r): r is { kind: string; name: string; action: string } => r !== null)

    return (
      <div>
        <Breadcrumb items={[{ label: 'CI/CD List', path: '/cicd/list' }, { label: '배포 진행' }]} />
        <div className="mx-auto max-w-3xl py-12">
          <div className="mb-8 text-center">
            <h2 className="m-0 text-xl font-bold text-[var(--color-text-primary)]">{form.appName}</h2>
            <p className="m-0 mt-1 text-sm text-[var(--color-text-secondary)]">{form.namespace} 네임스페이스</p>
          </div>

          <div className="mb-8 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-6">
            <div className="mb-5 flex items-center gap-2">
              {PHASES.map((phase, idx) => (
                <PhaseStep key={phase} label={phase} index={idx} progress={progress} />
              ))}
            </div>

            <div className="mb-1 flex justify-between text-xs">
              <span className="text-[var(--color-text-secondary)]">전체 진행률</span>
              <span className={cn('font-semibold', isFailed ? 'text-[#ef4444]' : 'text-[var(--color-text-primary)]')}>{progress}%</span>
            </div>
            <div className="mb-6 flex gap-px">
              {PROGRESS_SEGMENTS.map((segment) => (
                <div
                  key={segment}
                  className={cn(
                    'h-1 flex-1 rounded-full transition-colors duration-300',
                    segment <= progress
                      ? status === 'failed' ? 'bg-[#ef4444]' : 'bg-[#6366f1]'
                      : 'bg-[rgba(255,255,255,0.06)]'
                  )}
                />
              ))}
            </div>

            <div className="text-center text-xs text-[var(--color-text-secondary)]">
              Deployment ID: <span className="font-mono text-[var(--color-text-primary)]">{deploymentId}</span>
            </div>
          </div>

          <div className="mb-6 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[#0d1117]">
            <div className="flex items-center gap-2 border-b border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.03)] px-4 py-2">
              <div className="flex gap-1.5">
                <div className="h-2.5 w-2.5 rounded-full bg-[#ff5f57]" />
                <div className="h-2.5 w-2.5 rounded-full bg-[#febc2e]" />
                <div className="h-2.5 w-2.5 rounded-full bg-[#28c840]" />
              </div>
              <Terminal size={12} className="text-[rgba(255,255,255,0.4)]" />
              <span className="text-[11px] font-medium text-[rgba(255,255,255,0.4)]">
                {isConnected ? 'Streaming...' : 'Connecting...'}
              </span>
            </div>
            <div ref={terminalRef} className="max-h-[400px] overflow-y-auto p-4">
              {logs.length === 0 ? (
                <span className="font-mono text-xs text-[#484f58]">
                  {isFailed ? '배포 실패. 로그를 확인할 수 없습니다.' : 'Waiting for deployment output...'}
                </span>
              ) : (
                <div className="flex flex-col gap-1.5">
                  {logs.map((entry) => (
                    <div key={entry.id} className="flex gap-2 leading-5">
                      <span className="shrink-0 text-xs text-[#484f58]">
                        {new Date(entry.timestamp).toLocaleTimeString('ko-KR')}
                      </span>
                      <span className={cn('rounded px-1 py-0.5 text-[10px] font-bold uppercase', LOG_LEVEL_STYLE[entry.level])}>
                        {entry.level}
                      </span>
                      <span className="text-xs text-[#c9d1d9]">{entry.message}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {isComplete && deployedResources.length > 0 && (() => {
            const nsScoped = deployedResources.filter((r) => r.kind !== 'namespace')
            const contextFlag = selectedCluster.name ? ` --context ${selectedCluster.name}` : ''
            const cmd = nsScoped.length > 0
              ? `kubectl get ${nsScoped.map((r) => `${r.kind.toLowerCase()}/${r.name}`).join(' ')} -n ${form.namespace}${contextFlag}`
              : null
            return (
              <div className="mb-6 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
                <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                  생성된 리소스
                </p>
                <div className="flex flex-col gap-1.5">
                  {deployedResources.map((r) => (
                    <div key={`${r.kind}-${r.name}`} className="flex items-center justify-between rounded-md bg-[rgba(255,255,255,0.03)] px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className="rounded bg-[rgba(99,102,241,0.15)] px-1.5 py-0.5 text-[11px] font-bold text-[#818cf8]">
                          {r.kind}
                        </span>
                        <span className="font-mono text-[13px] text-[var(--color-text-primary)]">{r.name}</span>
                      </div>
                      <span className={cn(
                        'text-[11px] font-semibold',
                        r.action === 'created' ? 'text-[#22c55e]' : 'text-[#d29922]'
                      )}>{r.action}</span>
                    </div>
                  ))}
                </div>
                {cmd && <CopyableCommand command={cmd} />}
              </div>
            )
          })()}

          {isDone && (
            <div className="flex justify-center gap-3">
              <Button variant="outline" size="md" onClick={() => { setDeploymentId(null); reset(DEFAULT_FORM); setStep(1) }}>
                새 배포
              </Button>
              <Button variant="primary" size="md" onClick={() => navigate('/cicd/list')}>
                CI/CD 목록 보기
              </Button>
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div>
      <Breadcrumb
        items={[
          { label: 'CI/CD List', path: '/cicd/list' },
          { label: 'Pipeline Setup & Deploy' },
        ]}
      />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div
          className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
        >
          <Rocket size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            CI/CD Pipeline Setup & Developer Deploy
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            파이프라인 템플릿 선택과 개발자 배포를 하나의 화면에서 진행하세요.
          </p>
        </div>
      </div>

      {/* Step indicator */}
      <div className="mb-6 flex flex-wrap items-center gap-1">
        {([1, 2, 3, 4, 5, 6] as Step[]).map((s, i) => (
          <div key={s} className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => s < step && setStep(s)}
              className={cn('flex items-center gap-1.5 rounded-md border-none bg-none px-1.5 py-1', s < step ? 'cursor-pointer' : 'cursor-default')}
            >
              <div
                className={cn(
                  'flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full text-[11px] font-bold',
                  s === step
                    ? 'bg-[#6366f1] text-white'
                    : s < step
                      ? 'bg-[rgba(34,197,94,0.3)] text-[#22c55e]'
                      : 'bg-[rgba(255,255,255,0.08)] text-[var(--color-text-secondary)]'
                )}
              >
                {s}
              </div>
              <span
                className={cn(
                  'text-[13px]',
                  s === step ? 'font-semibold text-[var(--color-text-primary)]' : 'font-normal text-[var(--color-text-secondary)]'
                )}
              >
                {STEP_LABELS[s]}
              </span>
            </button>
            {i < 5 && <ChevronRight size={14} className="shrink-0 text-[var(--color-text-secondary)]" />}
          </div>
        ))}
      </div>

      {/* Step content */}
      <div className="grid grid-cols-2 items-start gap-6">
        <div
          className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-6"
        >
          {step === 1 && (
            <StepSection title="앱 이름 입력">
              <Input
                placeholder="my-awesome-app"
                value={form.appName}
                onChange={(e) => setField('appName', e.target.value)}
              />
              {errors.appName && <span className="text-xs text-[#ef4444]">{errors.appName.message}</span>}
              <p className="mb-0 mt-1.5 text-xs text-[var(--color-text-secondary)]">
                소문자, 숫자, 하이픈만 사용 가능합니다.
              </p>
            </StepSection>
          )}

          {step === 2 && (
            <StepSection title="Git Repository URL">
              <div className="flex flex-col gap-3">
                <div>
                  <label htmlFor="deploy-stack" className={labelStyleClass}>스택 (선택)</label>
                  <NativeSelect
                    id="deploy-stack"
                    value={selectedStackId}
                    onChange={(e) => setSelectedStackId(e.target.value)}
                    className="w-full"
                  >
                    <option value="">직접 입력</option>
                    {stacks.map((s) => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </NativeSelect>
                </div>

                {selectedStackId ? (
                  <div className="grid grid-cols-2 gap-2">
                    <Input value={stackGitBaseUrl} disabled />
                    <Input
                      placeholder="repo-name.git"
                      value={repoName}
                      onChange={(e) => setRepoName(e.target.value)}
                    />
                  </div>
                ) : (
                  <Input
                    placeholder="https://github.com/org/repo.git"
                    value={form.gitUrl}
                    onChange={(e) => setField('gitUrl', e.target.value)}
                  />
                )}
              </div>
              {errors.gitUrl && <span className="text-xs text-[#ef4444]">{errors.gitUrl.message}</span>}
            </StepSection>
          )}

          {step === 3 && (
            <StepSection title="클러스터 & 네임스페이스">
              <div className="flex flex-col gap-3">
                <div>
                  <label htmlFor="deploy-cluster" className={labelStyleClass}>클러스터</label>
                  <NativeSelect
                    id="deploy-cluster"
                    value={form.clusterId}
                    onChange={(e) => {
                      setField('clusterId', e.target.value)
                      if (namespaces[0]) setField('namespace', namespaces[0])
                    }}
                    className="w-full"
                  >
                    {clusters.map((c) => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </NativeSelect>
                  {errors.clusterId && <span className="text-xs text-[#ef4444]">{errors.clusterId.message}</span>}
                </div>
                <div>
                  <label htmlFor="deploy-namespace" className={labelStyleClass}>네임스페이스</label>
                  <NativeSelect
                    id="deploy-namespace"
                    value={form.namespace}
                    onChange={(e) => setField('namespace', e.target.value)}
                    className="w-full"
                  >
                    {namespaces.map((ns) => (
                      <option key={ns} value={ns}>{ns}</option>
                    ))}
                  </NativeSelect>
                  {errors.namespace && <span className="text-xs text-[#ef4444]">{errors.namespace.message}</span>}
                </div>
              </div>
            </StepSection>
          )}

          {step === 4 && (
            <StepSection title="리소스 설정">
              <div className="flex flex-col gap-4">
                <ResourceSlider
                  label="Replicas"
                  value={String(form.replicas)}
                  options={['1', '2', '3', '4', '5']}
                  onChange={(v) => setField('replicas', Number(v))}
                />
                <ResourceSlider
                  label="CPU Request"
                  value={form.cpuRequest}
                  options={['100m', '200m', '500m', '1000m']}
                  onChange={(v) => setField('cpuRequest', v)}
                />
                <ResourceSlider
                  label="CPU Limit"
                  value={form.cpuLimit}
                  options={['200m', '500m', '1000m', '2000m']}
                  onChange={(v) => setField('cpuLimit', v)}
                />
                <ResourceSlider
                  label="Memory Request"
                  value={form.memoryRequest}
                  options={['64Mi', '128Mi', '256Mi', '512Mi']}
                  onChange={(v) => setField('memoryRequest', v)}
                />
                <ResourceSlider
                  label="Memory Limit"
                  value={form.memoryLimit}
                  options={['128Mi', '256Mi', '512Mi', '1Gi', '2Gi']}
                  onChange={(v) => setField('memoryLimit', v)}
                />
              </div>
            </StepSection>
          )}

          {step === 5 && (
            <StepSection title="환경 변수">
              <div className="flex flex-col gap-2">
                {fields.map((field, i) => (
                  <div key={field.id}>
                    <div className="flex items-center gap-2">
                      <Input
                        placeholder="KEY"
                        {...register(`envVars.${i}.key`)}
                        className="flex-1 font-mono text-[13px]"
                      />
                      <Input
                        placeholder="value"
                        {...register(`envVars.${i}.value`)}
                        className="flex-[2] font-mono text-[13px]"
                      />
                      <button
                        type="button"
                        onClick={() => remove(i)}
                        className="shrink-0 cursor-pointer border-none bg-none p-1 text-[#f87171]"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                    {errors.envVars?.[i]?.key?.message && (
                      <span className="text-xs text-[#ef4444]">
                        {errors.envVars[i]?.key?.message}
                      </span>
                    )}
                  </div>
                ))}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => append({ key: '', value: '' })}
                  className="mt-1 self-start"
                  type="button"
                >
                  <Plus size={13} />
                  변수 추가
                </Button>
              </div>
            </StepSection>
          )}

          {step === 6 && (
            <StepSection title="매니페스트 확인 및 편집">
              <p className="mb-3 text-xs text-[var(--color-text-secondary)]">
                생성된 YAML 매니페스트를 확인하고 필요 시 수정하세요.
              </p>
              <textarea
                value={customManifest ?? generateYaml(form, appType)}
                onChange={(e) => setCustomManifest(e.target.value)}
                className="h-[400px] w-full resize-none rounded-lg border border-[var(--color-border-default)] bg-[#0d1117] p-4 font-mono text-xs leading-5 text-[#c9d1d9] focus:outline-none focus:ring-1 focus:ring-[#6366f1]"
                spellCheck={false}
              />
              <button type="button" onClick={() => setCustomManifest(null)} className="mt-2 cursor-pointer border-none bg-none text-xs text-[var(--color-text-secondary)] underline">
                기본값으로 초기화
              </button>
            </StepSection>
          )}

          {/* Navigation */}
          <div className="mt-6 flex justify-end gap-2.5">
            {step > 1 && (
              <Button variant="outline" size="md" onClick={() => setStep((s) => (s - 1) as Step)}>
                이전
              </Button>
            )}
            {step < 6 ? (
              <Button
                variant="primary"
                size="md"
                disabled={!canNext[step]}
                onClick={async () => {
                  const isStepValid = await validateCurrentStep()
                  if (!isStepValid) return
                  setStep((s) => (s + 1) as Step)
                }}
              >
                다음
              </Button>
            ) : (
              <Button
                variant="primary"
                size="md"
                loading={createPipelineMutation.isPending || deployPipelineMutation.isPending}
                disabled={isSubmitting || !!errors.envVars}
                onClick={handleSubmit((data) => {
                  setValue('gitUrl', gitUrl, { shouldValidate: true, shouldDirty: true })
                  return onSubmit({ ...data, gitUrl })
                })}
              >
                <Rocket size={14} />
                Deploy
              </Button>
            )}
          </div>
        </div>

        {/* YAML preview */}
        {step !== 6 && (
        <div>
          <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            YAML 매니페스트 미리보기
          </p>
          <CodePreview
            code={generateYaml(form, appType)}
            language="yaml"
            title={`${form.appName || 'my-app'}.yaml`}
            maxHeight="600px"
          />
        </div>
        )}
      </div>
    </div>
  )
}

function StepSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-4 mt-0 text-[15px] font-bold text-[var(--color-text-primary)]">
        {title}
      </p>
      {children}
    </div>
  )
}

function ResourceSlider({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: string[]
  onChange: (v: string) => void
}) {
  const idx = options.indexOf(value)
  const isCustom = idx === -1
  const sliderId = `resource-${label.toLowerCase().replace(/\s+/g, '-')}`
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <label htmlFor={sliderId} className={cn(labelStyleClass, 'mb-0')}>{label}</label>
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-24 text-right font-mono text-[13px]"
        />
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={options.length - 1}
        value={isCustom ? 0 : idx}
        onChange={(e) => onChange(options[Number(e.target.value)])}
        className="w-full accent-[#6366f1]"
      />
      <div className="mt-1 flex justify-between">
        {options.map((o) => (
          <span key={o} className="font-mono text-[10px] text-[var(--color-text-secondary)]">
            {o}
          </span>
        ))}
      </div>
    </div>
  )
}

function CopyableCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="mt-3 flex items-center gap-2 rounded-md bg-[#0d1117] px-3 py-2">
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-[#c9d1d9]">
        <span className="mr-1.5 text-[#484f58]">$</span>{command}
      </code>
      <button
        type="button"
        onClick={() => { void navigator.clipboard.writeText(command); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
        className="shrink-0 cursor-pointer border-none bg-none p-1 text-[rgba(255,255,255,0.4)] transition-colors hover:text-white"
      >
        {copied ? <Check size={14} className="text-[#3fb950]" /> : <Copy size={14} />}
      </button>
    </div>
  )
}

const labelStyleClass = 'mb-1.5 block text-xs font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]'
