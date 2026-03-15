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

const MOCK_TEMPLATES = [
  {
    id: 'gitlab-all-in-one',
    name: 'GitLab All-in-One',
    description: 'GitLab을 중심으로 소스, 컨테이너 레지스트리, CI/CD, 모니터링을 통합하는 올인원 스택.',
    tools: ['GitLab', 'GitLab CI', 'GitLab Registry', 'Prometheus', 'Grafana', 'OpenSearch'],
    estimatedMinutes: 25,
    category: 'gitlab',
    createdBy: 'admin',
  },
  {
    id: 'gitlab-argocd',
    name: 'GitLab + ArgoCD',
    description: 'GitLab으로 소스/CI를 관리하고 ArgoCD로 GitOps 기반 CD를 구현하는 하이브리드 스택.',
    tools: ['GitLab', 'GitLab CI', 'ArgoCD', 'Prometheus', 'Grafana', 'OpenTelemetry'],
    estimatedMinutes: 30,
    category: 'hybrid',
    createdBy: 'admin',
  },
  {
    id: 'github-argocd',
    name: 'GitHub + ArgoCD',
    description: 'GitHub Actions로 CI를 처리하고 ArgoCD로 쿠버네티스 배포를 자동화하는 클라우드 네이티브 스택.',
    tools: ['GitHub', 'GitHub Actions', 'ArgoCD', 'Prometheus', 'Grafana', 'OpenTelemetry', 'OpenSearch'],
    estimatedMinutes: 20,
    category: 'github',
    createdBy: 'admin',
  },
]

const TEMPLATE_DETAILS: Record<string, { fullDescription: string; resource: string; compatibility: string }> = {
  'gitlab-all-in-one': {
    fullDescription:
      'GitLab 중심의 통합형 템플릿으로 소스 관리, 패키지/이미지 저장소, CI/CD, 관측성을 하나의 운영 흐름으로 묶습니다. 플랫폼 팀이 빠르게 표준 환경을 구축할 때 적합합니다.',
    resource: '4 vCPU / 8Gi Memory / 80Gi Storage',
    compatibility: 'Kubernetes 1.27-1.30, Helm 3.14+, Containerd 1.7+',
  },
  'gitlab-argocd': {
    fullDescription:
      'GitLab의 소스/CI 기능과 ArgoCD GitOps 배포를 결합한 하이브리드 템플릿입니다. 변경 이력 추적성과 배포 안정성을 동시에 확보할 수 있습니다.',
    resource: '6 vCPU / 12Gi Memory / 100Gi Storage',
    compatibility: 'Kubernetes 1.26-1.30, Helm 3.13+, Ingress Controller 필요',
  },
  'github-argocd': {
    fullDescription:
      'GitHub Actions 기반 CI와 ArgoCD 기반 CD를 조합한 클라우드 네이티브 템플릿입니다. SaaS 중심 팀에서 최소 운영 부담으로 시작하기 좋습니다.',
    resource: '4 vCPU / 8Gi Memory / 60Gi Storage',
    compatibility: 'Kubernetes 1.27-1.31, Helm 3.14+, OIDC 연동 권장',
  },
}

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

  const templates = Array.isArray(apiTemplates) ? apiTemplates : MOCK_TEMPLATES

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase()) ||
      t.tools.some((tool) => tool.toLowerCase().includes(search.toLowerCase()))
  )

  const selectedTemplate = selectedTemplateId ? templates.find((template) => template.id === selectedTemplateId) ?? null : null

  const selectedDetail = selectedTemplate
    ? TEMPLATE_DETAILS[selectedTemplate.id] ?? {
      fullDescription: selectedTemplate.description,
      resource: '4 vCPU / 8Gi Memory / 64Gi Storage',
      compatibility: 'Kubernetes 1.27+ (검증 필요)',
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
      <Breadcrumb items={[{ label: 'Stack Template' }]} />

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
            size={14}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
          <Input
            placeholder="템플릿 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
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
            className="flex cursor-pointer flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] text-left transition-colors duration-150 hover:border-[var(--color-border-hover)]"
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
            <div className="flex items-center justify-between border-t border-[var(--color-border-default)] pt-2.5">
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
