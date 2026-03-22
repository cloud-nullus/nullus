import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, ChevronDown, ChevronRight, Clock, Pencil, Plus, Search, Trash2, User, Wrench, X } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useCreateTemplate, useDeleteTemplate, useTemplates, useUpdateTemplate } from '../api/stack-api'
import { useStackConfigStore } from '../stores/stack-config-store'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { useAuthStore } from '../../../stores/auth-store'
import type { StackTemplate } from '../api/stack-api'

interface TemplateFormState {
  id: string
  name: string
  description: string
  tools: ToolEntry[]
  estimatedInstallTime: string
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
      { category: 'storage', label: 'Storage Backend', options: ['MinIO', 'AWS S3'] },
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
      { category: 'monitoring', label: 'Monitoring', options: ['Prometheus', 'Thanos', 'Victoria Metrics'] },
      { category: 'visualization', label: 'Visualization', options: ['Grafana'] },
      { category: 'logging', label: 'Logging', options: ['Loki', 'OpenTelemetry', 'Fluentd'] },
      { category: 'log_search', label: 'Log Search', options: ['OpenSearch', 'Elasticsearch'] },
    ],
  },
]

type ToolSection = ToolSectionDefinition
type ToolCategory = ToolCategoryDefinition
type AddToolDraft = { category: string; name: string }

const TOOL_CATEGORY_LOOKUP = new Map<string, ToolCategory>(
  TOOL_SECTIONS.flatMap((section) => section.categories.map((category) => [category.category, category] as const))
)

const TOOL_SECTION_LOOKUP = new Map<string, ToolSection>(
  TOOL_SECTIONS.flatMap((section) => section.categories.map((category) => [category.category, section] as const))
)

const buildInitialSectionOpenState = () =>
  Object.fromEntries(TOOL_SECTIONS.map((section) => [section.id, true])) as Record<string, boolean>

const buildInitialAddToolDrafts = () =>
  Object.fromEntries(
    TOOL_SECTIONS.map((section) => {
      const firstCategory = section.categories[0]
      return [section.id, { category: firstCategory.category, name: firstCategory.options[0] ?? '' }]
    })
  ) as Record<string, AddToolDraft>

const toToolEntry = (toolName: string): ToolEntry => {
  const matched = TOOL_SECTIONS
    .flatMap((section) => section.categories)
    .find((category) => category.options.includes(toolName))

  return {
    category: matched?.category ?? '',
    name: toolName,
    helm_version: '',
    app_version: '',
  }
}

const EMPTY_TEMPLATE_FORM: TemplateFormState = {
  id: '',
  name: '',
  description: '',
  tools: [],
  estimatedInstallTime: String(30 * 60 * 1_000_000_000),
  recommendedUseCase: '',
  minResources: '',
}

