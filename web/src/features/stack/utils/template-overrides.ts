import type { StackTemplate, TemplateToolDetail } from '../../../types'
import type { StackConfigDraft } from '../stores/stack-config-store'
import { getToolAppVersion } from '../stores/stack-config-store'

const TOOL_ID_BY_NAME: Record<string, string> = {
  'gitlab ce': 'gitlab',
  'gitlab package registry': 'gitlab',
  'gitlab registry': 'gitlab-registry',
  'gitlab ci': 'gitlab-ci',
  'argo cd': 'argocd',
  minio: 'minio',
  prometheus: 'prometheus',
  grafana: 'grafana',
  opensearch: 'opensearch',
  tempo: 'tempo',
  nexus: 'nexus',
  'jfrog artifactory': 'jfrog',
  github: 'github',
  gitea: 'gitea',
  harbor: 'harbor',
  'docker registry': 'docker-hub',
  'github actions': 'github-actions',
  jenkins: 'jenkins',
  flux: 'flux',
  thanos: 'thanos',
  'victoria metrics': 'victoriametrics',
  kibana: 'kibana',
  'opensearch dashboards': 'opensearch-dashboards',
  jaeger: 'jaeger',
  'opentelemetry collector': 'opentelemetry-collector',
  elasticsearch: 'elasticsearch',
  loki: 'loki',
}

function normalizeToolKey(name: string): string {
  return name.trim().toLowerCase()
}

export function resolveToolIdByName(name: string): string {
  return TOOL_ID_BY_NAME[normalizeToolKey(name)] ?? normalizeToolKey(name)
}

type NormalizedTemplateTool = {
  category: string
  name: string
  app_version?: string
}

function normalizeTemplateTools(template: StackTemplate): NormalizedTemplateTool[] {
  const details = (template.toolDetails ?? [])
    .filter((tool): tool is TemplateToolDetail => Boolean(tool?.name && tool.name.trim().length > 0))
    .map((tool) => ({
      category: tool.category,
      name: tool.name,
      app_version: tool.app_version,
    }))

  if (details.length > 0) {
    return details
  }

  const legacyTools = Array.isArray(template.tools) ? template.tools : []
  return legacyTools.map((toolName) => ({
    category: '',
    name: toolName,
  }))
}

export function buildInstallOverridesFromTemplate(template: StackTemplate): Partial<StackConfigDraft> {
  const tools = normalizeTemplateTools(template)
  const hasExplicitPackageRegistry = tools.some((tool) => tool.category === 'package_registry')
  const hasGitLabTool = tools.some((tool) => {
    const toolId = resolveToolIdByName(tool.name)
    return toolId === 'gitlab' || toolId === 'gitlab-ci' || toolId === 'gitlab-registry'
  })

  const overrides: Partial<StackConfigDraft> = {
    artifacts: {
      packageRegistry: { tool: '', version: '' },
      sourceRepository: { tool: '', version: '' },
      containerRegistry: { tool: '', version: '' },
      storageBackend: { tool: '', version: '' },
    },
    pipeline: {
      cicdPlatform: { tool: '', version: '' },
      cdTool: { tool: '', version: '' },
    },
    monitoring: {
      collection: { tool: '', version: '' },
      visualization: { tool: '', version: '' },
      visualizations: [],
    },
    logging: {
      search: { tool: '', version: '' },
      traceLayer: { tool: '', version: '' },
      traceExporter: { tool: '', version: '' },
    },
    authentication: {
      provider: '',
    },
  }

  const apply = (
    target: 'artifacts' | 'pipeline' | 'monitoring' | 'logging',
    field: string,
    name: string,
    appVersion?: string
  ) => {
    const toolId = resolveToolIdByName(name)
    const version = appVersion || getToolAppVersion(toolId)
    ;(overrides[target] as unknown as Record<string, { tool: string; version: string }>)[field] = { tool: toolId, version }
  }

  for (const tool of tools) {
    switch (tool.category) {
      case 'package_registry':
        apply('artifacts', 'packageRegistry', tool.name, tool.app_version)
        break
      case 'source_repository':
        apply('artifacts', 'sourceRepository', tool.name, tool.app_version)
        break
      case 'container_registry':
        apply('artifacts', 'containerRegistry', tool.name, tool.app_version)
        break
      case 'storage_backend':
        apply('artifacts', 'storageBackend', tool.name, tool.app_version)
        break
      case 'ci_platform':
        apply('pipeline', 'cicdPlatform', tool.name, tool.app_version)
        break
      case 'cd_tool':
        apply('pipeline', 'cdTool', tool.name, tool.app_version)
        break
      case 'monitoring_collection':
        apply('monitoring', 'collection', tool.name, tool.app_version)
        break
      case 'monitoring_visualization':
        apply('monitoring', 'visualization', tool.name, tool.app_version)
        if (overrides.monitoring) {
          const toolId = resolveToolIdByName(tool.name)
          const version = tool.app_version || getToolAppVersion(toolId)
          const exists = overrides.monitoring.visualizations.some((item) => item.tool === toolId)
          if (!exists) {
            overrides.monitoring.visualizations.push({ tool: toolId, version })
          }
        }
        break
      case 'log_search':
        apply('logging', 'search', tool.name, tool.app_version)
        break
      case 'trace_layer':
        apply('logging', 'traceLayer', tool.name, tool.app_version)
        break
      case 'agent':
        apply('logging', 'traceExporter', tool.name, tool.app_version)
        break
      default:
        break
    }
  }

  if (hasGitLabTool && !hasExplicitPackageRegistry && overrides.artifacts) {
    overrides.artifacts.packageRegistry = {
      tool: 'gitlab',
      version: getToolAppVersion('gitlab'),
    }
  }

  return overrides
}
