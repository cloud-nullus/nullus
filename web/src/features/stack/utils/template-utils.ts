import { getToolAppVersion, getToolChartVersion } from '../stores/stack-config-store'
import { resolveToolIdByName } from './template-overrides'
import type { StackTemplate } from '../api/stack-api'
import type { TemplateToolDetail } from '../../../types'

export interface TemplateFormState {
  id: string
  name: string
  description: string
  tools: ToolEntry[]
  recommendedUseCase: string
  minResources: string
}

export interface ToolEntry {
  category: string
  name: string
  helm_version: string
  app_version: string
}

export interface ToolCategoryDefinition {
  category: string
  label: string
  options: string[]
}

export interface ToolSectionDefinition {
  id: string
  label: string
  categories: ToolCategoryDefinition[]
}

export type AddToolDraft = { category: string; name: string; helm_version: string; app_version: string }

export const TOOL_SECTIONS: ToolSectionDefinition[] = [
  {
    id: 'artifacts',
    label: 'Artifacts',
    categories: [
      { category: 'package_registry', label: 'Package Registry', options: ['Nexus', 'GitLab Package Registry', 'JFrog Artifactory'] },
      { category: 'source_repository', label: 'Source Repository', options: ['GitLab CE', 'Gitea', 'GitHub'] },
      { category: 'container_registry', label: 'Container Registry', options: ['Harbor', 'GitLab Registry', 'Docker Registry'] },
    ],
  },
  {
    id: 'cicd',
    label: 'CI/CD',
    categories: [
      { category: 'ci_platform', label: 'CI Platform', options: ['GitLab CI', 'Jenkins', 'GitHub Actions', 'Tekton'] },
      { category: 'cd_tool', label: 'CD Tool', options: ['Argo CD', 'Flux', 'Tekton'] },
    ],
  },
  {
    id: 'observability',
    label: 'Observability',
    categories: [
      { category: 'monitoring_collection', label: 'Metrics', options: ['Prometheus', 'Thanos', 'Victoria Metrics'] },
      { category: 'monitoring_visualization', label: 'Visualization', options: ['Grafana', 'OpenSearch Dashboards'] },
      { category: 'log_search', label: 'Logs', options: ['Loki', 'OpenSearch', 'Fluentd'] },
      { category: 'agent', label: 'Agent', options: ['OpenTelemetry Collector'] },
      { category: 'trace_layer', label: 'Traces', options: ['Tempo', 'Jaeger'] },
    ],
  },
]

export const TOOL_CATEGORY_LOOKUP = new Map<string, ToolCategoryDefinition>(
  TOOL_SECTIONS.flatMap((section) => section.categories.map((category) => [category.category, category] as const))
)

export const NS_PER_MINUTE = 60 * 1_000_000_000

const ESTIMATE_BASE_MINUTES = 5
const DISPLAY_ESTIMATE_MIN_MINUTES = 10
const DISPLAY_ESTIMATE_MAX_MINUTES = 19
const CATEGORY_MINUTES: Record<string, number> = {
  package_registry: 7,
  source_repository: 8,
  container_registry: 6,
  ci_platform: 9,
  cd_tool: 7,
  monitoring_collection: 6,
  monitoring_visualization: 3,
  log_search: 7,
  agent: 4,
  trace_layer: 5,
}
const TOOL_BONUS_MINUTES: Record<string, number> = {
  'gitlab ce': 2,
  jenkins: 4,
  thanos: 4,
  'victoria metrics': 3,
  opensearch: 3,
  elasticsearch: 4,
}

export const TEMPLATE_DESCRIPTION_I18N: Record<string, { ko: string; en: string }> = {
  'empty-template-v1': {
    ko: '아직 도구가 선택되지 않은 빈 스택 템플릿입니다.',
    en: 'An empty stack template with no tools selected yet.',
  },
  'gitlab-allinone-v1': {
    ko: 'GitLab 올인원 기반으로 소스/CI/CD를 한 번에 구성하는 스택입니다.',
    en: 'An all-in-one GitLab stack that configures source and CI/CD together.',
  },
  'gitlab-argocd-v1': {
    ko: 'GitLab과 ArgoCD를 결합해 Git 기반 CI와 GitOps CD를 함께 구성합니다.',
    en: 'Combines GitLab and ArgoCD to provide Git-based CI and GitOps CD together.',
  },
  'github-argocd-v1': {
    ko: 'GitHub와 ArgoCD 조합으로 GitHub 중심 개발팀에 최적화된 GitOps 스택입니다.',
    en: 'A GitOps stack optimized for GitHub-centric teams using GitHub and ArgoCD.',
  },
}

