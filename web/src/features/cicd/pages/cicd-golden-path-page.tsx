import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, ExternalLink, Search } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useGoldenPaths } from '../api/cicd-api'
import type { CICDGoldenPath, CICDTool } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { useStackConfigStore } from '../../stack/stores/stack-config-store'

// Golden Path 도구 이름 → Stack 설정 tool ID 매핑
const TOOL_NAME_TO_ID: Record<string, string> = {
  'GitLab CE': 'gitlab',
  'GitLab CI': 'gitlab-ci',
  'GitLab Registry': 'gitlab-registry',
  'GitHub': 'github',
  'GitHub Actions': 'github-actions',
  'Harbor': 'harbor',
  'Docker Hub': 'docker-hub',
  'MinIO': 'minio',
  'AWS S3': 's3',
  'Argo CD': 'argocd',
  'Flux CD': 'flux',
  'Spinnaker': 'spinnaker',
  'Prometheus': 'prometheus',
  'Thanos': 'thanos',
  'VictoriaMetrics': 'victoriametrics',
  'Grafana': 'grafana',
  'Kibana': 'kibana',
  'Loki': 'loki',
  'OpenSearch': 'opensearch',
  'Elasticsearch': 'elasticsearch',
  'Tempo': 'tempo',
  'Jaeger': 'jaeger',
}

/** Golden Path 도구 목록을 Stack 설정 오버라이드로 변환 */
function goldenPathToStackOverrides(tools: CICDTool[]) {
  const artifacts = {
    packageRegistry: { tool: 'gitlab', version: 'latest' },
    sourceRepository: { tool: 'gitlab', version: 'latest' },
    containerRegistry: { tool: 'gitlab-registry', version: 'latest' },
    storageBackend: { tool: 'minio', version: 'latest' },
  }
  const pipeline = {
    cicdPlatform: { tool: 'gitlab-ci', version: 'latest' },
    cdTool: { tool: 'argocd', version: 'latest' },
  }
  const monitoring = {
    collection: { tool: 'prometheus', version: 'latest' },
    visualization: { tool: 'grafana', version: 'latest' },
    visualizations: [{ tool: 'grafana', version: 'latest' }],
  }
  const logging = {
    search: { tool: 'opensearch', version: 'latest' },
    traceLayer: { tool: 'tempo', version: 'latest' },
    traceExporter: { tool: 'opentelemetry-collector', version: 'latest' },
  }

  for (const tool of tools) {
    const toolId = TOOL_NAME_TO_ID[tool.name]
    if (!toolId) continue
    const version = tool.helm_version

    switch (tool.category) {
      case 'source_repository':
        artifacts.sourceRepository = { tool: toolId, version }
        break
      case 'container_registry':
        artifacts.containerRegistry = { tool: toolId, version }
        break
      case 'storage_backend':
        artifacts.storageBackend = { tool: toolId, version }
        break
      case 'ci_platform':
        pipeline.cicdPlatform = { tool: toolId, version }
        break
      case 'cd_tool':
        pipeline.cdTool = { tool: toolId, version }
        break
      case 'monitoring_collection':
        monitoring.collection = { tool: toolId, version }
        break
      case 'monitoring_visualization':
        monitoring.visualization = { tool: toolId, version }
        monitoring.visualizations = [{ tool: toolId, version }]
        break
      case 'log_aggregation':
        logging.search = { tool: toolId, version }
        break
    }
  }

  return { artifacts, pipeline, monitoring, logging }
}

const TOOL_CATEGORY_COLORS: Record<string, { bg: string; color: string }> = {
  source_repository: { bg: 'rgba(59,130,246,0.12)', color: '#60a5fa' },
  ci_platform: { bg: 'rgba(139,92,246,0.12)', color: '#a78bfa' },
  container_registry: { bg: 'rgba(34,197,94,0.12)', color: '#86efac' },
  storage_backend: { bg: 'rgba(249,115,22,0.12)', color: '#fb923c' },
  cd_tool: { bg: 'rgba(236,72,153,0.12)', color: '#f472b6' },
  monitoring_collection: { bg: 'rgba(168,85,247,0.12)', color: '#d8b4fe' },
  monitoring_visualization: { bg: 'rgba(14,165,233,0.12)', color: '#38bdf8' },
  log_aggregation: { bg: 'rgba(234,179,8,0.12)', color: '#fde047' },
}

