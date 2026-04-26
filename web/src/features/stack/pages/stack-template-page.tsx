import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { BookOpen, ChevronDown, ChevronRight, Clock, Pencil, Plus, Search, Trash2, User, Wrench, X } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useCreateTemplate, useDeleteTemplate, useTemplates, useUpdateTemplate } from '../api/stack-api'
import { getToolAppVersion, getToolChartVersion, useStackConfigStore } from '../stores/stack-config-store'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { useAuthStore } from '../../../stores/auth-store'
import type { StackTemplate } from '../api/stack-api'
import type { TemplateToolDetail } from '../../../types'
import { resolveLocale } from '../../../lib/locale'
import { buildInstallOverridesFromTemplate, resolveToolIdByName } from '../utils/template-overrides'

interface TemplateFormState {
  id: string
  name: string
  description: string
  tools: ToolEntry[]
  recommendedUseCase: string
  minResources: string
}

interface ToolEntry {
  category: string
  name: string
  helm_version: string
  app_version: string
}

interface ToolCategoryDefinition {
  category: string
  label: string
  options: string[]
}

interface ToolSectionDefinition {
  id: string
  label: string
  categories: ToolCategoryDefinition[]
}

const TOOL_SECTIONS: ToolSectionDefinition[] = [
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

type ToolCategory = ToolCategoryDefinition
type AddToolDraft = { category: string; name: string; helm_version: string; app_version: string }

const TOOL_CATEGORY_LOOKUP = new Map<string, ToolCategory>(
  TOOL_SECTIONS.flatMap((section) => section.categories.map((category) => [category.category, category] as const))
)

const defaultVersionsForTool = (toolName: string) => {
  const toolId = resolveToolIdByName(toolName)
  return {
    helm_version: getToolChartVersion(toolId) ?? '',
    app_version: getToolAppVersion(toolId),
  }
}

const TEMPLATE_DESCRIPTION_I18N: Record<string, { ko: string; en: string }> = {
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

const TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES: Record<string, { ko: string; en: string }> = {
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

const NS_PER_MINUTE = 60 * 1_000_000_000
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

const normalizeDisplayEstimateMinutes = (minutes: number): number => {
  if (minutes <= DISPLAY_ESTIMATE_MIN_MINUTES) {
    return DISPLAY_ESTIMATE_MIN_MINUTES
  }

  if (minutes <= DISPLAY_ESTIMATE_MAX_MINUTES) {
    return minutes
  }

  const compressed =
    DISPLAY_ESTIMATE_MIN_MINUTES +
    Math.floor((minutes - DISPLAY_ESTIMATE_MAX_MINUTES) / 4)

  return Math.min(DISPLAY_ESTIMATE_MAX_MINUTES, compressed)
}

const buildInitialSectionOpenState = () =>
  Object.fromEntries(TOOL_SECTIONS.map((section) => [section.id, true])) as Record<string, boolean>







const OBSERVABILITY_CATEGORY_ORDER = ['monitoring_collection', 'monitoring_visualization', 'log_search', 'agent', 'trace_layer']

const getSectionCategories = (section: ToolSectionDefinition): ToolCategoryDefinition[] => {
  if (section.id !== 'observability') {
    return section.categories
  }

  const orderMap = new Map(OBSERVABILITY_CATEGORY_ORDER.map((category, index) => [category, index]))
  return [...section.categories].sort((a, b) => (orderMap.get(a.category) ?? 99) - (orderMap.get(b.category) ?? 99))
}

const addToolDraftKey = (sectionId: string, category: string) => `${sectionId}:${category}`

const buildInitialAddToolDrafts = () =>
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

const toToolEntry = (toolName: string, detail?: Partial<TemplateToolDetail>): ToolEntry => {
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

const EMPTY_TEMPLATE_FORM: TemplateFormState = {
  id: '',
  name: '',
  description: '',
  tools: [],
  recommendedUseCase: '',
  minResources: '',
}

const normalizeToolKey = (name: string) => name.trim().toLowerCase()

const toTemplateSlug = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')

const createTemplateUUID = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  return `${Date.now()}-${Math.random().toString(16).slice(2, 10)}-${Math.random().toString(16).slice(2, 10)}`
}

const estimateInstallMinutesFromTools = (tools: ToolEntry[]): number => {
  if (tools.length === 0) {
    return DISPLAY_ESTIMATE_MIN_MINUTES
  }

  const total = tools.reduce((sum, tool) => {
    const categoryCost = CATEGORY_MINUTES[tool.category] ?? 6
    const bonus = TOOL_BONUS_MINUTES[normalizeToolKey(tool.name)] ?? 0
    return sum + categoryCost + bonus
  }, ESTIMATE_BASE_MINUTES)

  const rounded = Math.max(ESTIMATE_BASE_MINUTES, Math.round(total))
  return normalizeDisplayEstimateMinutes(rounded)
}

const estimateInstallMinutesForTemplate = (template: StackTemplate): number => {
  const toolsFromDetails = (template.toolDetails ?? [])
    .filter((tool) => tool.name && tool.name.trim().length > 0)
    .map((tool) => toToolEntry(tool.name, tool))

  const targetTools = toolsFromDetails.length > 0
    ? toolsFromDetails
    : (Array.isArray(template.tools) ? template.tools : []).map((toolName) => toToolEntry(toolName))

  return estimateInstallMinutesFromTools(targetTools)
}
export function StackTemplatePage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { data: apiTemplates } = useTemplates()
  const createTemplate = useCreateTemplate()
  const updateTemplate = useUpdateTemplate()
  const deleteTemplate = useDeleteTemplate()
  const role = useAuthStore((state) => state.role)
  const isAdmin = role === 'admin'
  const { setTemplate, loadFromTemplate } = useStackConfigStore()
  const [search, setSearch] = useState('')
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [deleteTemplateId, setDeleteTemplateId] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [form, setForm] = useState<TemplateFormState>(EMPTY_TEMPLATE_FORM)
  const [customSections, setCustomSections] = useState<ToolSectionDefinition[]>([])
  const [removedBaseSectionIds, setRemovedBaseSectionIds] = useState<string[]>([])
  const [addSectionOpen, setAddSectionOpen] = useState(false)
  const [newSectionLabel, setNewSectionLabel] = useState('')
  const [newCategoryLabel, setNewCategoryLabel] = useState('')
  const [newCategoryOptions, setNewCategoryOptions] = useState('')

  const visibleBaseSections = useMemo(() => {
    if (!editingTemplateId) {
      return TOOL_SECTIONS.filter((section) => !removedBaseSectionIds.includes(section.id))
    }

    return TOOL_SECTIONS.filter((section) => {
      if (removedBaseSectionIds.includes(section.id)) {
        return false
      }
      const categoryIds = new Set(section.categories.map((category) => category.category))
      return form.tools.some((tool) => categoryIds.has(tool.category))
    })
  }, [editingTemplateId, form.tools, removedBaseSectionIds])

  const allSections = [...visibleBaseSections, ...customSections]
  const estimatedInstallMinutes = useMemo(() => estimateInstallMinutesFromTools(form.tools), [form.tools])
  const estimatedInstallTimeNs = estimatedInstallMinutes * NS_PER_MINUTE

  const [openToolSections, setOpenToolSections] = useState<Record<string, boolean>>(buildInitialSectionOpenState)
  const [addToolDrafts, setAddToolDrafts] = useState<Record<string, AddToolDraft>>(buildInitialAddToolDrafts)

  const templates = Array.isArray(apiTemplates) ? apiTemplates : []

  const isKorean = resolveLocale(i18n.resolvedLanguage || i18n.language) === 'ko-KR'
  const resolveTemplateDescription = (template: StackTemplate) => {
    const localized = TEMPLATE_DESCRIPTION_I18N[template.id]
    const rawDescription = (template.description ?? '').trim()
    const override = TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES[rawDescription]

    if (override) {
      return isKorean ? override.ko : override.en
    }

    if (!localized) return template.description

    if (
      rawDescription.length > 0
      && rawDescription !== localized.en
      && rawDescription !== localized.ko
    ) {
      return rawDescription
    }

    return isKorean ? localized.ko : localized.en
  }

  const filtered = templates.filter((template) => {
    const localizedDescription = resolveTemplateDescription(template).toLowerCase()
    const normalizedSearch = search.toLowerCase()
    return (
      template.name.toLowerCase().includes(normalizedSearch) ||
      localizedDescription.includes(normalizedSearch) ||
      template.tools.some((tool) => tool.toLowerCase().includes(normalizedSearch))
    )
  })

  const selectedTemplate = selectedTemplateId ? templates.find((template) => template.id === selectedTemplateId) ?? null : null

  const selectedDetail = selectedTemplate
    ? {
      fullDescription: resolveTemplateDescription(selectedTemplate),
      resource: selectedTemplate.minResources ?? t('stackTemplatePage.modal.na', 'N/A'),
      compatibility: selectedTemplate.recommendedUseCase ?? t(
        'stackTemplatePage.modal.compatibilityFallback',
        'Compatibility details are managed in Stack Version.'
      ),
    }
    : null

  const handleUseTemplate = (template: StackTemplate) => {
    const overrides = buildInstallOverridesFromTemplate(template)
    setTemplate(template.id)
    loadFromTemplate(template.id, overrides)
    navigate(`/stack/install?template=${template.id}`)
  }

  const nextDuplicateTemplateId = (templateId: string) => {
    const existing = new Set(templates.map((item) => item.id))
    const base = `${toTemplateSlug(templateId) || 'template'}-copy`

    if (!existing.has(base)) {
      return base
    }

    let index = 2
    while (existing.has(`${base}-${index}`)) {
      index += 1
    }

    return `${base}-${index}`
  }

  const handleDuplicateTemplate = (template: StackTemplate) => {
    const duplicateName = `${template.name} ${t('stackTemplatePage.duplicate.suffix', 'Duplicate')}`
    const duplicateId = nextDuplicateTemplateId(template.id)
    const duplicateMinutes = estimateInstallMinutesForTemplate(template)
    const duplicateNs = duplicateMinutes * NS_PER_MINUTE
    const duplicatedTools = (template.toolDetails && template.toolDetails.length > 0)
      ? template.toolDetails.map((tool) => ({
        category: tool.category,
        name: tool.name,
        helm_version: tool.helm_version,
        app_version: tool.app_version,
      }))
      : (Array.isArray(template.tools) ? template.tools : []).map((toolName) => toToolEntry(toolName))

    createTemplate.mutate(
      {
        id: duplicateId,
        name: duplicateName,
        description: resolveTemplateDescription(template),
        tools: duplicatedTools,
        estimated_install_time: duplicateNs,
        recommended_use_case: template.recommendedUseCase ?? '',
        min_resources: template.minResources ?? '',
      },
      {
        onError: () => {
          setFormError(t('stackTemplatePage.errors.duplicateFailed', 'Failed to duplicate template.'))
        },
      }
    )
  }

  const resetForm = () => {
    setForm(EMPTY_TEMPLATE_FORM)
    setFormError(null)
    setEditingTemplateId(null)
    setOpenToolSections(buildInitialSectionOpenState)
    setAddToolDrafts(buildInitialAddToolDrafts)
    setRemovedBaseSectionIds([])
  }

  const openCreateModal = () => {
    resetForm()
    setForm((prev) => ({ ...prev, id: createTemplateUUID() }))
    setFormOpen(true)
  }

  const openEditModal = (template: StackTemplate) => {
    const toolsFromDetail = (template.toolDetails ?? [])
      .filter((tool) => tool.name && tool.name.trim().length > 0)
      .map((tool) => toToolEntry(tool.name, tool))

    const tools = toolsFromDetail.length > 0
      ? toolsFromDetail
      : (Array.isArray(template.tools) ? template.tools : []).map((toolName) => toToolEntry(toolName))

    const toolCategoryIds = new Set(tools.map((tool) => tool.category))
    const removedInTemplate = TOOL_SECTIONS
      .filter((section) => !section.categories.some((category) => toolCategoryIds.has(category.category)))
      .map((section) => section.id)

    setRemovedBaseSectionIds(removedInTemplate)
    setEditingTemplateId(template.id)
    setFormError(null)
    setForm({
      id: template.id,
      name: template.name,
      description: resolveTemplateDescription(template),
      tools,
      recommendedUseCase: template.recommendedUseCase ?? '',
      minResources: template.minResources ?? '',
    })
    setFormOpen(true)
  }

  const closeFormModal = () => {
    setFormOpen(false)
    resetForm()
  }

  const handleFormChange = (key: keyof TemplateFormState, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const toggleToolSection = (sectionId: string) => {
    setOpenToolSections((prev) => ({ ...prev, [sectionId]: !prev[sectionId] }))
  }




  const updateAddToolDraft = (draftKey: string, patch: Partial<AddToolDraft>) => {
    setAddToolDrafts((prev) => {
      const current = prev[draftKey]
      if (!current) return prev
      return {
        ...prev,
        [draftKey]: {
          ...current,
          ...patch,
        },
      }
    })
  }

  const submitAddTool = (draftKey: string) => {
    const draft = addToolDrafts[draftKey]
    if (!draft?.category || !draft.name) {
      return
    }

    setForm((prev) => {
      const existingIndex = prev.tools.findIndex((tool) => tool.category === draft.category)
      const nextEntry: ToolEntry = {
        category: draft.category,
        name: draft.name,
        helm_version: draft.helm_version,
        app_version: draft.app_version,
      }

      if (existingIndex >= 0) {
        return {
          ...prev,
          tools: prev.tools.map((tool, index) => (index === existingIndex ? nextEntry : tool)),
        }
      }

      return {
        ...prev,
        tools: [...prev.tools, nextEntry],
      }
    })
  }

  const removeCategoryTool = (sectionId: string, category: string) => {
    setForm((prev) => ({
      ...prev,
      tools: prev.tools.filter((tool) => tool.category !== category),
    }))

    const categoryMeta = TOOL_CATEGORY_LOOKUP.get(category)
    const defaultName = categoryMeta?.options[0] ?? ''
    const defaults = defaultVersionsForTool(defaultName)
    const draftKey = addToolDraftKey(sectionId, category)
    updateAddToolDraft(draftKey, {
      category,
      name: defaultName,
      ...defaults,
    })
  }

  const addSection = () => {
    if (!newSectionLabel.trim() || !newCategoryLabel.trim()) return
    const sectionId = newSectionLabel.toLowerCase().replace(/\s+/g, '-')
    const categoryId = `${sectionId}_${newCategoryLabel.toLowerCase().replace(/\s+/g, '_')}`
    const options = newCategoryOptions.split(',').map((o) => o.trim()).filter(Boolean)

    const newSection: ToolSectionDefinition = {
      id: sectionId,
      label: newSectionLabel.trim(),
      categories: [{ category: categoryId, label: newCategoryLabel.trim(), options: options.length > 0 ? options : ['Custom Tool'] }],
    }
    setCustomSections((prev) => [...prev, newSection])
    setOpenToolSections((prev) => ({ ...prev, [sectionId]: true }))
    setAddToolDrafts((prev) => { const defaultName = options[0] ?? 'Custom Tool'; const defaults = defaultVersionsForTool(defaultName); return { ...prev, [addToolDraftKey(sectionId, categoryId)]: { category: categoryId, name: defaultName, ...defaults } } })
    setNewSectionLabel('')
    setNewCategoryLabel('')
    setNewCategoryOptions('')
    setAddSectionOpen(false)
  }

  const removeSection = (sectionId: string) => {
    const section = allSections.find((s) => s.id === sectionId) ?? TOOL_SECTIONS.find((s) => s.id === sectionId)
    if (!section) return
    const categoryIds = new Set(section.categories.map((c) => c.category))
    setForm((prev) => ({ ...prev, tools: prev.tools.filter((t) => !categoryIds.has(t.category)) }))

    if (TOOL_SECTIONS.some((base) => base.id === sectionId)) {
      setRemovedBaseSectionIds((prev) => (prev.includes(sectionId) ? prev : [...prev, sectionId]))
    } else {
      setCustomSections((prev) => prev.filter((s) => s.id !== sectionId))
      setOpenToolSections((prev) => { const next = { ...prev }; delete next[sectionId]; return next })
      setAddToolDrafts((prev) => { const next = { ...prev }; Object.keys(next).forEach((key) => { if (key.startsWith(`${sectionId}:`)) delete next[key] }); return next })
    }
  }



  const submitTemplate = () => {
    const templateId = editingTemplateId ?? (form.id || createTemplateUUID())

    const payload = {
      id: templateId,
      name: form.name,
      description: form.description,
      tools: form.tools,
      estimated_install_time: estimatedInstallTimeNs,
      recommended_use_case: form.recommendedUseCase,
      min_resources: form.minResources,
    }

    if (editingTemplateId) {
      updateTemplate.mutate(
        { ...payload, id: editingTemplateId },
        {
          onSuccess: () => {
            closeFormModal()
          },
          onError: () => {
            setFormError(t('stackTemplatePage.errors.updateFailed', 'Failed to update template.'))
          },
        }
      )
      return
    }

    createTemplate.mutate(payload, {
      onSuccess: () => {
        closeFormModal()
      },
      onError: () => {
        setFormError(t('stackTemplatePage.errors.createFailed', 'Failed to create template.'))
      },
    })
  }

  const handleDeleteTemplate = () => {
    if (!deleteTemplateId) return
    deleteTemplate.mutate(deleteTemplateId, {
      onSuccess: () => {
        setDeleteTemplateId(null)
      },
    })
  }

  return (
    <div>
      <Breadcrumb
        items={[
          { label: t('stackTemplatePage.breadcrumb.stackList', 'Stack List'), path: '/stack/list' },
          { label: t('stackTemplatePage.breadcrumb.current', 'Stack Template') },
        ]}
      />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between gap-4">
        <div className="mb-2 flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(16,185,129,0.15)] text-[#34d399]">
            <BookOpen size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('stackTemplatePage.title', 'Stack Template')}
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackTemplatePage.description', 'Select a validated DevSecOps stack template to get started quickly.')}
            </p>
          </div>
        </div>
        {isAdmin && (
          <Button variant="primary" size="md" type="button" onClick={openCreateModal}>
            <Plus size={15} />
            {t('stackTemplatePage.actions.createTemplate', 'Create Template')}
          </Button>
        )}
      </div>

      {/* Search */}
      <div className="mb-5 max-w-[360px]">
        <div className="relative">
          <Search
            size={13}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
          <input
            placeholder={t('stackTemplatePage.searchPlaceholder', 'Search templates...')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
          />
        </div>
      </div>

      {/* Template cards */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,460px),1fr))] gap-[var(--grid-gap)]">
        {filtered.map((template) => (
          <div
            key={template.id}
            role="button"
            tabIndex={0}
            onClick={() => setSelectedTemplateId(template.id)}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setSelectedTemplateId(template.id) }}
            className="flex h-full cursor-pointer flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] text-left transition-colors duration-150 hover:border-[var(--color-border-hover)]"
          >
            {/* Card header */}
            <div>
              <h3 className="mb-1.5 mt-0 text-[15px] font-bold text-[var(--color-text-primary)]">
                {template.name}
              </h3>
              <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                {resolveTemplateDescription(template)}
              </p>
            </div>

            {/* Tools */}
            <div className="flex flex-wrap gap-1.5">
              {template.tools.map((tool) => (
                <span
                  key={tool}
                  className="rounded-md bg-[rgba(99,102,241,0.12)] px-2 py-[3px] text-[11px] font-medium text-[#a5b4fc]"
                >
                  {tool}
                </span>
              ))}
            </div>

            {/* Footer */}
            <div className="mt-auto flex items-center justify-between border-t border-[var(--color-border-default)] pt-2.5">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-[5px] text-xs text-[var(--color-text-secondary)]">
                  <Clock size={13} />
                  <span>{t('stackTemplatePage.card.estimatedMinutes', '{{minutes}} min', { minutes: estimateInstallMinutesForTemplate(template) })}</span>
                </div>
                {template.createdBy && (
                  <div className="flex items-center gap-[5px] text-xs text-[var(--color-text-muted)]">
                    <User size={12} />
                    <span>{template.createdBy}</span>
                  </div>
                )}
              </div>
                <div className="flex items-center gap-1.5">
                  {isAdmin && (
                    <>
                    <Button
                      variant="ghost"
                      size="sm"
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        openEditModal(template)
                      }}
                    >
                      <Pencil size={13} />
                      {t('stackTemplatePage.actions.edit', 'Edit')}
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        setDeleteTemplateId(template.id)
                      }}
                    >
                      <Trash2 size={13} />
                      {t('stackTemplatePage.actions.delete', 'Delete')}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        handleDuplicateTemplate(template)
                      }}
                    >
                      {t('stackTemplatePage.actions.duplicateTemplate', 'Duplicate Template')}
                    </Button>
                  </>
                )}
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  className="whitespace-nowrap bg-[linear-gradient(135deg,#facc15,#eab308)] text-[#111827]"
                  onClick={(event) => {
                    event.stopPropagation()
                    handleUseTemplate(template)
                  }}
                >
                  {t('stackTemplatePage.actions.useBaseTemplate', 'Use Base Template')}
                </Button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t('stackTemplatePage.empty', 'No search results found.')}
        </div>
      )}

      <Modal
        open={selectedTemplate !== null}
        onClose={() => setSelectedTemplateId(null)}
        title={selectedTemplate?.name ?? t('stackTemplatePage.modal.templateDetail', 'Template Detail')}
        wide
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setSelectedTemplateId(null)} type="button">
              {t('stackTemplatePage.actions.close', 'Close')}
            </Button>
            {selectedTemplate && (
              <Button
                variant="primary"
                size="sm"
                type="button"
                onClick={() => {
                  setSelectedTemplateId(null)
                  handleUseTemplate(selectedTemplate)
                }}
              >
                {t('stackTemplatePage.actions.baseTemplate', 'Base Template')}
              </Button>
            )}
          </>
        }
      >
        {selectedTemplate && selectedDetail && (
          <div className="flex flex-col gap-4">
            <div>
              <div className="mb-1.5 text-[13px] text-[var(--color-text-secondary)]">{t('stackTemplatePage.modal.description', 'Description')}</div>
              <p className="m-0 text-sm leading-[1.7] text-[var(--color-text-primary)]">
                {selectedDetail.fullDescription}
              </p>
            </div>

            <div>
              <div className="mb-2 text-[13px] text-[var(--color-text-secondary)]">{t('stackTemplatePage.modal.includedTools', 'Included Tools')}</div>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
                {selectedTemplate.tools.map((tool) => (
                  <div
                    key={tool}
                    className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-2.5 py-2"
                  >
                    <Wrench size={13} color="#fbbf24" />
                    <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">{tool}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-lg border border-[var(--color-border-default)] p-3">
                <div className="mb-1.5 text-xs text-[var(--color-text-secondary)]">{t('stackTemplatePage.modal.estimatedDeployTime', 'Estimated Deploy Time')}</div>
                <div className="text-base font-bold text-[#fcd34d]">
                  {t('stackTemplatePage.modal.minutes', '{{minutes}} minutes', { minutes: estimateInstallMinutesForTemplate(selectedTemplate) })}
                </div>
              </div>
              <div className="rounded-lg border border-[var(--color-border-default)] p-3">
                <div className="mb-1.5 text-xs text-[var(--color-text-secondary)]">{t('stackTemplatePage.modal.resourceRequirements', 'Resource Requirements')}</div>
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">{selectedDetail.resource}</div>
              </div>
            </div>

            <div className="rounded-lg border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.08)] p-3">
              <div className="mb-1.5 text-xs font-bold text-[#86efac]">{t('stackTemplatePage.modal.compatibility', 'Compatibility')}</div>
              <div className="text-[13px] text-[var(--color-text-primary)]">{selectedDetail.compatibility}</div>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={formOpen}
        onClose={closeFormModal}
        title={editingTemplateId ? t('stackTemplatePage.modal.editTitle', 'Edit Template') : t('stackTemplatePage.modal.createTitle', 'Create Template')}
        wide
        footer={
          <>
            <Button variant="outline" size="sm" onClick={closeFormModal} type="button">
              {t('common.cancel', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={submitTemplate}
              loading={createTemplate.isPending || updateTemplate.isPending}
            >
              {editingTemplateId ? t('common.save', 'Save') : t('stackTemplatePage.actions.create', 'Create')}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label={t('stackTemplatePage.form.templateId', 'Template ID')}
              value={editingTemplateId ?? form.id}
              onChange={(event) => handleFormChange('id', event.target.value)}
              disabled={editingTemplateId !== null}
            />
            <Input
              label={t('stackTemplatePage.form.name', 'Name')}
              value={form.name}
              onChange={(event) => handleFormChange('name', event.target.value)}
            />
          </div>
          <Input
            label={t('stackTemplatePage.form.description', 'Description')}
            value={form.description}
            onChange={(event) => handleFormChange('description', event.target.value)}
          />
          <div className="flex flex-col gap-2">
            <div className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              {t('stackTemplatePage.form.tools', 'Tools')}
            </div>
            <div className="text-[11px] text-[var(--color-text-muted)]">
              {t('stackTemplatePage.form.toolsHint', 'Create the template scaffold first, then add or refine tools by section.')}
            </div>
            <div className="rounded-lg border border-[var(--color-border-default)]">
              {allSections.map((section) => {
                const isOpen = openToolSections[section.id]
                const sectionCategories = getSectionCategories(section)
                return (
                  <div key={section.id} className="border-b border-[var(--color-border-default)] last:border-b-0">
                    <div className="flex items-center bg-[rgba(255,255,255,0.03)]">
                      <button
                        type="button"
                        onClick={() => toggleToolSection(section.id)}
                        className="flex flex-1 cursor-pointer items-center justify-between px-3 py-2 text-left"
                      >
                        <span className="text-sm font-semibold text-[var(--color-text-primary)]">{section.label}</span>
                        {isOpen ? (
                          <ChevronDown size={15} className="text-[var(--color-text-secondary)]" />
                        ) : (
                          <ChevronRight size={15} className="text-[var(--color-text-secondary)]" />
                        )}
                      </button>
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); removeSection(section.id) }}
                        className="mr-2 flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent text-[var(--color-text-muted)] transition-colors hover:border-[rgba(248,113,113,0.5)] hover:text-[#f87171]"
                        title={t('stackTemplatePage.actions.removeSection', 'Remove {{section}} section', { section: section.label })}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>

                    {isOpen && (
                      <div className="flex flex-col gap-2 px-3 py-3">
                        <div className="flex flex-col gap-2 rounded-lg border border-dashed border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-2">
                          {sectionCategories.map((category) => {
                            const draftKey = addToolDraftKey(section.id, category.category)
                            const existing = form.tools.find((tool) => tool.category === category.category)
                            const baseDraft = addToolDrafts[draftKey]
                            const defaultName = category.options[0] ?? ''
                            const name = existing?.name || baseDraft?.name || defaultName
                            const defaults = defaultVersionsForTool(name)
                            const helmVersion = existing?.helm_version || baseDraft?.helm_version || defaults.helm_version
                            const appVersion = existing?.app_version || baseDraft?.app_version || defaults.app_version
                            const hasApplied = !!existing
                            return (
                              <div key={draftKey} className="grid gap-2 sm:grid-cols-[minmax(0,0.7fr)_minmax(0,1fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_auto_auto]">
                                <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-[9px] text-sm text-[var(--color-text-secondary)]">
                                  {category.label}
                                </div>
                                <select
                                  value={name}
                                  onChange={(event) => {
                                    const nextName = event.target.value
                                    const nextDefaults = defaultVersionsForTool(nextName)
                                    updateAddToolDraft(draftKey, {
                                      category: category.category,
                                      name: nextName,
                                      helm_version: nextDefaults.helm_version,
                                      app_version: nextDefaults.app_version,
                                    })
                                  }}
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                                >
                                  {category.options.map((option) => (
                                    <option key={option} value={option}>
                                      {option}
                                    </option>
                                  ))}
                                </select>
                                <input
                                  type="text"
                                  value={helmVersion}
                                  onChange={(event) => updateAddToolDraft(draftKey, { category: category.category, name, helm_version: event.target.value, app_version: appVersion })}
                                  placeholder={t('stackTemplatePage.form.helmVersionPlaceholder', 'Helm version')}
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                                />
                                <input
                                  type="text"
                                  value={appVersion}
                                  onChange={(event) => updateAddToolDraft(draftKey, { category: category.category, name, helm_version: helmVersion, app_version: event.target.value })}
                                  placeholder={t('stackTemplatePage.form.appVersionPlaceholder', 'App version')}
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                                />
                                <Button variant="outline" size="sm" type="button" onClick={() => submitAddTool(draftKey)}>
                                  {hasApplied ? t('stackTemplatePage.actions.updateTool', 'Update Tool') : t('stackTemplatePage.actions.addTool', 'Add Tool')}
                                </Button>
                                <button
                                  type="button"
                                  aria-label={`Remove ${name}`}
                                  onClick={() => removeCategoryTool(section.id, category.category)}
                                  className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-lg border border-[var(--color-border-default)] text-[var(--color-text-secondary)] transition-colors hover:border-[rgba(248,113,113,0.5)] hover:text-[#f87171]"
                                >
                                  <X size={15} />
                                </button>
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>

            {addSectionOpen ? (
              <div className="mt-2 flex flex-col gap-2 rounded-lg border border-dashed border-[var(--color-border-default)] p-3">
                <input
                  type="text"
                  value={newSectionLabel}
                  onChange={(e) => setNewSectionLabel(e.target.value)}
                  placeholder={t('stackTemplatePage.form.sectionNamePlaceholder', 'Section name (e.g. Security)')}
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <input
                  type="text"
                  value={newCategoryLabel}
                  onChange={(e) => setNewCategoryLabel(e.target.value)}
                  placeholder={t('stackTemplatePage.form.firstCategoryPlaceholder', 'First category (e.g. Scanner)')}
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <input
                  type="text"
                  value={newCategoryOptions}
                  onChange={(e) => setNewCategoryOptions(e.target.value)}
                  placeholder={t('stackTemplatePage.form.toolOptionsPlaceholder', 'Tool options (comma separated, e.g. Trivy, SonarQube)')}
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={addSection}>{t('stackTemplatePage.actions.addSection', 'Add Section')}</Button>
                  <Button variant="ghost" size="sm" type="button" onClick={() => setAddSectionOpen(false)}>{t('common.cancel', 'Cancel')}</Button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setAddSectionOpen(true)}
                className="mt-2 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-dashed border-[var(--color-border-default)] px-3 py-2.5 text-sm text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-border-hover)] hover:text-[var(--color-text-primary)]"
              >
                <Plus size={14} />
                {t('stackTemplatePage.actions.addSection', 'Add Section')}
              </button>
            )}
          </div>
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2.5">
            <div className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              {t('stackTemplatePage.form.estimatedInstallTimeAuto', 'Estimated Install Time (Auto)')}
            </div>
            <div className="mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
              {t('stackTemplatePage.form.estimatedInstallTimeValue', '{{minutes}} min ({{nanoseconds}} ns)', {
                minutes: estimatedInstallMinutes,
                nanoseconds: estimatedInstallTimeNs,
              })}
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label={t('stackTemplatePage.form.recommendedUseCase', 'Recommended Use Case')}
              value={form.recommendedUseCase}
              onChange={(event) => handleFormChange('recommendedUseCase', event.target.value)}
            />
            <Input
              label={t('stackTemplatePage.form.minimumResources', 'Minimum Resources')}
              value={form.minResources}
              onChange={(event) => handleFormChange('minResources', event.target.value)}
            />
          </div>
          {formError && <div className="text-xs text-[#f87171]">{formError}</div>}
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteTemplateId !== null}
        onClose={() => setDeleteTemplateId(null)}
        onConfirm={handleDeleteTemplate}
        title={t('stackTemplatePage.confirm.deleteTitle', 'Delete Template')}
        description={t('stackTemplatePage.confirm.deleteDescription', 'This template will no longer be shown in the list. Continue?')}
        confirmLabel={t('stackTemplatePage.confirm.deleteConfirmLabel', 'Delete Template')}
        loading={deleteTemplate.isPending}
      />
    </div>
  )
}
