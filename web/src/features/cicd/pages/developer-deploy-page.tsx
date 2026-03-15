import { useState } from 'react'
import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Rocket, Plus, Trash2, ChevronRight } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { CodePreview } from '../../../components/shared/code-preview'
import { useAuthStore } from '../../../stores/auth-store'
import { useAppTemplates, useDeployApp } from '../api/cicd-api'
import { useClusters } from '../../admin/api/admin-api'
import type { AppTemplate, DeployAppRequest } from '../api/cicd-api'
import { cn } from '../../../lib/utils'

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
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${form.appName ?? 'my-app'}
  namespace: ${form.namespace ?? 'default'}
  labels:
    app: ${form.appName ?? 'my-app'}
    template: ${form.template ?? 'react-spa'}
spec:
  replicas: 2
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
  template: AppTemplate
  appName: string
  gitUrl: string
  clusterId: string
  namespace: string
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  envVars: EnvVar[]
}

const deploySchema = z.object({
  template: z.enum(['react-spa', 'next-app', 'express-api', 'spring-boot', 'python-fastapi']),
  appName: z.string().min(2, 'App name must be at least 2 characters').max(50, 'App name must be 50 characters or less'),
  gitUrl: z.string().min(1, 'Git URL is required').url('Invalid Git URL'),
  clusterId: z.string().min(1, 'Cluster is required'),
  namespace: z.string().min(1, 'Namespace is required'),
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
  template: 'react-spa',
  appName: '',
  gitUrl: '',
  clusterId: 'c1',
  namespace: 'default',
  cpuRequest: '100m',
  cpuLimit: '500m',
  memoryRequest: '128Mi',
  memoryLimit: '512Mi',
  envVars: [{ key: '', value: '' }],
}

export function DeveloperDeployPage() {
  const role = useAuthStore((s) => s.role)
  const [step, setStep] = useState<Step>(1)
  const [deployed, setDeployed] = useState(false)
  const { data: appTemplatesRaw } = useAppTemplates()
  const appTemplates = (appTemplatesRaw ?? []).map((t) => ({
    id: t.id,
    name: t.name,
    description: t.description ?? '',
    language: t.language ?? '',
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

  const deployMutation = useDeployApp()

  if (role !== 'developer') {
    return (
      <div className="flex h-[300px] flex-col items-center justify-center gap-3 text-[var(--color-text-secondary)]">
        <Rocket size={40} className="opacity-30" />
        <p className="m-0 text-[15px]">이 페이지는 Developer 역할 전용입니다.</p>
      </div>
    )
  }

  const setField = (key: keyof FormState, value: FormState[keyof FormState]) => {
    setValue(key as never, value as never, { shouldValidate: true, shouldDirty: true })
  }

  const selectedCluster = clusters.find((c) => c.id === form.clusterId) ?? clusters[0] ?? { id: '', name: '', namespaces: ['default'] }

  const onSubmit = (data: FormState) => {
    const request: DeployAppRequest = {
      appName: data.appName,
      gitUrl: data.gitUrl,
      clusterId: data.clusterId,
      namespace: data.namespace,
      template: data.template,
      resources: {
        cpuRequest: data.cpuRequest,
        cpuLimit: data.cpuLimit,
        memoryRequest: data.memoryRequest,
        memoryLimit: data.memoryLimit,
      },
      envVars: data.envVars.filter((e) => e.key),
    }
    deployMutation.mutate(request, {
      onSuccess: () => setDeployed(true),
    })
  }

  const validateCurrentStep = async () => {
    if (step === 1) return trigger('appName')
    if (step === 2) return trigger('gitUrl')
    if (step === 3) return trigger(['clusterId', 'namespace'])
    if (step === 4) return trigger(['cpuLimit', 'memoryLimit'])
    return trigger('envVars')
  }

  const canNext: Record<Step, boolean> = {
    1: form.appName.trim().length >= 2,
    2: form.gitUrl.trim().length > 0 && !errors.gitUrl,
    3: !!form.clusterId && !!form.namespace,
    4: !!form.cpuLimit && !!form.memoryLimit,
    5: !errors.envVars,
  }

  if (deployed) {
    return (
      <div className="flex h-[360px] flex-col items-center justify-center gap-4">
        <div
          className="flex h-16 w-16 items-center justify-center rounded-full bg-[rgba(34,197,94,0.15)] text-[#22c55e]"
        >
          <Rocket size={28} />
        </div>
        <h2 className="m-0 text-xl font-extrabold text-[var(--color-text-primary)]">
          배포 요청 완료!
        </h2>
        <p className="m-0 text-sm text-[var(--color-text-secondary)]">
          {form.appName} 앱이 {form.namespace} 네임스페이스에 배포 요청되었습니다.
        </p>
        <Button
          variant="outline"
          size="md"
          onClick={() => {
            setDeployed(false)
            reset(DEFAULT_FORM)
            setStep(1)
          }}
        >
          새 배포
        </Button>
      </div>
    )
  }

  return (
    <div>
      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div
          className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
        >
          <Rocket size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Developer Self-Service 배포
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            앱 템플릿을 선택하고 배포 위자드를 따라 배포하세요.
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
              onClick={() => setField('template', t.id)}
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
                  <select
                    id="deploy-cluster"
                    value={form.clusterId}
                    onChange={(e) => {
                      setField('clusterId', e.target.value)
                      const cl = clusters.find((c) => c.id === e.target.value)
                      if (cl?.namespaces[0]) setField('namespace', cl.namespaces[0])
                    }}
                    className={selectStyleClass}
                  >
                    {clusters.map((c) => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                  {errors.clusterId && <span className="text-xs text-[#ef4444]">{errors.clusterId.message}</span>}
                </div>
                <div>
                  <label htmlFor="deploy-namespace" className={labelStyleClass}>네임스페이스</label>
                  <select
                    id="deploy-namespace"
                    value={form.namespace}
                    onChange={(e) => setField('namespace', e.target.value)}
                    className={selectStyleClass}
                  >
                    {selectedCluster.namespaces.map((ns) => (
                      <option key={ns} value={ns}>{ns}</option>
                    ))}
                  </select>
                  {errors.namespace && <span className="text-xs text-[#ef4444]">{errors.namespace.message}</span>}
                </div>
              </div>
            </StepSection>
          )}

          {step === 4 && (
            <StepSection title="리소스 설정">
              <div className="flex flex-col gap-4">
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
                loading={deployMutation.isPending}
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

const labelStyleClass = 'mb-1.5 block text-xs font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]'

const selectStyleClass =
  'w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'