export function CicdGoldenPathPage() {
  const navigate = useNavigate()
  const { data: goldenPaths, isLoading } = useGoldenPaths()
  const loadFromTemplate = useStackConfigStore((s) => s.loadFromTemplate)
  const [search, setSearch] = useState('')
  const [selectedPath, setSelectedPath] = useState<CICDGoldenPath | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const filtered = useMemo(
    () =>
      (goldenPaths || []).filter(
        (gp) =>
          gp.name.toLowerCase().includes(search.toLowerCase()) ||
          gp.description.toLowerCase().includes(search.toLowerCase())
      ),
    [goldenPaths, search]
  )

  const openDetail = (path: CICDGoldenPath) => {
    setSelectedPath(path)
    setDetailOpen(true)
  }

  const closeDetail = () => {
    setDetailOpen(false)
    setSelectedPath(null)
  }

  const getToolColor = (category: string) => {
    return TOOL_CATEGORY_COLORS[category] ?? { bg: 'rgba(107,114,128,0.12)', color: '#9ca3af' }
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: 'CI/CD List', path: '/cicd/list' },
        { label: 'CI/CD Golden Path' },
      ]} />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between gap-4">
        <div className="mb-2 flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(34,197,94,0.15)] text-[#86efac]"
          >
            <BookOpen size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              CI/CD Golden Path
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              검증된 CI/CD 도구 조합으로 빠르게 시작하세요.
            </p>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="mb-5 max-w-[360px]">
        <div className="relative">
          <Search
            size={13}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
          <input
            placeholder="Golden Path 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
          />
        </div>
      </div>

      {/* Golden Path cards */}
      {isLoading ? (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          로딩 중...
        </div>
      ) : filtered.length === 0 ? (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          검색 결과가 없습니다.
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(340px,1fr))] gap-4">
          {filtered.map((goldenPath) => (
            <div
              key={goldenPath.id}
              className="flex h-full flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-150 hover:border-[var(--color-border-hover)]"
            >
              {/* Card header */}
              <div>
                <h3 className="m-0 mb-1 text-[15px] font-bold text-[var(--color-text-primary)]">
                  {goldenPath.name}
                </h3>
                <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                  {goldenPath.description}
                </p>
              </div>

              {/* Info grid */}
              <div className="grid grid-cols-2 gap-3 rounded-lg bg-[rgba(255,255,255,0.02)] p-3">
                <div>
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    설치 시간
                  </span>
                  <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {goldenPath.estimated_install_time}분
                  </p>
                </div>
                <div>
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    권장 사용
                  </span>
                  <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {goldenPath.recommended_use_case}
                  </p>
                </div>
                <div className="col-span-2">
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    최소 리소스
                  </span>
                  <p className="m-0 mt-1 text-[12px] text-[var(--color-text-primary)]">
                    {goldenPath.min_resources}
                  </p>
                </div>
              </div>

              {/* Tools preview */}
              <div>
                <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                  포함된 도구
                </span>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {goldenPath.tools.slice(0, 4).map((tool) => {
                    const color = getToolColor(tool.category)
                    return (
                      <span
                        key={`${tool.category}-${tool.name}`}
                        className="rounded-md px-2 py-1 text-[11px] font-semibold"
                        style={{ backgroundColor: color.bg, color: color.color }}
                      >
                        {tool.name}
                      </span>
                    )
                  })}
                  {goldenPath.tools.length > 4 && (
                    <span className="rounded-md bg-[rgba(107,114,128,0.12)] px-2 py-1 text-[11px] font-semibold text-[#9ca3af]">
                      +{goldenPath.tools.length - 4}
                    </span>
                  )}
                </div>
              </div>

              {/* Footer */}
              <div className="mt-auto border-t border-[var(--color-border-default)] pt-3">
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  className="w-full"
                  onClick={() => openDetail(goldenPath)}
                >
                  <ExternalLink size={13} />
                  상세 보기
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {filtered.length === 0 && !isLoading && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          검색 결과가 없습니다.
        </div>
      )}
      {/* Detail Modal */}
      <Modal
        open={detailOpen}
        onClose={closeDetail}
        title={selectedPath?.name ?? ''}

        footer={
          <>
            <Button variant="outline" size="sm" onClick={closeDetail} type="button">
              닫기
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              className="bg-[linear-gradient(135deg,#34d399,#10b981)] text-white"
              onClick={() => {
                if (selectedPath) {
                  const overrides = goldenPathToStackOverrides(selectedPath.tools)
                  loadFromTemplate(selectedPath.id, overrides)
                  navigate('/stack/install')
                }
              }}
            >
              이 Golden Path 사용
            </Button>
          </>
        }
      >
        {selectedPath && (
          <div className="flex flex-col gap-4">
            <div>
              <h4 className="mb-1 text-sm font-semibold text-[var(--color-text-primary)]">
                설명
              </h4>
              <p className="m-0 text-sm text-[var(--color-text-secondary)]">
                {selectedPath.description}
              </p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  설치 시간
                </span>
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {selectedPath.estimated_install_time}분
                </p>
              </div>
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  권장 사용
                </span>
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {selectedPath.recommended_use_case}
                </p>
              </div>
              <div className="col-span-2">
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  최소 리소스
                </span>
                <p className="m-0 mt-1 text-sm text-[var(--color-text-primary)]">
                  {selectedPath.min_resources}
                </p>
              </div>
            </div>

            <div>
              <h4 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">
                포함된 도구
              </h4>
              <div className="space-y-2">
                {selectedPath.tools.map((tool) => {
                  const color = getToolColor(tool.category)
                  return (
                    <div
                      key={`${tool.category}-${tool.name}`}
                      className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] p-2.5"
                    >
                      <div className="flex items-center gap-2">
                        <span
                          className="rounded-md px-2 py-1 text-[11px] font-semibold"
                          style={{ backgroundColor: color.bg, color: color.color }}
                        >
                          {tool.category}
                        </span>
                        <span className="text-sm font-semibold text-[var(--color-text-primary)]">
                          {tool.name}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
                        <span>Helm: {tool.helm_version}</span>
                        <span>App: {tool.app_version}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
