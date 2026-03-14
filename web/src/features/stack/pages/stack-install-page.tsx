import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Download, Save, Rocket } from 'lucide-react'
import { useStackConfigStore } from '../stores/stack-config-store'
import type { InstallTab, ToolSelection, StackConfigDraft } from '../stores/stack-config-store'
import { useCreateStack, useSaveDraft, useEstimateResources } from '../api/stack-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { YamlEditor } from '../../../components/shared/yaml-editor'

// --- Tool option types ---

interface ToolOption {
  id: string
  label: string
  description: string
}

const ARTIFACTS_OPTIONS: Record<string, ToolOption[]> = {
  packageRegistry: [
    { id: 'gitlab', label: 'GitLab Package Registry', description: 'GitLab 내장 패키지 레지스트리' },
    { id: 'nexus', label: 'Nexus Repository', description: '범용 아티팩트 저장소' },
    { id: 'jfrog', label: 'JFrog Artifactory', description: '엔터프라이즈급 아티팩트 관리' },
  ],
  sourceRepository: [
    { id: 'gitlab', label: 'GitLab', description: 'GitLab 소스 코드 관리' },
    { id: 'github', label: 'GitHub', description: 'GitHub 소스 코드 관리' },
    { id: 'gitea', label: 'Gitea', description: '경량 셀프호스팅 Git 서비스' },
  ],
  containerRegistry: [
    { id: 'gitlab-registry', label: 'GitLab Container Registry', description: 'GitLab 내장 컨테이너 레지스트리' },
    { id: 'harbor', label: 'Harbor', description: '엔터프라이즈 컨테이너 레지스트리' },
    { id: 'docker-hub', label: 'Docker Hub', description: 'Docker 공식 레지스트리' },
  ],
  storageBackend: [
    { id: 'minio', label: 'MinIO', description: 'S3 호환 오브젝트 스토리지' },
    { id: 's3', label: 'AWS S3', description: 'Amazon S3 오브젝트 스토리지' },
    { id: 'gcs', label: 'Google Cloud Storage', description: 'GCP 오브젝트 스토리지' },
  ],
}

const PIPELINE_OPTIONS: Record<string, ToolOption[]> = {
  cicdPlatform: [
    { id: 'gitlab-ci', label: 'GitLab CI/CD', description: 'GitLab 내장 CI/CD 파이프라인' },
    { id: 'github-actions', label: 'GitHub Actions', description: 'GitHub 워크플로우 기반 CI/CD' },
    { id: 'jenkins', label: 'Jenkins', description: '전통적인 오픈소스 CI 서버' },
  ],
  cdTool: [
    { id: 'argocd', label: 'ArgoCD', description: 'GitOps 기반 쿠버네티스 CD' },
    { id: 'flux', label: 'Flux CD', description: 'GitOps 툴킷' },
    { id: 'spinnaker', label: 'Spinnaker', description: '멀티 클라우드 CD 플랫폼' },
  ],
}

const MONITORING_OPTIONS: Record<string, ToolOption[]> = {
  collection: [
    { id: 'prometheus', label: 'Prometheus', description: '시계열 메트릭 수집' },
    { id: 'datadog', label: 'Datadog', description: '클라우드 모니터링 플랫폼' },
    { id: 'newrelic', label: 'New Relic', description: 'APM 및 모니터링 플랫폼' },
  ],
  visualization: [
    { id: 'grafana', label: 'Grafana', description: '오픈소스 메트릭 시각화' },
    { id: 'kibana', label: 'Kibana', description: 'Elastic Stack 시각화' },
    { id: 'datadog-dashboards', label: 'Datadog Dashboards', description: 'Datadog 내장 대시보드' },
  ],
}

const LOGGING_OPTIONS: Record<string, ToolOption[]> = {
  collection: [
    { id: 'opentelemetry', label: 'OpenTelemetry', description: '벤더 중립 텔레메트리 수집' },
    { id: 'fluentbit', label: 'Fluent Bit', description: '경량 로그 수집기' },
    { id: 'logstash', label: 'Logstash', description: 'ELK Stack 로그 파이프라인' },
  ],
  search: [
    { id: 'opensearch', label: 'OpenSearch', description: 'Elasticsearch 호환 검색/분석' },
    { id: 'elasticsearch', label: 'Elasticsearch', description: '분산 검색/분석 엔진' },
    { id: 'loki', label: 'Grafana Loki', description: 'Prometheus 스타일 로그 집계' },
  ],
}

// --- ToolSelector component ---

