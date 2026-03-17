import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Boxes, FileCode2, FileText, GitBranch, Rocket, Server, Settings2 } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { YamlEditor } from '../../../components/shared/yaml-editor'
import { useCicdTemplates, useCreatePipeline } from '../api/cicd-api'
import { useClusters } from '../../admin/api/admin-api'
import type { CicdTemplate } from '../api/cicd-api'
import { cn } from '../../../lib/utils'

type SetupTab = 'cluster' | 'build' | 'deploy' | 'yaml'
type DeployMode = 'template' | 'custom'

interface DockerfilePreset {
  id: string
  label: string
  path: string
  content: string
}

interface DeployYamlPreset {
  id: string
  label: string
  description: string
  content: string
}

const TABS: { id: SetupTab; label: string; icon: typeof Server }[] = [
  { id: 'cluster', label: 'Cluster', icon: Server },
  { id: 'build', label: 'Build', icon: FileCode2 },
  { id: 'deploy', label: 'Deploy', icon: Boxes },
  { id: 'yaml', label: 'Pipeline YAML', icon: FileText },
]

const DEFAULT_TEMPLATES: CicdTemplate[] = [
  {
    id: 'web-frontend',
    name: 'Web Frontend',
    description: 'React/Next.js 웹 프론트엔드 앱 템플릿',
    appType: 'web-frontend',
    stages: ['Build', 'Test', 'Docker Build', 'ArgoCD Deploy'],
    createdBy: 'admin',
  },
  {
    id: 'web-backend',
    name: 'Backend API',
    description: 'REST API 백엔드 서비스 템플릿',
    appType: 'web-backend',
    stages: ['Build', 'Test', 'Security', 'Docker Build', 'ArgoCD Deploy'],
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

const DOCKERFILE_PRESETS: DockerfilePreset[] = [
  {
    id: 'dockerfile.root',
    label: 'Dockerfile (root)',
    path: './Dockerfile',
    content: [
      'FROM node:20-alpine AS builder',
      'WORKDIR /app',
      'COPY package*.json ./',
      'RUN npm ci',
      'COPY . .',
      'RUN npm run build',
      '',
      'FROM nginx:1.27-alpine',
      'COPY --from=builder /app/dist /usr/share/nginx/html',
      'EXPOSE 80',
      'CMD ["nginx", "-g", "daemon off;"]',
    ].join('\n'),
  },
  {
    id: 'dockerfile.app',
    label: 'Dockerfile (app/)',
    path: './app/Dockerfile',
    content: [
      'FROM golang:1.24-alpine AS builder',
      'WORKDIR /src',
      'COPY go.mod go.sum ./',
      'RUN go mod download',
      'COPY . .',
      'RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/server',
      '',
      'FROM gcr.io/distroless/static:nonroot',
      'COPY --from=builder /src/app /app',
      'USER nonroot:nonroot',
      'ENTRYPOINT ["/app"]',
    ].join('\n'),
  },
  {
    id: 'dockerfile.service',
    label: 'Dockerfile (services/api/)',
    path: './services/api/Dockerfile',
    content: [
      'FROM python:3.12-slim',
      'WORKDIR /service',
      'COPY requirements.txt .',
      'RUN pip install --no-cache-dir -r requirements.txt',
      'COPY . .',
      'EXPOSE 8080',
      'CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8080"]',
    ].join('\n'),
  },
]

const DEPLOY_YAML_PRESETS: DeployYamlPreset[] = [
  {
    id: 'k8s-deployment',
    label: 'Kubernetes Deployment',
    description: 'Deployment + Service 기본 매니페스트',
    content: [
      'apiVersion: apps/v1',
      'kind: Deployment',
      'metadata:',
      '  name: app-placeholder',
      'spec:',
      '  replicas: 2',
      '  selector:',
      '    matchLabels:',
      '      app: app-placeholder',
      '  template:',
      '    metadata:',
      '      labels:',
      '        app: app-placeholder',
      '    spec:',
      '      containers:',
      '        - name: app',
      '          image: harbor.local/app-placeholder:latest',
      '          ports:',
      '            - containerPort: 8080',
      '---',
      'apiVersion: v1',
      'kind: Service',
      'metadata:',
      '  name: app-placeholder-svc',
      'spec:',
      '  selector:',
      '    app: app-placeholder',
      '  ports:',
      '    - port: 80',
      '      targetPort: 8080',
      '  type: ClusterIP',
    ].join('\n'),
  },
  {
    id: 'k8s-cronjob',
    label: 'Kubernetes CronJob',
    description: '배치/스케줄 작업용 CronJob 매니페스트',
    content: [
      'apiVersion: batch/v1',
      'kind: CronJob',
      'metadata:',
      '  name: batch-placeholder',
      'spec:',
      '  schedule: "*/10 * * * *"',
      '  successfulJobsHistoryLimit: 3',
      '  failedJobsHistoryLimit: 1',
      '  jobTemplate:',
      '    spec:',
      '      template:',
      '        spec:',
      '          restartPolicy: OnFailure',
      '          containers:',
      '            - name: batch',
      '              image: harbor.local/batch-placeholder:latest',
    ].join('\n'),
  },
  {
    id: 'kustomize',
    label: 'Kustomize Base',
    description: 'Kustomization 기반 배포 구성',
    content: [
      'apiVersion: kustomize.config.k8s.io/v1beta1',
      'kind: Kustomization',
      'namespace: default',
      'resources:',
      '  - deployment.yaml',
      '  - service.yaml',
      'images:',
      '  - name: harbor.local/app-placeholder',
      '    newTag: latest',
    ].join('\n'),
  },
]

const appTypeOptionClassName =
  'w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]'

const getPipelineYaml = (input: {
  pipelineName: string
  appType: AppType
  clusterName: string
  templateName: string
  dockerfilePath: string
  deployYamlPath: string
  deployMode: DeployMode
  deployYamlContent: string
}): string => {
  const stageBlock = input.appType === 'batch-job'
    ? ['  - build', '  - test', '  - docker', '  - deploy-cronjob']
    : ['  - build', '  - test', '  - docker', '  - deploy']

  return [
    'pipeline:',
    `  name: ${input.pipelineName || 'new-pipeline'}`,
    `  template: ${input.templateName}`,
    `  appType: ${input.appType}`,
    `  cluster: ${input.clusterName}`,
    '  stages:',
    ...stageBlock,
    '  build:',
    `    dockerfile: ${input.dockerfilePath}`,
    '    context: .',
    '  deploy:',
    `    mode: ${input.deployMode}`,
    `    manifestPath: ${input.deployYamlPath}`,
    '    values:',
    '      namespace: default',
    '      strategy: rolling-update',
    '',
    '# Selected Deploy Manifest',
    ...input.deployYamlContent.split('\n').map((line) => `# ${line}`),
  ].join('\n')
}

export function CicdPipelineSetupPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const selectedTemplateId = searchParams.get('template')

  const { data: templatesData } = useCicdTemplates()
  const templates = Array.isArray(templatesData) && templatesData.length > 0 ? templatesData : DEFAULT_TEMPLATES
  const template = templates.find((item) => item.id === selectedTemplateId) ?? templates[0]

  const { data: clustersData } = useClusters()
  const clusterList = clustersData?.items ?? []
  const clusterOptions = clusterList.length > 0
    ? clusterList.map((cluster) => ({ id: cluster.id, name: cluster.name }))
    : [
      { id: 'c1', name: 'prod-k8s' },
      { id: 'c2', name: 'dev-k8s' },
    ]

  const createPipeline = useCreatePipeline()

  const [activeTab, setActiveTab] = useState<SetupTab>('cluster')
  const [pipelineName, setPipelineName] = useState(template ? `${template.name.toLowerCase().replace(/\s+/g, '-')}-pipeline` : '')
  const [clusterId, setClusterId] = useState(clusterOptions[0]?.id ?? '')
  const [dockerfileId, setDockerfileId] = useState(DOCKERFILE_PRESETS[0].id)
  const [deployMode, setDeployMode] = useState<DeployMode>('template')
  const [deployYamlId, setDeployYamlId] = useState(DEPLOY_YAML_PRESETS[0].id)
  const [customDeployYaml, setCustomDeployYaml] = useState(DEPLOY_YAML_PRESETS[0].content)
  const [formError, setFormError] = useState<string | null>(null)

  const selectedAppType = template?.appType ?? 'web-backend'

  const selectedDockerfile = DOCKERFILE_PRESETS.find((preset) => preset.id === dockerfileId) ?? DOCKERFILE_PRESETS[0]
  const selectedDeployYaml = DEPLOY_YAML_PRESETS.find((preset) => preset.id === deployYamlId) ?? DEPLOY_YAML_PRESETS[0]
  const selectedClusterName = clusterOptions.find((cluster) => cluster.id === clusterId)?.name ?? 'unknown-cluster'

  const effectiveDeployYaml = deployMode === 'template' ? selectedDeployYaml.content : customDeployYaml

  const generatedPipelineYaml = getPipelineYaml({
      pipelineName,
      appType: selectedAppType,
      clusterName: selectedClusterName,
      templateName: template?.name ?? 'custom-template',
      dockerfilePath: selectedDockerfile.path,
      deployYamlPath: deployMode === 'template' ? `./deploy/${selectedDeployYaml.id}.yaml` : './deploy/custom.yaml',
      deployMode,
      deployYamlContent: effectiveDeployYaml,
    })

  const handleCreatePipeline = () => {
    if (!pipelineName.trim()) {
      setFormError('Pipeline name is required.')
      return
    }
    if (!clusterId) {
      setFormError('Cluster selection is required.')
      return
    }

    setFormError(null)
    createPipeline.mutate(
      {
        name: pipelineName.trim(),
        appType: selectedAppType,
        clusterId,
        templateId: template?.id,
      },
      {
        onSuccess: () => {
          navigate('/cicd/list')
        },
      }
    )
  }

  return (
    <div>
      <Breadcrumb
        items={[
          { label: 'CI/CD List', path: '/cicd/list' },
          { label: 'CI/CD Template', path: '/cicd/templates' },
          { label: 'Pipeline Setup' },
        ]}
      />

      <div className="mb-6 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <Settings2 size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              CI/CD Pipeline Setup
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              Stack Install과 유사한 단계형 UX로 파이프라인을 설정합니다.
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="md" type="button" onClick={() => navigate('/cicd/templates')}>
            <GitBranch size={14} />
            Change Template
          </Button>
          <Button
            variant="primary"
            size="md"
            type="button"
            onClick={handleCreatePipeline}
            loading={createPipeline.isPending}
          >
            <Rocket size={14} />
            Create Pipeline
          </Button>
        </div>
      </div>

      <div className="mb-5 grid grid-cols-2 gap-4">
        <Input
          label="Pipeline Name"
          placeholder="예: web-frontend-prod"
          value={pipelineName}
          onChange={(event) => setPipelineName(event.target.value)}
        />
        <div className="flex flex-col gap-1">
          <label htmlFor="pipeline-template" className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
            Selected Template
          </label>
          <select
            id="pipeline-template"
            value={template?.id ?? ''}
            onChange={(event) => {
              const nextTemplate = templates.find((item) => item.id === event.target.value)
              if (nextTemplate) {
                setPipelineName(`${nextTemplate.name.toLowerCase().replace(/\s+/g, '-')}-pipeline`)
                navigate(`/cicd/pipeline/setup?template=${nextTemplate.id}`)
              }
            }}
            className={appTypeOptionClassName}
          >
            {templates.map((item) => (
              <option key={item.id} value={item.id} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">
                {item.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="flex items-start gap-5">
        <div className="min-w-0 flex-1">
          <div className="mb-5 flex gap-0 border-b border-[var(--color-border-default)]">
            {TABS.map((tab) => {
              const Icon = tab.icon
              const active = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => setActiveTab(tab.id)}
                  className={cn(
                    '-mb-px flex cursor-pointer items-center gap-1.5 border-b-2 border-b-transparent bg-none px-[16px] py-2.5 text-sm transition-all duration-150',
                    active
                      ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                      : 'font-normal text-[var(--color-text-secondary)]'
                  )}
                >
                  <Icon size={14} />
                  {tab.label}
                </button>
              )
            })}
          </div>

          <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            {activeTab === 'cluster' && (
              <div className="max-w-[420px]">
                <div className="flex flex-col gap-1">
                  <label htmlFor="pipeline-cluster" className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
                    Deploy Cluster
                  </label>
                  <select
                    id="pipeline-cluster"
                    value={clusterId}
                    onChange={(event) => setClusterId(event.target.value)}
                    className={appTypeOptionClassName}
                  >
                    {clusterOptions.map((cluster) => (
                      <option key={cluster.id} value={cluster.id} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">
                        {cluster.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            {activeTab === 'build' && (
              <div className="flex flex-col gap-4">
                <div>
                  <p className="mb-2 mt-0 text-[13px] text-[var(--color-text-secondary)]">
                    Build 단계에서 사용할 Dockerfile을 선택합니다.
                  </p>
                  <div className="grid grid-cols-3 gap-2.5">
                    {DOCKERFILE_PRESETS.map((preset) => {
                      const selected = dockerfileId === preset.id
                      return (
                        <button
                          key={preset.id}
                          type="button"
                          onClick={() => setDockerfileId(preset.id)}
                          className={cn(
                            'cursor-pointer rounded-lg border p-3 text-left transition-all duration-150',
                            selected
                              ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                              : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                          )}
                        >
                          <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                            {preset.label}
                          </div>
                          <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{preset.path}</div>
                        </button>
                      )
                    })}
                  </div>
                </div>

                <YamlEditor value={selectedDockerfile.content} readOnly height="360px" />
              </div>
            )}

            {activeTab === 'deploy' && (
              <div className="flex flex-col gap-4">
                <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2">
                  <button
                    type="button"
                    className={cn(
                      'cursor-pointer rounded-md px-3 py-1.5 text-xs font-semibold',
                      deployMode === 'template'
                        ? 'bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]'
                        : 'text-[var(--color-text-secondary)]'
                    )}
                    onClick={() => setDeployMode('template')}
                  >
                    Select Template YAML
                  </button>
                  <button
                    type="button"
                    className={cn(
                      'cursor-pointer rounded-md px-3 py-1.5 text-xs font-semibold',
                      deployMode === 'custom'
                        ? 'bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]'
                        : 'text-[var(--color-text-secondary)]'
                    )}
                    onClick={() => setDeployMode('custom')}
                  >
                    Write Custom YAML
                  </button>
                </div>

                {deployMode === 'template' ? (
                  <>
                    <div className="grid grid-cols-3 gap-2.5">
                      {DEPLOY_YAML_PRESETS.map((preset) => {
                        const selected = deployYamlId === preset.id
                        return (
                          <button
                            key={preset.id}
                            type="button"
                            onClick={() => setDeployYamlId(preset.id)}
                            className={cn(
                              'cursor-pointer rounded-lg border p-3 text-left transition-all duration-150',
                              selected
                                ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)]'
                                : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]'
                            )}
                          >
                            <div className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                              {preset.label}
                            </div>
                            <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{preset.description}</div>
                          </button>
                        )
                      })}
                    </div>
                    <YamlEditor value={selectedDeployYaml.content} readOnly height="360px" />
                  </>
                ) : (
                  <YamlEditor value={customDeployYaml} onChange={setCustomDeployYaml} height="360px" />
                )}
              </div>
            )}

            {activeTab === 'yaml' && (
              <div>
                <p className="mb-3 mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  현재 선택한 구성값을 바탕으로 생성된 파이프라인 YAML 미리보기입니다.
                </p>
                <YamlEditor value={generatedPipelineYaml} readOnly height="360px" />
              </div>
            )}

            {formError && <div className="mt-3 text-xs text-[#f87171]">{formError}</div>}
          </div>
        </div>

        <div className="sticky top-6 w-[270px] shrink-0 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h3 className="mb-3 mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-primary)]">
            Setup Summary
          </h3>
          {[
            ['Template', template?.name ?? '—'],
            ['Pipeline', pipelineName || '—'],
            ['App Type', selectedAppType],
            ['Cluster', selectedClusterName],
            ['Dockerfile', selectedDockerfile.path],
            ['Deploy Mode', deployMode === 'template' ? 'Template YAML' : 'Custom YAML'],
            ['Deploy YAML', deployMode === 'template' ? selectedDeployYaml.label : 'custom.yaml'],
          ].map(([label, value]) => (
            <div key={label} className="flex items-baseline justify-between gap-2 border-b border-[rgba(255,255,255,0.04)] py-1.5">
              <span className="shrink-0 text-[11px] text-[var(--color-text-secondary)]">{label}</span>
              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-right text-xs font-semibold text-[var(--color-text-primary)]">
                {value}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
