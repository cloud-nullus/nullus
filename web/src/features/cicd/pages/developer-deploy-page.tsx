import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
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

import { useCicdTemplates, useCreatePipeline, useDeployPipeline } from '../api/cicd-api'
import type { AppType } from '../api/cicd-api'
import { useClusterNamespaces, useClusters } from '../../admin/api/admin-api'
import { useStacks } from '../../stack/api/stack-api'
import { cn } from '../../../lib/utils'
import { useCicdDeployLog, type CicdLogLevel } from '../hooks/use-cicd-deploy-log'
import { formatTime, resolveLocale } from '../../../lib/locale'

type Step = 1 | 2 | 3 | 4 | 5 | 6

const STEP_LABEL_DEFAULTS: Record<Step, string> = {
  1: 'App Name',
  2: 'Git Repository',
  3: 'Cluster / Namespace',
  4: 'Resource Configuration',
  5: 'Environment Variables',
  6: 'Manifest Review',
}

const PROGRESS_SEGMENTS = Array.from({ length: 100 }, (_, i) => i + 1)

const LOG_LEVEL_STYLE: Record<CicdLogLevel, string> = {
  info: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  success: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  error: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
}

function PhaseStep({ label, index, progress, total }: { label: string; index: number; progress: number; total: number }) {
  const phaseProgress = 100 / total
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
      {index < total - 1 && (
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

/** YAML 값에 안전하지 않은 문자가 포함되면 따옴표로 감싼다 */
function yamlSafe(value: string): string {
  if (/[:\n\r#"'\\{}\[\],&*?|><!%@`]/.test(value) || value !== value.trim()) {
    return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"`
  }
  return value
}

function generateYaml(form: Partial<FormState>): string {
  const name = yamlSafe(form.appName ?? 'my-app')
  const ns = yamlSafe(form.namespace ?? 'default')
  const tpl = yamlSafe(form.template ?? 'react-spa')
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  const envLines = (form.envVars ?? [])
    .filter((e) => e.key)
    .map((e) => `            - name: ${yamlSafe(e.key)}\n              value: ${yamlSafe(e.value)}`)
    .join('\n')

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    app: ${name}
    template: ${tpl}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
        - name: ${name}
          image: harbor.nullus.io/${name}:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${envLines ? `          env:\n${envLines}` : ''}
---
apiVersion: v1
kind: Service
metadata:
  name: ${name}-svc
  namespace: ${ns}
spec:
  selector:
    app: ${name}
  ports:
    - port: 80
      targetPort: 8080`
}

interface EnvVar { key: string; value: string }

type AppTemplate = 'react-spa' | 'next-app' | 'express-api' | 'spring-boot' | 'python-fastapi' | 'go-web-api'

interface FormState {
  appName: string
  gitUrl: string
  dockerfilePath: string
  dockerContext: string
  template: AppTemplate
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
  dockerfilePath: z.string(),
  dockerContext: z.string(),
  template: z.enum(['react-spa', 'next-app', 'express-api', 'spring-boot', 'python-fastapi', 'go-web-api'] as const),
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
  dockerfilePath: '',
  dockerContext: '',
  template: 'react-spa',
  clusterId: '',
  namespace: 'default',
  replicas: 2,
  cpuRequest: '100m',
  cpuLimit: '500m',
  memoryRequest: '128Mi',
  memoryLimit: '512Mi',
  envVars: [],
}

export function DeveloperDeployPage() {
  const { t, i18n } = useTranslation()
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language)
  const [step, setStep] = useState<Step>(1)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const appTypeParam = (searchParams.get('appType') ?? 'backend') as AppType
  const [deploymentId, setDeploymentId] = useState<string | null>(null)
  const [customManifest, setCustomManifest] = useState<string | null>(null)
  const [selectedStackId, setSelectedStackId] = useState('')
  const [repoName, setRepoName] = useState('')
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [selectedAppType, setSelectedAppType] = useState<AppType>(appTypeParam)
  const [createNewNamespace, setCreateNewNamespace] = useState(false)
  const { logs, status, progress, isConnected } = useCicdDeployLog(deploymentId)
  const terminalRef = useRef<HTMLDivElement>(null)
  const { data: stacksData } = useStacks()
  const stacks = (stacksData?.items ?? []).map((s) => ({
    id: s.id,
    name: s.name,
  }))
  const { data: clustersData } = useClusters()
  const { data: templatesData } = useCicdTemplates()
  const templates = templatesData ?? []
  const quickStartTemplates = templates.filter((template) =>
    !!(template.gitRepoUrl?.trim() || template.dockerfilePath?.trim() || template.dockerContext?.trim())
  )
  const clusters = (clustersData?.items ?? [])
    .filter((cluster) => {
      const rawTypes = Array.isArray(cluster.types) && cluster.types.length > 0
        ? cluster.types
        : (cluster.type ? [cluster.type] : [])
      const normalizedTypes = rawTypes
        .flatMap((type) => type.split(','))
        .map((type) => type.trim().toLowerCase())
        .filter((type) => type.length > 0)
      return normalizedTypes.includes('target')
    })
    .map((c) => ({
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
  const templateIdParam = searchParams.get('template') ?? ''

  const form = watch()
  const { data: namespacesData } = useClusterNamespaces(form.clusterId)
  const namespaces = useMemo(() => (namespacesData ?? []).map((ns) => ns.name), [namespacesData])
  const namespaceOptions = useMemo(
    () => Array.from(new Set(['default', ...namespaces.filter((ns) => ns && ns !== 'default')])),
    [namespaces]
  )
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
      setCreateNewNamespace(!namespaceOptions.includes(namespaceParam))
    }
    if (appNameParam) {
      setValue('appName', appNameParam, { shouldValidate: true })
    }
  }, [appNameParam, clusterIdParam, namespaceOptions, namespaceParam, pipelineIdParam, setValue])

  useEffect(() => {
    if (!templateIdParam || templates.length === 0) {
      return
    }
    if (selectedTemplateId === templateIdParam) {
      return
    }

    const template = templates.find((item) => item.id === templateIdParam)
    if (!template) {
      return
    }

    const suggestedAppName = template.id.replace(/-v\d+$/, '').replace(/^nullus-/, '')
    setSelectedTemplateId(template.id)
    setSelectedAppType(template.appType)
    setSelectedStackId('')
    setRepoName('')
    setValue('appName', suggestedAppName, { shouldValidate: true, shouldDirty: true })
    setValue('gitUrl', template.gitRepoUrl ?? '', { shouldValidate: true, shouldDirty: true })
    setValue('dockerfilePath', template.dockerfilePath ?? '', { shouldValidate: true, shouldDirty: true })
    setValue('dockerContext', template.dockerContext ?? '', { shouldValidate: true, shouldDirty: true })
    if (template.envVars && Object.keys(template.envVars).length > 0) {
      const envArray = Object.entries(template.envVars).map(([key, value]) => ({ key, value }))
      setValue('envVars', [...envArray, { key: '', value: '' }], { shouldValidate: true, shouldDirty: true })
    }
    setStep(3)
  }, [selectedTemplateId, setValue, templateIdParam, templates])

  const firstNamespace = namespaceOptions[0] ?? 'default'
  useEffect(() => {
    if (createNewNamespace) {
      return
    }
    if (form.namespace && namespaceOptions.includes(form.namespace)) {
      return
    }
    setValue('namespace', firstNamespace, { shouldValidate: true })
  }, [createNewNamespace, firstNamespace, form.namespace, namespaceOptions, setValue])

  const createPipelineMutation = useCreatePipeline()
  const deployPipelineMutation = useDeployPipeline()

  const setField = (key: keyof FormState, value: FormState[keyof FormState]) => {
    setValue(key as never, value as never, { shouldValidate: true, shouldDirty: true })
  }

  const selectedCluster = clusters.find((c) => c.id === form.clusterId) ?? clusters[0] ?? { id: '', name: '' }
  const hasBuildPipeline = form.dockerfilePath.trim() !== ''
  const phaseLabels = hasBuildPipeline
    ? [
        t('developerDeployPage.phases.gitClone', 'Git Clone'),
        t('developerDeployPage.phases.dockerBuild', 'Docker Build'),
        t('developerDeployPage.phases.imageLoad', 'Image Load'),
        t('developerDeployPage.phases.namespace', 'Create Namespace'),
        t('developerDeployPage.phases.deployment', 'Create Deployment'),
        t('developerDeployPage.phases.service', 'Create Service'),
      ]
    : [
        t('developerDeployPage.phases.namespace', 'Create Namespace'),
        t('developerDeployPage.phases.deployment', 'Create Deployment'),
        t('developerDeployPage.phases.service', 'Create Service'),
      ]

  const onSubmit = async (data: FormState) => {
    try {
      const envVarsMap: Record<string, string> = {}
      data.envVars.forEach(({ key, value }) => {
        if (key.trim()) envVarsMap[key.trim()] = value
      })

      const pipeline = await createPipelineMutation.mutateAsync({
        name: data.appName,
        appType: selectedAppType,
        clusterId: data.clusterId,
        namespace: data.namespace,
        templateId: selectedTemplateId || undefined,
        gitRepoUrl: data.gitUrl,
        dockerfilePath: data.dockerfilePath,
        dockerContext: data.dockerContext,
        envVars: envVarsMap,
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

    const parsedResources = logs
      .map((entry) => entry.message)
      .filter((line) => !line.startsWith('$') && !line.startsWith('error'))
      .map((line) => {
        const match = line.match(/^(\w+)\/(\S+)\s+(\w+)$/)
        return match ? { kind: match[1], name: match[2], action: match[3] } : null
      })
      .filter((r): r is { kind: string; name: string; action: string } => r !== null)
    const deployedResources = Array.from(
      parsedResources.reduce((acc, item) => {
        const key = `${item.kind}/${item.name}`
        const prev = acc.get(key)
        if (!prev || (prev.action !== 'created' && item.action === 'created')) {
          acc.set(key, item)
        }
        return acc
      }, new Map<string, { kind: string; name: string; action: string }>()).values()
    )

    return (
      <div>
        <Breadcrumb items={[{ label: t('sidebar.cicdList', 'CI/CD List'), path: '/cicd/list' }, { label: t('developerDeployPage.deployProgress', 'Deploy Progress') }]} />
        <div className="mx-auto max-w-3xl py-12">
          <div className="mb-8 text-center">
            <h2 className="m-0 text-xl font-bold text-[var(--color-text-primary)]">{form.appName}</h2>
            <p className="m-0 mt-1 text-sm text-[var(--color-text-secondary)]">{form.namespace} {t('developerDeployPage.namespaceSuffix', 'namespace')}</p>
          </div>

          <div className="mb-8 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-6">
            <div className="mb-5 flex items-center gap-2">
              {phaseLabels.map((phase, idx) => (
                <PhaseStep key={phase} label={phase} index={idx} progress={progress} total={phaseLabels.length} />
              ))}
            </div>

            <div className="mb-1 flex justify-between text-xs">
              <span className="text-[var(--color-text-secondary)]">{t('developerDeployPage.totalProgress', 'Total Progress')}</span>
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
              {t('developerDeployPage.deploymentId', 'Deployment ID')}: <span className="font-mono text-[var(--color-text-primary)]">{deploymentId}</span>
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
                {isConnected ? t('developerDeployPage.streaming', 'Streaming...') : t('developerDeployPage.connecting', 'Connecting...')}
              </span>
            </div>
            <div ref={terminalRef} className="max-h-[400px] overflow-y-auto p-4">
              {logs.length === 0 ? (
                <span className="font-mono text-xs text-[#484f58]">
                  {isFailed ? t('developerDeployPage.deployFailedNoLog', 'Deployment failed. Logs are unavailable.') : t('developerDeployPage.waitingLogs', 'Waiting for deployment output...')}
                </span>
              ) : (
                <div className="flex flex-col gap-1.5">
                  {logs.map((entry) => (
                    <div key={entry.id} className="flex gap-2 leading-5">
                      <span className="shrink-0 text-xs text-[#484f58]">
                        {formatTime(entry.timestamp, locale)}
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
                  {t('developerDeployPage.createdResources', 'Created resources')}
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
                {t('developerDeployPage.newDeployment', 'New Deployment')}
              </Button>
              <Button variant="primary" size="md" onClick={() => navigate('/cicd/list')}>
                {t('developerDeployPage.viewCicdList', 'View CI/CD List')}
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
          { label: t('sidebar.cicdList', 'CI/CD List'), path: '/cicd/list' },
          { label: t('developerDeployPage.title', 'Pipeline Setup & Deploy') },
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
            {t('developerDeployPage.title', 'CI/CD Pipeline Setup & Developer Deploy')}
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            {t('developerDeployPage.description', 'Proceed with pipeline template selection and developer deployment on a single screen.')}
          </p>
        </div>
      </div>

      {quickStartTemplates.length > 0 && (
        <div className="mb-6 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <p className="mb-2 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            {t('developerDeployPage.quickStart.title', 'Quick Start — Select a Template')}
          </p>
          <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
            {quickStartTemplates.map((template) => (
              <button
                key={template.id}
                type="button"
                onClick={() => {
                  const suggestedAppName = template.id.replace(/-v\d+$/, '').replace(/^nullus-/, '')
                  setSelectedTemplateId(template.id)
                  setSelectedAppType(template.appType)
                  setSelectedStackId('')
                  setRepoName('')
                  setValue('appName', suggestedAppName, { shouldValidate: true, shouldDirty: true })
                  setValue('gitUrl', template.gitRepoUrl ?? '', { shouldValidate: true, shouldDirty: true })
                  setValue('dockerfilePath', template.dockerfilePath ?? '', { shouldValidate: true, shouldDirty: true })
                  setValue('dockerContext', template.dockerContext ?? '', { shouldValidate: true, shouldDirty: true })
                  if (template.envVars && Object.keys(template.envVars).length > 0) {
                    const envArray = Object.entries(template.envVars).map(([key, value]) => ({ key, value }))
                    setValue('envVars', [...envArray, { key: '', value: '' }], { shouldValidate: true, shouldDirty: true })
                  }
                  setStep(3)
                }}
                className={cn(
                  'rounded-lg border bg-[rgba(255,255,255,0.03)] px-4 py-3 text-left transition-colors hover:border-[#6366f1] hover:bg-[rgba(99,102,241,0.05)]',
                  selectedTemplateId === template.id
                    ? 'border-[#6366f1] bg-[rgba(99,102,241,0.08)]'
                    : 'border-[var(--color-border-default)]'
                )}
              >
                <span className="block text-sm font-medium text-[var(--color-text-primary)]">{template.name}</span>
                <span className="mt-1 block text-xs text-[var(--color-text-secondary)]">{template.description}</span>
              </button>
            ))}
          </div>
        </div>
      )}

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
                {t(`developerDeployPage.steps.${s}`, STEP_LABEL_DEFAULTS[s])}
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
            <StepSection title={t('developerDeployPage.sections.appName', 'Enter App Name')}>
              <Input
                placeholder="my-awesome-app"
                value={form.appName}
                onChange={(e) => setField('appName', e.target.value)}
              />
              {errors.appName && <span className="text-xs text-[#ef4444]">{errors.appName.message}</span>}
              <p className="mb-0 mt-1.5 text-xs text-[var(--color-text-secondary)]">
                {t('developerDeployPage.appNameRule', 'Only lowercase letters, numbers, and hyphens are allowed.')}
              </p>
            </StepSection>
          )}

          {step === 2 && (
            <StepSection title={t('developerDeployPage.sections.gitRepository', 'Git Repository URL')}>
              <div className="flex flex-col gap-3">
                <div>
                  <label htmlFor="deploy-stack" className={labelStyleClass}>{t('developerDeployPage.form.stackOptional', 'Stack (Optional)')}</label>
                  <NativeSelect
                    id="deploy-stack"
                    value={selectedStackId}
                    onChange={(e) => setSelectedStackId(e.target.value)}
                    className="w-full"
                  >
                    <option value="">{t('developerDeployPage.form.manualInput', 'Manual Input')}</option>
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
              <div className="mt-4 rounded-lg border border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.02)] p-4">
                <p className="mb-3 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                  {t('developerDeployPage.form.buildConfig', 'Build Configuration (Optional)')}
                </p>
                <div className="flex flex-col gap-3">
                  <div>
                    <label htmlFor="deploy-dockerfile" className={labelStyleClass}>
                      {t('developerDeployPage.form.dockerfilePath', 'Dockerfile Path')}
                    </label>
                    <Input
                      id="deploy-dockerfile"
                      placeholder="backend/Dockerfile"
                      value={form.dockerfilePath}
                      onChange={(e) => setField('dockerfilePath', e.target.value)}
                    />
                    <p className="mb-0 mt-1 text-[11px] text-[var(--color-text-muted)]">
                      {t('developerDeployPage.form.dockerfileHint', 'Relative path to Dockerfile in the repository. Leave empty to use a default base image.')}
                    </p>
                  </div>
                  <div>
                    <label htmlFor="deploy-context" className={labelStyleClass}>
                      {t('developerDeployPage.form.dockerContext', 'Docker Build Context')}
                    </label>
                    <Input
                      id="deploy-context"
                      placeholder="backend/"
                      value={form.dockerContext}
                      onChange={(e) => setField('dockerContext', e.target.value)}
                    />
                    <p className="mb-0 mt-1 text-[11px] text-[var(--color-text-muted)]">
                      {t('developerDeployPage.form.contextHint', 'Directory to use as Docker build context. Defaults to repository root.')}
                    </p>
                  </div>
                </div>
              </div>
            </StepSection>
          )}

          {step === 3 && (
            <StepSection title={t('developerDeployPage.sections.clusterNamespace', 'Cluster & Namespace')}>
              <div className="flex flex-col gap-3">
                <div>
                  <label htmlFor="deploy-cluster" className={labelStyleClass}>{t('developerDeployPage.form.cluster', 'Cluster')}</label>
                  <NativeSelect
                    id="deploy-cluster"
                    value={form.clusterId}
                    onChange={(e) => {
                      setField('clusterId', e.target.value)
                      setCreateNewNamespace(false)
                      setField('namespace', 'default')
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
                  <label htmlFor="deploy-namespace" className={labelStyleClass}>{t('developerDeployPage.form.namespace', 'Namespace')}</label>
                  <NativeSelect
                    id="deploy-namespace"
                    value={createNewNamespace ? '__new__' : (form.namespace || 'default')}
                    onChange={(e) => {
                      if (e.target.value === '__new__') {
                        setCreateNewNamespace(true)
                        setField('namespace', '')
                        return
                      }
                      setCreateNewNamespace(false)
                      setField('namespace', e.target.value)
                    }}
                    className="w-full"
                  >
                    {namespaceOptions.map((ns) => (
                      <option key={ns} value={ns}>{ns}</option>
                    ))}
                    <option value="__new__">{t('developerDeployPage.form.newNamespace', 'New Namespace')}</option>
                  </NativeSelect>
                  {createNewNamespace && (
                    <Input
                      className="mt-2"
                      placeholder={t('developerDeployPage.form.newNamespacePlaceholder', 'my-namespace')}
                      value={form.namespace}
                      onChange={(e) => setField('namespace', e.target.value)}
                    />
                  )}
                  {errors.namespace && <span className="text-xs text-[#ef4444]">{errors.namespace.message}</span>}
                </div>
              </div>
            </StepSection>
          )}

          {step === 4 && (
            <StepSection title={t('developerDeployPage.sections.resources', 'Resource Configuration')}>
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
            <StepSection title={t('developerDeployPage.sections.envVars', 'Environment Variables')}>
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
                  {t('developerDeployPage.actions.addVariable', 'Add Variable')}
                </Button>
              </div>
            </StepSection>
          )}

          {step === 6 && (
            <StepSection title={t('developerDeployPage.sections.manifest', 'Review and Edit Manifest')}>
              <p className="mb-3 text-xs text-[var(--color-text-secondary)]">
                {t('developerDeployPage.manifestDescription', 'Review the generated YAML manifest and edit if needed.')}
              </p>
              <textarea
                value={customManifest ?? generateYaml(form)}
                onChange={(e) => setCustomManifest(e.target.value)}
                className="h-[400px] w-full resize-none rounded-lg border border-[var(--color-border-default)] bg-[#0d1117] p-4 font-mono text-xs leading-5 text-[#c9d1d9] focus:outline-none focus:ring-1 focus:ring-[#6366f1]"
                spellCheck={false}
              />
              <button type="button" onClick={() => setCustomManifest(null)} className="mt-2 cursor-pointer border-none bg-none text-xs text-[var(--color-text-secondary)] underline">
                {t('developerDeployPage.actions.resetDefault', 'Reset to default')}
              </button>
            </StepSection>
          )}

          {/* Navigation */}
          <div className="mt-6 flex justify-end gap-2.5">
            {step > 1 && (
              <Button variant="outline" size="md" onClick={() => setStep((s) => (s - 1) as Step)}>
                {t('developerDeployPage.actions.previous', 'Previous')}
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
                {t('developerDeployPage.actions.next', 'Next')}
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
                {t('developerDeployPage.actions.deploy', 'Deploy')}
              </Button>
            )}
          </div>
        </div>

        {/* YAML preview */}
        {step !== 6 && (
        <div>
          <p className="mb-2.5 mt-0 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            {t('developerDeployPage.yamlPreview', 'YAML Manifest Preview')}
          </p>
          <CodePreview
            code={generateYaml(form)}
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
