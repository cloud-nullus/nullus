import { useEffect, useRef, useState } from 'react'
import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { Rocket, Plus, Trash2, ChevronRight, Check, X, Loader2, Copy } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { CodePreview } from '../../../components/shared/code-preview'
import { Breadcrumb } from '../../../components/shared/breadcrumb'

import { useAppTemplates, useCreatePipeline, useDeployPipeline, useDeploymentStatus } from '../api/cicd-api'
import { useClusters } from '../../admin/api/admin-api'
import { cn } from '../../../lib/utils'

const TEMPLATE_GIT_REPOS: Record<string, string> = {
  'go-web-api': 'https://github.com/cloud-nullus/sample-go-api',
  'react-vite': 'https://github.com/cloud-nullus/sample-react-app',
  'spring-boot': 'https://github.com/cloud-nullus/sample-spring-boot',
}

type Step = 1 | 2 | 3 | 4 | 5

const STEP_LABELS: Record<Step, string> = {
  1: '앱 이름',
  2: 'Git Repository',
  3: '클러스터 / 네임스페이스',
  4: '리소스 설정',
  5: '환경 변수',
}

function generateYaml(form: Partial<FormState>): string {
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
    template: ${form.template ?? 'go-web-api'}
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
  template: string
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
  template: z.string().min(1, 'Template is required'),
  appName: z.string().min(2, 'App name must be at least 2 characters').max(50, 'App name must be 50 characters or less'),
  gitUrl: z.string().min(1, 'Git URL is required').url('Invalid Git URL'),
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
            code: z.ZodIssueCode.custom,
            message: 'Key is required when value exists',
            path: [index, 'key'],
          })
        }
      })
    }),
})

