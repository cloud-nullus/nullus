import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, Clock, Pencil, Plus, Search, Trash2, User, Wrench } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useCreateTemplate, useDeleteTemplate, useTemplates, useUpdateTemplate } from '../api/stack-api'
import { useStackConfigStore } from '../stores/stack-config-store'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { useAuthStore } from '../../../stores/auth-store'
import type { StackTemplate } from '../api/stack-api'

const MOCK_TEMPLATES: StackTemplate[] = [
  {
    id: 'gitlab-allinone-v1',
    name: 'GitLab All-in-One',
    description: 'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    tools: ['GitLab CE', 'GitLab CI', 'GitLab Registry', 'MinIO', 'Argo CD', 'Prometheus', 'Grafana'],
    estimatedMinutes: 90,
    category: 'gitlab',
    createdBy: 'admin',
    recommendedUseCase: '중견기업, 단일 플랫폼 선호',
    minResources: '8 vCPU / 16Gi RAM / 100Gi Storage',
  },
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    description: 'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.',
    tools: ['GitLab CE', 'GitLab CI', 'Harbor', 'MinIO', 'Argo CD', 'Prometheus', 'Grafana'],
    estimatedMinutes: 120,
    category: 'gitlab',
    createdBy: 'admin',
    recommendedUseCase: 'GitOps 중심 조직',
    minResources: '10 vCPU / 20Gi RAM / 130Gi Storage',
  },
  {
    id: 'github-argocd-v1',
    name: 'GitHub + Argo CD',
    description: 'GitHub Actions를 외부 CI로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.',
    tools: ['GitHub', 'GitHub Actions', 'Harbor', 'MinIO', 'Argo CD', 'Prometheus', 'Grafana'],
    estimatedMinutes: 60,
    category: 'github',
    createdBy: 'admin',
    recommendedUseCase: 'GitHub 사용 조직',
    minResources: '6 vCPU / 12Gi RAM / 80Gi Storage',
  },
]

interface TemplateFormState {
  id: string
  name: string
  description: string
  tools: string
  estimatedInstallTime: string
  recommendedUseCase: string
  minResources: string
}

const EMPTY_TEMPLATE_FORM: TemplateFormState = {
  id: '',
  name: '',
  description: '',
  tools: '[]',
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

  const templates = Array.isArray(apiTemplates) && apiTemplates.length > 0 ? apiTemplates : MOCK_TEMPLATES

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
  }

  const openCreateModal = () => {
    resetForm()
    setFormOpen(true)
  }

  const openEditModal = (template: { id: string; name: string; description: string; tools: string[]; estimatedMinutes: number }) => {
    setEditingTemplateId(template.id)
    setFormError(null)
    setForm({
      id: template.id,
      name: template.name,
      description: template.description,
      tools: JSON.stringify(
        template.tools.map((tool) => ({ category: '', name: tool, helm_version: '', app_version: '' })),
        null,
        2
      ),
      estimatedInstallTime: String(Math.max(1, Math.round(template.estimatedMinutes * 60 * 1_000_000_000))),
      recommendedUseCase: '',
      minResources: '',
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

  const submitTemplate = () => {
    let parsedTools: unknown[]
    try {
      const maybeTools = JSON.parse(form.tools)
      if (!Array.isArray(maybeTools)) {
        setFormError('Tools JSON must be an array.')
        return
      }
      parsedTools = maybeTools
    } catch {
      setFormError('Tools JSON is invalid.')
      return
    }

    const estimatedInstallTime = Number(form.estimatedInstallTime)
    if (!Number.isFinite(estimatedInstallTime) || estimatedInstallTime < 0) {
      setFormError('Estimated install time must be a non-negative number.')
      return
    }

    const payload = {
      id: form.id,
      name: form.name,
      description: form.description,
      tools: parsedTools,
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
          <button
            key={template.id}
            type="button"
            onClick={() => setSelectedTemplateId(template.id)}
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
          </button>
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
          <div className="flex flex-col gap-1">
            <label htmlFor="template-tools-json" className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              Tools (JSON)
            </label>
            <textarea
              id="template-tools-json"
              value={form.tools}
              onChange={(event) => handleFormChange('tools', event.target.value)}
              className="min-h-28 w-full rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] outline-none"
            />
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