export function StackTemplatePage() {
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
  const [addSectionOpen, setAddSectionOpen] = useState(false)
  const [newSectionLabel, setNewSectionLabel] = useState('')
  const [newCategoryLabel, setNewCategoryLabel] = useState('')
  const [newCategoryOptions, setNewCategoryOptions] = useState('')

  const allSections = [...TOOL_SECTIONS, ...customSections]

  const [openToolSections, setOpenToolSections] = useState<Record<string, boolean>>(buildInitialSectionOpenState)
  const [addToolDrafts, setAddToolDrafts] = useState<Record<string, AddToolDraft>>(buildInitialAddToolDrafts)
  const [activeAddToolSection, setActiveAddToolSection] = useState<string | null>(null)

  const templates = Array.isArray(apiTemplates) ? apiTemplates : []

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase()) ||
      t.tools.some((tool) => tool.toLowerCase().includes(search.toLowerCase()))
  )

  const selectedTemplate = selectedTemplateId ? templates.find((template) => template.id === selectedTemplateId) ?? null : null

  const selectedDetail = selectedTemplate
    ? {
      fullDescription: selectedTemplate.description,
      resource: selectedTemplate.minResources ?? 'N/A',
      compatibility: selectedTemplate.recommendedUseCase ?? 'Compatibility details are managed in Stack Version.',
    }
    : null

  const handleUseTemplate = (templateId: string) => {
    setTemplate(templateId)
    loadFromTemplate(templateId)
    navigate(`/stack/install?template=${templateId}`)
  }

  const resetForm = () => {
    setForm(EMPTY_TEMPLATE_FORM)
    setFormError(null)
    setEditingTemplateId(null)
    setOpenToolSections(buildInitialSectionOpenState)
    setAddToolDrafts(buildInitialAddToolDrafts)
    setActiveAddToolSection(null)
  }

  const openCreateModal = () => {
    resetForm()
    setFormOpen(true)
  }

  const openEditModal = (template: StackTemplate) => {
    setEditingTemplateId(template.id)
    setFormError(null)
    setForm({
      id: template.id,
      name: template.name,
      description: template.description,
      tools: template.tools.map(toToolEntry),
      estimatedInstallTime: String(Math.max(1, Math.round(template.estimatedMinutes * 60 * 1_000_000_000))),
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

  const addTool = (category: string, name: string) => {
    setForm((prev) => ({
      ...prev,
      tools: [...prev.tools, { category, name, helm_version: '', app_version: '' }],
    }))
  }

  const removeTool = (index: number) => {
    setForm((prev) => ({
      ...prev,
      tools: prev.tools.filter((_, i) => i !== index),
    }))
  }

  const updateTool = (index: number, field: keyof ToolEntry, value: string) => {
    setForm((prev) => ({
      ...prev,
      tools: prev.tools.map((tool, i) => (i === index ? { ...tool, [field]: value } : tool)),
    }))
  }

  const updateAddToolCategory = (sectionId: string, category: string) => {
    const categoryMeta = TOOL_CATEGORY_LOOKUP.get(category)
    setAddToolDrafts((prev) => ({
      ...prev,
      [sectionId]: {
        category,
        name: categoryMeta?.options[0] ?? '',
      },
    }))
  }

  const updateAddToolName = (sectionId: string, name: string) => {
    setAddToolDrafts((prev) => ({
      ...prev,
      [sectionId]: {
        ...prev[sectionId],
        name,
      },
    }))
  }

  const submitAddTool = (sectionId: string) => {
    const draft = addToolDrafts[sectionId]
    if (!draft?.category || !draft.name) {
      return
    }

    addTool(draft.category, draft.name)
    setActiveAddToolSection(null)
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
    setAddToolDrafts((prev) => ({ ...prev, [sectionId]: { category: categoryId, name: options[0] ?? 'Custom Tool' } }))
    setNewSectionLabel('')
    setNewCategoryLabel('')
    setNewCategoryOptions('')
    setAddSectionOpen(false)
  }

  const removeSection = (sectionId: string) => {
    const section = allSections.find((s) => s.id === sectionId)
    if (!section) return
    const categoryIds = new Set(section.categories.map((c) => c.category))
    setForm((prev) => ({ ...prev, tools: prev.tools.filter((t) => !categoryIds.has(t.category)) }))
    setCustomSections((prev) => prev.filter((s) => s.id !== sectionId))
    setOpenToolSections((prev) => { const next = { ...prev }; delete next[sectionId]; return next })
    setAddToolDrafts((prev) => { const next = { ...prev }; delete next[sectionId]; return next })
  }



  const submitTemplate = () => {
    const estimatedInstallTime = Number(form.estimatedInstallTime)
    if (!Number.isFinite(estimatedInstallTime) || estimatedInstallTime < 0) {
      setFormError('Estimated install time must be a non-negative number.')
      return
    }

    const payload = {
      id: form.id,
      name: form.name,
      description: form.description,
      tools: form.tools,
      estimated_install_time: estimatedInstallTime,
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
            setFormError('Failed to update template.')
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
        setFormError('Failed to create template.')
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
          { label: 'Stack List', path: '/stack/list' },
          { label: 'Stack Template' },
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
              Stack Template
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              검증된 DevSecOps 스택 템플릿을 선택하여 빠르게 시작하세요.
            </p>
          </div>
        </div>
        {isAdmin && (
          <Button variant="primary" size="md" type="button" onClick={openCreateModal}>
            <Plus size={15} />
            Create Template
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
            placeholder="템플릿 검색..."
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
                {template.description}
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
                  <span>약 {template.estimatedMinutes}분</span>
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
                      Edit
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
                      Delete
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
                    handleUseTemplate(template.id)
                  }}
                >
                  Use Base Template
                </Button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          검색 결과가 없습니다.
        </div>
      )}

      <Modal
        open={selectedTemplate !== null}
        onClose={() => setSelectedTemplateId(null)}
        title={selectedTemplate?.name ?? 'Template Detail'}
        wide
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setSelectedTemplateId(null)} type="button">
              Close
            </Button>
            {selectedTemplate && (
              <Button
                variant="primary"
                size="sm"
                type="button"
                onClick={() => {
                  setSelectedTemplateId(null)
                  handleUseTemplate(selectedTemplate.id)
                }}
              >
                Base Template
              </Button>
            )}
          </>
        }
      >
        {selectedTemplate && selectedDetail && (
          <div className="flex flex-col gap-4">
            <div>
              <div className="mb-1.5 text-[13px] text-[var(--color-text-secondary)]">Description</div>
              <p className="m-0 text-sm leading-[1.7] text-[var(--color-text-primary)]">
                {selectedDetail.fullDescription}
              </p>
            </div>

            <div>
              <div className="mb-2 text-[13px] text-[var(--color-text-secondary)]">Included Tools</div>
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
                <div className="mb-1.5 text-xs text-[var(--color-text-secondary)]">Estimated Deploy Time</div>
                <div className="text-base font-bold text-[#fcd34d]">{selectedTemplate.estimatedMinutes} minutes</div>
              </div>
              <div className="rounded-lg border border-[var(--color-border-default)] p-3">
                <div className="mb-1.5 text-xs text-[var(--color-text-secondary)]">Resource Requirements</div>
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">{selectedDetail.resource}</div>
              </div>
            </div>

            <div className="rounded-lg border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.08)] p-3">
              <div className="mb-1.5 text-xs font-bold text-[#86efac]">Compatibility</div>
              <div className="text-[13px] text-[var(--color-text-primary)]">{selectedDetail.compatibility}</div>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={formOpen}
        onClose={closeFormModal}
        title={editingTemplateId ? 'Edit Template' : 'Create Template'}
        footer={
          <>
            <Button variant="outline" size="sm" onClick={closeFormModal} type="button">
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={submitTemplate}
              loading={createTemplate.isPending || updateTemplate.isPending}
            >
              {editingTemplateId ? 'Save' : 'Create'}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <Input
            label="Template ID"
            value={editingTemplateId ?? form.id}
            onChange={(event) => handleFormChange('id', event.target.value)}
            disabled={editingTemplateId !== null}
          />
          <Input
            label="Name"
            value={form.name}
            onChange={(event) => handleFormChange('name', event.target.value)}
          />
          <Input
            label="Description"
            value={form.description}
            onChange={(event) => handleFormChange('description', event.target.value)}
          />
          <div className="flex flex-col gap-2">
            <div className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              Tools
            </div>
            <div className="rounded-lg border border-[var(--color-border-default)]">
              {allSections.map((section) => {
                const isOpen = openToolSections[section.id]
                const sectionCategoryIds = new Set(section.categories.map((c) => c.category))
                const sectionTools = form.tools
                  .map((tool, index) => ({ tool, index }))
                  .filter(({ tool }) => sectionCategoryIds.has(tool.category) || TOOL_SECTION_LOOKUP.get(tool.category)?.id === section.id)
                const addDraft = addToolDrafts[section.id]
                const addCategoryMeta = TOOL_CATEGORY_LOOKUP.get(addDraft?.category ?? '')

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
                        title={`Remove ${section.label} section`}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>

                    {isOpen && (
                      <div className="flex flex-col gap-2 px-3 py-3">
                        {sectionTools.length === 0 && (
                          <div className="rounded-md border border-dashed border-[var(--color-border-default)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                            No tools added in this section.
                          </div>
                        )}

                        {sectionTools.map(({ tool, index }) => {
                          const categoryMeta = TOOL_CATEGORY_LOOKUP.get(tool.category)
                          return (
                            <div
                              key={`${tool.category}-${index}`}
                              className="flex flex-col gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-2"
                            >
                              <div className="text-xs font-medium text-[var(--color-text-secondary)]">
                                {categoryMeta?.label ?? 'Custom Category'}
                              </div>
                              <div className="grid gap-2 sm:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1fr)_auto]">
                                <select
                                  value={tool.name}
                                  onChange={(event) => updateTool(index, 'name', event.target.value)}
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                                >
                                  {(categoryMeta?.options.length ? categoryMeta.options : [tool.name]).map((option) => (
                                    <option key={option} value={option}>
                                      {option}
                                    </option>
                                  ))}
                                </select>
                                <input
                                  type="text"
                                  value={tool.helm_version}
                                  onChange={(event) => updateTool(index, 'helm_version', event.target.value)}
                                  placeholder="Helm version"
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                                />
                                <input
                                  type="text"
                                  value={tool.app_version}
                                  onChange={(event) => updateTool(index, 'app_version', event.target.value)}
                                  placeholder="App version"
                                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                                />
                                <button
                                  type="button"
                                  aria-label={`Remove ${tool.name}`}
                                  onClick={() => removeTool(index)}
                                  className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-lg border border-[var(--color-border-default)] text-[var(--color-text-secondary)] transition-colors hover:border-[rgba(248,113,113,0.5)] hover:text-[#f87171]"
                                >
                                  <X size={15} />
                                </button>
                              </div>
                            </div>
                          )
                        })}

                        {activeAddToolSection === section.id ? (
                          <div className="grid gap-2 rounded-lg border border-dashed border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto]">
                            <select
                              value={addDraft?.category ?? section.categories[0].category}
                              onChange={(event) => updateAddToolCategory(section.id, event.target.value)}
                              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                            >
                              {section.categories.map((category) => (
                                <option key={category.category} value={category.category}>
                                  {category.label}
                                </option>
                              ))}
                            </select>
                            <select
                              value={addDraft?.name ?? addCategoryMeta?.options[0] ?? ''}
                              onChange={(event) => updateAddToolName(section.id, event.target.value)}
                              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                            >
                              {(addCategoryMeta?.options ?? []).map((option) => (
                                <option key={option} value={option}>
                                  {option}
                                </option>
                              ))}
                            </select>
                            <Button variant="outline" size="sm" type="button" onClick={() => submitAddTool(section.id)}>
                              Add
                            </Button>
                            <Button variant="ghost" size="sm" type="button" onClick={() => setActiveAddToolSection(null)}>
                              Cancel
                            </Button>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setActiveAddToolSection(section.id)}
                            className="flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-dashed border-[var(--color-border-default)] px-3 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-border-hover)] hover:text-[var(--color-text-primary)]"
                          >
                            <Plus size={14} />
                            Add Tool
                          </button>
                        )}
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
                  placeholder="Section name (e.g. Security)"
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <input
                  type="text"
                  value={newCategoryLabel}
                  onChange={(e) => setNewCategoryLabel(e.target.value)}
                  placeholder="First category (e.g. Scanner)"
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <input
                  type="text"
                  value={newCategoryOptions}
                  onChange={(e) => setNewCategoryOptions(e.target.value)}
                  placeholder="Tool options (comma separated, e.g. Trivy, SonarQube)"
                  className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
                />
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={addSection}>Add Section</Button>
                  <Button variant="ghost" size="sm" type="button" onClick={() => setAddSectionOpen(false)}>Cancel</Button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setAddSectionOpen(true)}
                className="mt-2 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-dashed border-[var(--color-border-default)] px-3 py-2.5 text-sm text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-border-hover)] hover:text-[var(--color-text-primary)]"
              >
                <Plus size={14} />
                Add Section
              </button>
            )}
          </div>
          <Input
            label="Estimated Install Time (ns)"
            value={form.estimatedInstallTime}
            onChange={(event) => handleFormChange('estimatedInstallTime', event.target.value)}
            type="number"
          />
          <Input
            label="Recommended Use Case"
            value={form.recommendedUseCase}
            onChange={(event) => handleFormChange('recommendedUseCase', event.target.value)}
          />
          <Input
            label="Minimum Resources"
            value={form.minResources}
            onChange={(event) => handleFormChange('minResources', event.target.value)}
          />
          {formError && <div className="text-xs text-[#f87171]">{formError}</div>}
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteTemplateId !== null}
        onClose={() => setDeleteTemplateId(null)}
        onConfirm={handleDeleteTemplate}
        title="Delete Template"
        description="템플릿을 삭제하면 더 이상 목록에 표시되지 않습니다. 계속하시겠습니까?"
        confirmLabel="Delete Template"
        loading={deleteTemplate.isPending}
      />
    </div>
  )
}