interface ToolSelectorProps {
  label: string
  options: ToolOption[]
  value: ToolSelection
  onChange: (v: ToolSelection) => void
}

function ToolSelector({ label, options, value, onChange }: ToolSelectorProps) {
  return (
    <div style={{ marginBottom: '20px' }}>
      <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: '10px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
        {label}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {options.map((opt) => {
          const selected = value.tool === opt.id
          return (
            <div
              key={opt.id}
              onClick={() => onChange({ tool: opt.id, version: 'latest' })}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '12px 14px',
                background: selected ? 'rgba(99,102,241,0.1)' : 'rgba(255,255,255,0.02)',
                border: `1px solid ${selected ? 'rgba(99,102,241,0.5)' : 'var(--color-border-default)'}`,
                borderRadius: '8px',
                cursor: 'pointer',
                transition: 'all var(--transition-fast)',
              }}
            >
              <div
                style={{
                  width: '16px',
                  height: '16px',
                  borderRadius: '50%',
                  border: `2px solid ${selected ? '#6366f1' : 'var(--color-border-hover)'}`,
                  background: selected ? '#6366f1' : 'transparent',
                  flexShrink: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                {selected && (
                  <div style={{ width: '6px', height: '6px', borderRadius: '50%', background: '#fff' }} />
                )}
              </div>
              <div>
                <div style={{ fontSize: '14px', fontWeight: 600, color: selected ? '#a5b4fc' : 'var(--color-text-primary)' }}>
                  {opt.label}
                </div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{opt.description}</div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// --- YAML conversion ---

function draftToYaml(draft: StackConfigDraft): string {
  const lines: string[] = [
    `stackName: ${draft.stackName || '""'}`,
    `templateId: ${draft.selectedTemplateId ?? 'null'}`,
    `clusterId: ${draft.clusterId ?? 'null'}`,
    '',
    'artifacts:',
    `  packageRegistry: ${draft.artifacts.packageRegistry.tool}`,
    `  sourceRepository: ${draft.artifacts.sourceRepository.tool}`,
    `  containerRegistry: ${draft.artifacts.containerRegistry.tool}`,
    `  storageBackend: ${draft.artifacts.storageBackend.tool}`,
    '',
    'pipeline:',
    `  cicdPlatform: ${draft.pipeline.cicdPlatform.tool}`,
    `  cdTool: ${draft.pipeline.cdTool.tool}`,
    '',
    'monitoring:',
    `  collection: ${draft.monitoring.collection.tool}`,
    `  visualization: ${draft.monitoring.visualization.tool}`,
    '',
    'logging:',
    `  collection: ${draft.logging.collection.tool}`,
    `  search: ${draft.logging.search.tool}`,
    '',
    'resources:',
    `  developerCount: ${draft.resources.developerCount}`,
    `  concurrentRunners: ${draft.resources.concurrentRunners}`,
    `  commitsPerDay: ${draft.resources.commitsPerDay}`,
    `  buildFrequency: ${draft.resources.buildFrequency}`,
    `  currency: ${draft.resources.currency}`,
  ]
  return lines.join('\n')
}

// --- Tab definitions ---

const TABS: { id: InstallTab; label: string }[] = [
  { id: 'artifacts', label: 'Artifacts' },
  { id: 'pipeline', label: 'Pipeline' },
  { id: 'monitoring', label: 'Monitoring' },
  { id: 'logging', label: 'Logging' },
  { id: 'resources', label: 'Resources' },
  { id: 'yaml', label: 'YAML View' },
]

// --- Main page ---

export function StackInstallPage() {
  const navigate = useNavigate()
  const { draft, setActiveTab, setTool, setStackName, updateResources } = useStackConfigStore()
  const createStack = useCreateStack()
  const saveDraft = useSaveDraft()
  const estimateResources = useEstimateResources()
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)

  const switchTab = (tab: InstallTab) => {
    setLocalTab(tab)
    setActiveTab(tab)
  }

  const handleDeploy = () => {
    createStack.mutate(
      {
        templateId: draft.selectedTemplateId,
        clusterId: draft.clusterId,
        stackName: draft.stackName,
        artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
        pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
        monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
        logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
        resources: draft.resources,
      },
      {
        onSuccess: () => navigate('/stack/list'),
      }
    )
  }

  const handleSaveDraft = () => {
    saveDraft.mutate({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      stackName: draft.stackName,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
    })
  }

  const handleEstimate = () => {
    estimateResources.mutate(draft.resources)
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              width: 'var(--icon-size)',
              height: 'var(--icon-size)',
              background: 'rgba(99,102,241,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#818cf8',
            }}
          >
            <Download size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Stack Install
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              5단계 워크플로우로 DevSecOps 스택을 구성하세요.
            </p>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          <Button variant="secondary" size="md" loading={saveDraft.isPending} onClick={handleSaveDraft}>
            <Save size={14} />
            Save Draft
          </Button>
          <Button variant="primary" size="md" loading={createStack.isPending} onClick={handleDeploy}>
            <Rocket size={14} />
            Deploy
          </Button>
        </div>
      </div>

      {/* Stack name */}
      <div style={{ marginBottom: '20px', maxWidth: '400px' }}>
        <Input
          label="Stack Name"
          placeholder="예: prod-gitlab-stack"
          value={draft.stackName}
          onChange={(e) => setStackName(e.target.value)}
        />
      </div>

      <div style={{ display: 'flex', gap: '20px', alignItems: 'flex-start' }}>
        {/* Left: tabs + content */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {/* Tabs */}
          <div
            style={{
              display: 'flex',
              gap: '0',
              borderBottom: '1px solid var(--color-border-default)',
              marginBottom: '20px',
            }}
          >
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  onClick={() => switchTab(tab.id)}
                  style={{
                    background: 'none',
                    border: 'none',
                    borderBottom: `2px solid ${isActive ? '#6366f1' : 'transparent'}`,
                    padding: '10px 18px',
                    cursor: 'pointer',
                    fontSize: '14px',
                    fontWeight: isActive ? 600 : 400,
                    color: isActive ? '#a5b4fc' : 'var(--color-text-secondary)',
                    transition: 'all var(--transition-fast)',
                    marginBottom: '-1px',
                  }}
                >
                  {tab.label}
                </button>
              )
            })}
          </div>

          {/* Tab content */}
          <div
            style={{
              background: 'var(--color-surface-card)',
              border: '1px solid var(--color-border-default)',
              borderRadius: 'var(--card-radius)',
              padding: '20px',
            }}
          >
            {activeTab === 'artifacts' && (
              <>
                <ToolSelector
                  label="Package Registry"
                  options={ARTIFACTS_OPTIONS.packageRegistry}
                  value={draft.artifacts.packageRegistry}
                  onChange={(v) => setTool('artifacts', 'packageRegistry', v)}
                />
                <ToolSelector
                  label="Source Repository"
                  options={ARTIFACTS_OPTIONS.sourceRepository}
                  value={draft.artifacts.sourceRepository}
                  onChange={(v) => setTool('artifacts', 'sourceRepository', v)}
                />
                <ToolSelector
                  label="Container Registry"
                  options={ARTIFACTS_OPTIONS.containerRegistry}
                  value={draft.artifacts.containerRegistry}
                  onChange={(v) => setTool('artifacts', 'containerRegistry', v)}
                />
                <ToolSelector
                  label="Storage Backend"
                  options={ARTIFACTS_OPTIONS.storageBackend}
                  value={draft.artifacts.storageBackend}
                  onChange={(v) => setTool('artifacts', 'storageBackend', v)}
                />
              </>
            )}

            {activeTab === 'pipeline' && (
              <>
                <ToolSelector
                  label="CI/CD Platform"
                  options={PIPELINE_OPTIONS.cicdPlatform}
                  value={draft.pipeline.cicdPlatform}
                  onChange={(v) => setTool('pipeline', 'cicdPlatform', v)}
                />
                <ToolSelector
                  label="CD Tool"
                  options={PIPELINE_OPTIONS.cdTool}
                  value={draft.pipeline.cdTool}
                  onChange={(v) => setTool('pipeline', 'cdTool', v)}
                />
              </>
            )}

            {activeTab === 'monitoring' && (
              <>
                <ToolSelector
                  label="Metrics Collection"
                  options={MONITORING_OPTIONS.collection}
                  value={draft.monitoring.collection}
                  onChange={(v) => setTool('monitoring', 'collection', v)}
                />
                <ToolSelector
                  label="Visualization"
                  options={MONITORING_OPTIONS.visualization}
                  value={draft.monitoring.visualization}
                  onChange={(v) => setTool('monitoring', 'visualization', v)}
                />
              </>
            )}

            {activeTab === 'logging' && (
              <>
                <ToolSelector
                  label="Log Collection"
                  options={LOGGING_OPTIONS.collection}
                  value={draft.logging.collection}
                  onChange={(v) => setTool('logging', 'collection', v)}
                />
                <ToolSelector
                  label="Log Search"
                  options={LOGGING_OPTIONS.search}
                  value={draft.logging.search}
                  onChange={(v) => setTool('logging', 'search', v)}
                />
              </>
            )}

            {activeTab === 'yaml' && (
              <div>
                <p style={{ margin: '0 0 14px 0', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
                  현재 스택 설정의 YAML 표현입니다. (읽기 전용)
                </p>
                <YamlEditor
                  value={draftToYaml(draft)}
                  readOnly
                  height="360px"
                />
              </div>
            )}

            {activeTab === 'resources' && (
              <div>
                <p style={{ margin: '0 0 16px 0', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
                  팀 규모와 사용 패턴을 입력하면 필요한 리소스를 계산합니다.
                </p>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px', marginBottom: '16px' }}>
                  <Input
                    label="개발자 수"
                    type="number"
                    min={1}
                    value={draft.resources.developerCount}
                    onChange={(e) => updateResources({ developerCount: Number(e.target.value) })}
                  />
                  <Input
                    label="동시 러너 수"
                    type="number"
                    min={1}
                    value={draft.resources.concurrentRunners}
                    onChange={(e) => updateResources({ concurrentRunners: Number(e.target.value) })}
                  />
                  <Input
                    label="일일 커밋 수"
                    type="number"
                    min={1}
                    value={draft.resources.commitsPerDay}
                    onChange={(e) => updateResources({ commitsPerDay: Number(e.target.value) })}
                  />
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>
                      빌드 빈도
                    </label>
                    <select
                      value={draft.resources.buildFrequency}
                      onChange={(e) =>
                        updateResources({ buildFrequency: e.target.value as 'low' | 'medium' | 'high' })
                      }
                      style={{
                        background: 'rgba(255,255,255,0.04)',
                        border: '1px solid var(--color-border-default)',
                        borderRadius: '8px',
                        padding: '9px 12px',
                        fontSize: '14px',
                        color: 'var(--color-text-primary)',
                      }}
                    >
                      <option value="low">낮음 (Low)</option>
                      <option value="medium">보통 (Medium)</option>
                      <option value="high">높음 (High)</option>
                    </select>
                  </div>
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  loading={estimateResources.isPending}
                  onClick={handleEstimate}
                  style={{ marginBottom: '16px' }}
                >
                  리소스 계산
                </Button>
                {estimateResources.data && (
                  <div
                    style={{
                      background: 'rgba(99,102,241,0.08)',
                      border: '1px solid rgba(99,102,241,0.3)',
                      borderRadius: '8px',
                      padding: '14px',
                      display: 'grid',
                      gridTemplateColumns: 'repeat(4, 1fr)',
                      gap: '12px',
                    }}
                  >
                    {[
                      ['CPU', estimateResources.data.cpu],
                      ['Memory', estimateResources.data.memory],
                      ['Storage', estimateResources.data.storage],
                      ['월 비용', `${estimateResources.data.estimatedCostMonthly.toLocaleString()} ${estimateResources.data.currency}`],
                    ].map(([label, val]) => (
                      <div key={label}>
                        <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '4px' }}>{label}</div>
                        <div style={{ fontSize: '15px', fontWeight: 700, color: '#a5b4fc' }}>{val}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Right: Configuration Summary */}
        <div
          style={{
            width: '260px',
            flexShrink: 0,
            background: 'var(--color-surface-card)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
            padding: '16px',
            position: 'sticky',
            top: '24px',
          }}
        >
          <h3 style={{ margin: '0 0 14px 0', fontSize: '13px', fontWeight: 700, color: 'var(--color-text-primary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            Configuration Summary
          </h3>
          {[
            ['Template', draft.selectedTemplateId ?? '—'],
            ['Stack Name', draft.stackName || '—'],
            ['Package Registry', draft.artifacts.packageRegistry.tool],
            ['Source Repo', draft.artifacts.sourceRepository.tool],
            ['Container Registry', draft.artifacts.containerRegistry.tool],
            ['Storage', draft.artifacts.storageBackend.tool],
            ['CI/CD Platform', draft.pipeline.cicdPlatform.tool],
            ['CD Tool', draft.pipeline.cdTool.tool],
            ['Metrics', draft.monitoring.collection.tool],
            ['Visualization', draft.monitoring.visualization.tool],
            ['Log Collection', draft.logging.collection.tool],
            ['Log Search', draft.logging.search.tool],
          ].map(([label, val]) => (
            <div
              key={label}
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                padding: '6px 0',
                borderBottom: '1px solid rgba(255,255,255,0.04)',
                gap: '8px',
              }}
            >
              <span style={{ fontSize: '11px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{label}</span>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-primary)', textAlign: 'right', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {val}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
