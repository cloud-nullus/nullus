import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ExternalLink, Route } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { useGoldenPaths } from '../api/cicd-api'
import type { CICDGoldenPath, CICDTool } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import { useStackConfigStore } from '../../stack/stores/stack-config-store'
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'

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
  source_repository: { bg: 'color-mix(in srgb, var(--color-info) 12%, transparent)', color: 'var(--color-info)' },
  ci_platform: { bg: 'color-mix(in srgb, var(--color-accent-alt) 12%, transparent)', color: 'var(--color-accent-alt)' },
  container_registry: { bg: 'color-mix(in srgb, var(--color-success) 12%, transparent)', color: 'var(--color-success)' },
  storage_backend: { bg: 'color-mix(in srgb, var(--color-warning) 12%, transparent)', color: 'var(--color-warning)' },
  cd_tool: { bg: 'color-mix(in srgb, var(--color-accent-alt) 12%, transparent)', color: 'var(--color-accent-alt)' },
  monitoring_collection: { bg: 'color-mix(in srgb, var(--color-accent-alt) 12%, transparent)', color: 'var(--color-accent-alt)' },
  monitoring_visualization: { bg: 'color-mix(in srgb, var(--color-info) 12%, transparent)', color: 'var(--color-info)' },
  log_aggregation: { bg: 'color-mix(in srgb, var(--color-warning) 12%, transparent)', color: 'var(--color-warning)' },
}

export function CicdGoldenPathPage() {
  const { t } = useTranslation()
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
    return TOOL_CATEGORY_COLORS[category] ?? { bg: 'color-mix(in srgb, var(--color-text-muted) 12%, transparent)', color: 'var(--color-text-muted)' }
  }

  return (
    <div>
      <PageHeader
        breadcrumb={
          [
            { label: 'CI/CD List', path: '/cicd/list' },
            { label: 'CI/CD Golden Path' },
          ]
        }
        icon={<Route {...iconProps('sm')} />}
        tone="success"
        title="CI/CD Golden Path"
        subtitle={t('goldenPathPage.description', 'Start quickly with a validated CI/CD tool combination.')}
      />

      {/* Search */}
      <div className="mb-5 max-w-[360px]">
        <SearchInput
          wrapperClassName="w-full"
          placeholder={t('goldenPathPage.searchPlaceholder', 'Search Golden Paths...')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Golden Path cards */}
      {isLoading ? (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t('common.loading', 'Loading...')}
        </div>
      ) : filtered.length === 0 ? (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t('goldenPathPage.empty.search', 'No matching results.')}
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
              <div className="grid grid-cols-2 gap-3 rounded-lg bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3">
                <div>
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    {t('stackTemplatePage.card.estimatedTime', 'Estimated Time')}
                  </span>
                  <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {t('stackTemplatePage.card.minutes', '{{minutes}} min', { minutes: goldenPath.estimated_install_time })}
                  </p>
                </div>
                <div>
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    {t('stackTemplatePage.card.recommendedUse', 'Recommended Use')}
                  </span>
                  <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {goldenPath.recommended_use_case}
                  </p>
                </div>
                <div className="col-span-2">
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    {t('stackTemplatePage.card.minResources', 'Minimum Resources')}
                  </span>
                  <p className="m-0 mt-1 text-[12px] text-[var(--color-text-primary)]">
                    {goldenPath.min_resources}
                  </p>
                </div>
              </div>

              {/* Tools preview */}
              <div>
                <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                  {t('stackTemplatePage.card.includedTools', 'Included Tools')}
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
                    <span className="rounded-md bg-[color-mix(in_srgb,_var(--color-text-muted)_12%,_transparent)] px-2 py-1 text-[11px] font-semibold text-[var(--color-text-muted)]">
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
                  <ExternalLink {...iconProps('xs')} />
                  {t('goldenPathPage.actions.viewDetail', 'View Detail')}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {filtered.length === 0 && !isLoading && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t('goldenPathPage.empty.search', 'No matching results.')}
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
              {t('common.close', 'Close')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              className="bg-[linear-gradient(135deg,var(--color-success),var(--color-success))] text-white"
              onClick={() => {
                if (selectedPath) {
                  const overrides = goldenPathToStackOverrides(selectedPath.tools)
                  loadFromTemplate(selectedPath.id, overrides)
                  navigate('/stack/install')
                }
              }}
            >
              {t('goldenPathPage.actions.use', 'Use this Golden Path')}
            </Button>
          </>
        }
      >
        {selectedPath && (
          <div className="flex flex-col gap-4">
            <div>
              <h4 className="mb-1 text-sm font-semibold text-[var(--color-text-primary)]">
                {t('goldenPathPage.detail.description', 'Description')}
              </h4>
              <p className="m-0 text-sm text-[var(--color-text-secondary)]">
                {selectedPath.description}
              </p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  {t('stackTemplatePage.card.estimatedTime', 'Estimated Time')}
                </span>
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {t('stackTemplatePage.card.minutes', '{{minutes}} min', { minutes: selectedPath.estimated_install_time })}
                </p>
              </div>
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  {t('stackTemplatePage.card.recommendedUse', 'Recommended Use')}
                </span>
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {selectedPath.recommended_use_case}
                </p>
              </div>
              <div className="col-span-2">
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  {t('stackTemplatePage.card.minResources', 'Minimum Resources')}
                </span>
                <p className="m-0 mt-1 text-sm text-[var(--color-text-primary)]">
                  {selectedPath.min_resources}
                </p>
              </div>
            </div>

            <div>
              <h4 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">
                {t('stackTemplatePage.card.includedTools', 'Included Tools')}
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