export const TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES: Record<string, { ko: string; en: string }> = {
  'Start from an empty stack configuration with every tool left unselected.': {
    ko: '아직 도구가 선택되지 않은 빈 스택 템플릿입니다.',
    en: 'An empty stack template with no tools selected yet.',
  },
  'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.': {
    ko: 'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    en: 'Single-platform stack based on GitLab CE. It provides source code management, CI/CD, and container registry in one integrated GitLab setup.',
  },
  'GitLab CI와 GitLab Registry를 사용하고 Argo CD로 GitOps 패턴을 강화한 구성입니다.': {
    ko: 'GitLab CI와 GitLab Registry를 사용하고 Argo CD로 GitOps 패턴을 강화한 구성입니다.',
    en: 'Uses GitLab CI and GitLab Registry, and strengthens the setup with Argo CD for a GitOps workflow.',
  },
  'GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.': {
    ko: 'GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.',
    en: 'Use GitHub and GitHub Actions as external services, and install only Harbor + Argo CD + monitoring in the cluster.',
  },
}

const OBSERVABILITY_CATEGORY_ORDER = ['monitoring_collection', 'monitoring_visualization', 'log_search', 'agent', 'trace_layer']

export const defaultVersionsForTool = (toolName: string) => {
  const toolId = resolveToolIdByName(toolName)
  return {
    helm_version: getToolChartVersion(toolId) ?? '',
    app_version: getToolAppVersion(toolId),
  }
}

const normalizeDisplayEstimateMinutes = (minutes: number): number => {
  if (minutes <= DISPLAY_ESTIMATE_MIN_MINUTES) return DISPLAY_ESTIMATE_MIN_MINUTES
  if (minutes <= DISPLAY_ESTIMATE_MAX_MINUTES) return minutes
  const compressed = DISPLAY_ESTIMATE_MIN_MINUTES + Math.floor((minutes - DISPLAY_ESTIMATE_MAX_MINUTES) / 4)
  return Math.min(DISPLAY_ESTIMATE_MAX_MINUTES, compressed)
}

export const buildInitialSectionOpenState = () =>
  Object.fromEntries(TOOL_SECTIONS.map((section) => [section.id, true])) as Record<string, boolean>

export const getSectionCategories = (section: ToolSectionDefinition): ToolCategoryDefinition[] => {
  if (section.id !== 'observability') return section.categories
  const orderMap = new Map(OBSERVABILITY_CATEGORY_ORDER.map((category, index) => [category, index]))
  return [...section.categories].sort((a, b) => (orderMap.get(a.category) ?? 99) - (orderMap.get(b.category) ?? 99))
}

export const addToolDraftKey = (sectionId: string, category: string) => `${sectionId}:${category}`

export const buildInitialAddToolDrafts = () =>
  Object.fromEntries(
    TOOL_SECTIONS.flatMap((section) => {
      const orderedCategories = getSectionCategories(section)
      return orderedCategories.map((category) => {
        const defaultName = category.options[0] ?? ''
        const defaults = defaultVersionsForTool(defaultName)
        return [
          addToolDraftKey(section.id, category.category),
          { category: category.category, name: defaultName, ...defaults },
        ]
      })
    })
  ) as Record<string, AddToolDraft>

export const toToolEntry = (toolName: string, detail?: Partial<TemplateToolDetail>): ToolEntry => {
  const matched = TOOL_SECTIONS
    .flatMap((section) => section.categories)
    .find((category) => category.options.includes(toolName))

  return {
    category: detail?.category ?? matched?.category ?? '',
    name: toolName,
    helm_version: detail?.helm_version ?? '',
    app_version: detail?.app_version ?? '',
  }
}

export const EMPTY_TEMPLATE_FORM: TemplateFormState = {
  id: '',
  name: '',
  description: '',
  tools: [],
  recommendedUseCase: '',
  minResources: '',
}

export const normalizeToolKey = (name: string) => name.trim().toLowerCase()

export const toTemplateSlug = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')

export const createTemplateUUID = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2, 10)}-${Math.random().toString(16).slice(2, 10)}`
}

export const estimateInstallMinutesFromTools = (tools: ToolEntry[]): number => {
  if (tools.length === 0) return DISPLAY_ESTIMATE_MIN_MINUTES

  const total = tools.reduce((sum, tool) => {
    const categoryCost = CATEGORY_MINUTES[tool.category] ?? 6
    const bonus = TOOL_BONUS_MINUTES[normalizeToolKey(tool.name)] ?? 0
    return sum + categoryCost + bonus
  }, ESTIMATE_BASE_MINUTES)

  return normalizeDisplayEstimateMinutes(Math.max(ESTIMATE_BASE_MINUTES, Math.round(total)))
}

export const estimateInstallMinutesForTemplate = (template: StackTemplate): number => {
  const toolsFromDetails = (template.toolDetails ?? [])
    .filter((tool) => tool.name && tool.name.trim().length > 0)
    .map((tool) => toToolEntry(tool.name, tool))

  const targetTools = toolsFromDetails.length > 0
    ? toolsFromDetails
    : (Array.isArray(template.tools) ? template.tools : []).map((toolName) => toToolEntry(toolName))

  return estimateInstallMinutesFromTools(targetTools)
}