const DEFAULT_FORM: FormState = {
  template: 'go-web-api',
  appName: '',
  gitUrl: TEMPLATE_GIT_REPOS['go-web-api'] ?? '',
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
  const [deploymentId, setDeploymentId] = useState<string | null>(null)
  const { data: deploymentStatus } = useDeploymentStatus(deploymentId)
  const terminalRef = useRef<HTMLDivElement>(null)
  const { data: appTemplatesRaw } = useAppTemplates()
  const appTemplates = (appTemplatesRaw ?? []).map((t) => ({
    id: t.id,
    name: t.name,
    description: t.description ?? t.runtime ?? '',
    language: t.language ?? t.runtime ?? '',
    color: '#6366f1',
  }))
  const { data: clustersData } = useClusters()
  const clusters = (clustersData?.items ?? []).map((c) => ({
    id: c.id,
    name: c.name,
    namespaces: ['default', 'production', 'staging'],
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

  const form = watch()

  const firstClusterId = clusters[0]?.id ?? ''
  useEffect(() => {
    if (firstClusterId && !form.clusterId) {
      setValue('clusterId', firstClusterId, { shouldValidate: true })
    }
  }, [firstClusterId, form.clusterId, setValue])

  useEffect(() => {
    const el = terminalRef.current
    if (el) el.scrollTop = el.scrollHeight
  })

  const createPipelineMutation = useCreatePipeline()
  const deployPipelineMutation = useDeployPipeline()

  const setField = (key: keyof FormState, value: FormState[keyof FormState]) => {
    setValue(key as never, value as never, { shouldValidate: true, shouldDirty: true })
  }

  const selectedCluster = clusters.find((c) => c.id === form.clusterId) ?? clusters[0] ?? { id: '', name: '', namespaces: ['default'] }

  const onSubmit = async (data: FormState) => {
    try {
      const pipeline = await createPipelineMutation.mutateAsync({
        name: data.appName,
        appType: 'backend',
        clusterId: data.clusterId,
        namespace: data.namespace,
        templateId: 'web-backend-v1',
      })
      const result = await deployPipelineMutation.mutateAsync(pipeline.id)
      setDeploymentId(result.deploymentId)
    } catch { /* react-query handles mutation errors */ }
  }

  const validateCurrentStep = async () => {
    if (step === 1) return trigger('appName')
    if (step === 2) return trigger('gitUrl')
    if (step === 3) return trigger(['clusterId', 'namespace'])
    if (step === 4) return trigger(['replicas', 'cpuLimit', 'memoryLimit'])
    return trigger('envVars')
  }

  const canNext: Record<Step, boolean> = {
    1: form.appName.trim().length >= 2,
    2: form.gitUrl.trim().length > 0 && !errors.gitUrl,
    3: !!form.clusterId && !!form.namespace,
    4: form.replicas >= 1 && !!form.cpuLimit && !!form.memoryLimit,
    5: !errors.envVars,
  }

  if (deploymentId) {
    const status = deploymentStatus?.status ?? 'running'
    const isComplete = status === 'success'
    const isFailed = status === 'failed'
    const isDone = isComplete || isFailed

    const apiSteps = deploymentStatus?.steps ?? []
    const hasSteps = apiSteps.length > 0
    const allLogs = apiSteps.flatMap((s) => s.logs ?? [])
    const deployedResources = allLogs
      .filter((line) => !line.startsWith('$') && !line.startsWith('error'))
      .map((line) => {
        const match = line.match(/^(\w+)\/(\S+)\s+(\w+)$/)
        return match ? { kind: match[1], name: match[2], action: match[3] } : null
      })
      .filter((r): r is { kind: string; name: string; action: string } => r !== null)

    const progressSteps: Array<{ label: string; status: string; message?: string }> = hasSteps
      ? [
          { label: '파이프라인 생성', status: 'success' },
          ...apiSteps.map((s) => ({ label: s.name, status: s.status, message: s.message })),
          { label: isComplete ? '배포 완료' : isFailed ? '배포 실패' : '검증 대기', status: isDone ? (isComplete ? 'success' : 'failed') : 'pending' },
        ]
      : [
          { label: '파이프라인 생성', status: 'success' },
          { label: '클러스터에 배포 중', status: isDone ? 'success' : 'running' },
          { label: isComplete ? '배포 완료' : isFailed ? '배포 실패' : '검증 대기', status: isDone ? (isComplete ? 'success' : 'failed') : 'pending' },
        ]

    return (
      <div>
        <Breadcrumb items={[{ label: 'CI/CD List', path: '/cicd/list' }, { label: '배포 진행' }]} />
        <div className="mx-auto max-w-lg py-12">
          <div className="mb-8 text-center">
            <h2 className="m-0 text-xl font-bold text-[var(--color-text-primary)]">{form.appName}</h2>
            <p className="m-0 mt-1 text-sm text-[var(--color-text-secondary)]">{form.namespace} 네임스페이스</p>
          </div>

          <div className="mb-8 flex flex-col gap-3">
            {progressSteps.map((s, i) => (
              <div key={`${s.label}-${i}`} className="flex items-center gap-3">
                <div className={cn(
                  'flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold',
                  s.status === 'success' ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]' :
                  s.status === 'failed' ? 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]' :
                  s.status === 'running' ? 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]' :
                  'bg-[rgba(100,116,139,0.1)] text-[var(--color-text-muted)]'
                )}>
                  {s.status === 'success' ? <Check size={14} /> :
                   s.status === 'failed' ? <X size={14} /> :
                   s.status === 'running' ? <Loader2 size={14} className="animate-spin" /> :
                   <span>{i + 1}</span>}
                </div>
                <div className="flex flex-col">
                  <span className={cn(
                    'text-sm',
                    s.status === 'success' || s.status === 'running' ? 'font-medium text-[var(--color-text-primary)]' :
                    s.status === 'failed' ? 'font-medium text-[#ef4444]' :
                    'text-[var(--color-text-muted)]'
                  )}>{s.label}</span>
                  {s.message && (
                    <code className="mt-0.5 text-xs text-[var(--color-text-muted)] font-mono">{s.message}</code>
                  )}
                </div>
              </div>
            ))}
          </div>

          <div className="mb-6 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[#0d1117]">
            <div className="flex items-center gap-2 border-b border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.03)] px-4 py-2">
              <div className="flex gap-1.5">
                <div className="h-2.5 w-2.5 rounded-full bg-[#ff5f57]" />
                <div className="h-2.5 w-2.5 rounded-full bg-[#febc2e]" />
                <div className="h-2.5 w-2.5 rounded-full bg-[#28c840]" />
              </div>
              <span className="text-[11px] font-medium text-[rgba(255,255,255,0.4)]">Deploy Output</span>
            </div>
            <div ref={terminalRef} className="max-h-[240px] overflow-y-auto p-4">
              {allLogs.length === 0 ? (
                <span className="font-mono text-xs text-[#484f58]">Waiting for deployment output...</span>
              ) : (
                <pre className="m-0 whitespace-pre-wrap font-mono text-xs leading-5">
                  {allLogs.map((line, i) => (
                    <div key={`log-${i}`} className={
                      line.startsWith('$') ? 'text-[#58a6ff]' :
                      line.includes('created') ? 'text-[#3fb950]' :
                      line.includes('configured') ? 'text-[#d29922]' :
                      line.includes('error') || line.includes('failed') ? 'text-[#f85149]' :
                      'text-[#c9d1d9]'
                    }>{line}</div>
                  ))}
                </pre>
              )}
            </div>
          </div>

          {deploymentStatus?.startedAt && (
            <div className="mb-6 text-center text-xs text-[var(--color-text-muted)]">
              시작: {new Date(deploymentStatus.startedAt).toLocaleTimeString('ko-KR')}
              {deploymentStatus.completedAt && ` · 완료: ${new Date(deploymentStatus.completedAt).toLocaleTimeString('ko-KR')}`}
            </div>
          )}

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

      {/* Template selection */}
      <div className="mb-7">
        <p className="mb-3 mt-0 text-[13px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
          앱 템플릿
        </p>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2.5">
          {appTemplates.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => {
                setField('template', t.id)
                const repoUrl = TEMPLATE_GIT_REPOS[t.id]
                if (repoUrl) setField('gitUrl', repoUrl)
              }}
              className={cn(
                'cursor-pointer rounded-[10px] border p-[14px] text-left transition-all duration-150',
                form.template === t.id
                  ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.15)]'
                  : 'border-[var(--color-border-default)] bg-[var(--color-surface-card)]'
              )}
            >
              <div
                className="mb-2 h-2 w-2 rounded-full"
                style={{ backgroundColor: t.color }}
              />
              <p className="mb-1 mt-0 text-[13px] font-bold text-[var(--color-text-primary)]">
                {t.name}
              </p>
              <p className="m-0 text-[11px] text-[var(--color-text-secondary)]">
                {t.language}
              </p>
            </button>
          ))}
        </div>
      </div>

      {/* Step indicator */}
      <div className="mb-6 flex flex-wrap items-center gap-1">
        {([1, 2, 3, 4, 5] as Step[]).map((s, i) => (
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
            {i < 4 && <ChevronRight size={14} className="shrink-0 text-[var(--color-text-secondary)]" />}
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
              <Input
                placeholder="https://github.com/org/repo.git"
                value={form.gitUrl}
                onChange={(e) => setField('gitUrl', e.target.value)}
              />
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
                      const cl = clusters.find((c) => c.id === e.target.value)
                      if (cl?.namespaces[0]) setField('namespace', cl.namespaces[0])
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
                    {selectedCluster.namespaces.map((ns) => (
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

          {/* Navigation */}
          <div className="mt-6 flex justify-end gap-2.5">
            {step > 1 && (
              <Button variant="outline" size="md" onClick={() => setStep((s) => (s - 1) as Step)}>
                이전
              </Button>
            )}
            {step < 5 ? (
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
                onClick={handleSubmit(onSubmit)}
              >
                <Rocket size={14} />
                Deploy
              </Button>
            )}
          </div>
        </div>

        {/* YAML preview */}
        <div>
          <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            YAML 매니페스트 미리보기
          </p>
          <CodePreview
            code={generateYaml(form)}
            language="yaml"
            title={`${form.appName || 'my-app'}.yaml`}
            maxHeight="600px"
          />
        </div>
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
  const sliderId = `resource-${label.toLowerCase().replace(/\s+/g, '-')}`
  return (
    <div>
      <div className="mb-1.5 flex justify-between">
        <label htmlFor={sliderId} className={cn(labelStyleClass, 'mb-0')}>{label}</label>
        <span
          className="font-mono text-[13px] font-semibold text-[#a5b4fc]"
        >
          {value}
        </span>
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={options.length - 1}
        value={idx >= 0 ? idx : 0}
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
