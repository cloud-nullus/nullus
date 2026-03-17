import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Pencil, Plus, Search, Trash2, User } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import {
  useCicdTemplates,
  useCreateCicdTemplate,
  useUpdateCicdTemplate,
  useDeleteCicdTemplate,
} from '../api/cicd-api'
import type { CicdTemplate } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { useAuthStore } from '../../../stores/auth-store'

const APP_TYPE_COLOR: Record<string, { bg: string; color: string }> = {
  'web-backend': { bg: 'rgba(99,102,241,0.12)', color: '#a5b4fc' },
  'web-frontend': { bg: 'rgba(16,185,129,0.12)', color: '#34d399' },
  'batch-job': { bg: 'rgba(245,158,11,0.12)', color: '#fbbf24' },
}

const STAGE_OPTIONS = ['Production', 'QA', 'Development', 'Beta'] as const

interface TemplateFormState {
  id: string
  name: string
  description: string
  stages: string[]
}

const EMPTY_FORM: TemplateFormState = {
  id: '',
  name: '',
  description: '',
  stages: [],
}

const MOCK_CICD_TEMPLATES: CicdTemplate[] = [
  { id: 'web-frontend', name: 'Web Frontend', description: 'React/Next.js 웹 프론트엔드 앱을 위한 표준 CI/CD 파이프라인. Docker 빌드 후 ArgoCD로 배포.', appType: 'web-frontend', stages: ['Build', 'Test', 'Docker Build', 'ArgoCD Deploy'], createdBy: 'admin' },
  { id: 'web-backend', name: 'Backend API', description: 'REST API 백엔드 서비스를 위한 파이프라인. Security Scan(Trivy) 포함, Kubernetes Deployment 배포.', appType: 'web-backend', stages: ['Build', 'Test', 'Security', 'Docker Build', 'ArgoCD Deploy'], createdBy: 'admin' },
  { id: 'batch-job', name: 'Batch Job', description: '정기 실행 배치 잡을 위한 파이프라인. Kubernetes CronJob으로 배포, 실행 결과 자동 기록.', appType: 'batch-job', stages: ['Build', 'Test', 'Docker Build', 'CronJob Deploy'], createdBy: 'admin' },
]

export function CicdTemplatePage() {
  const navigate = useNavigate()
  const role = useAuthStore((state) => state.role)
  const isAdmin = role === 'admin'

  const { data: apiTemplates } = useCicdTemplates()
  const createTemplate = useCreateCicdTemplate()
  const updateTemplate = useUpdateCicdTemplate()
  const deleteTemplate = useDeleteCicdTemplate()
  const templates = Array.isArray(apiTemplates) && apiTemplates.length > 0 ? apiTemplates : MOCK_CICD_TEMPLATES

  const [search, setSearch] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [deleteTemplateId, setDeleteTemplateId] = useState<string | null>(null)
  const [form, setForm] = useState<TemplateFormState>(EMPTY_FORM)
  const [formError, setFormError] = useState<string | null>(null)

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase())
  )

  const resetForm = () => {
    setForm(EMPTY_FORM)
    setFormError(null)
    setEditingTemplateId(null)
  }

  const openCreateModal = () => {
    resetForm()
    setFormOpen(true)
  }

  const closeFormModal = () => {
    setFormOpen(false)
    resetForm()
  }

  const openEditModal = (template: CicdTemplate) => {
    setEditingTemplateId(template.id)
    setFormError(null)
    setForm({
      id: template.id,
      name: template.name,
      description: template.description,
      stages: template.stages.filter((s) => (STAGE_OPTIONS as readonly string[]).includes(s)),
    })
    setFormOpen(true)
  }

  const handleFormChange = (key: Exclude<keyof TemplateFormState, 'stages'>, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const toggleStage = (stage: string) => {
    setForm((prev) => ({
      ...prev,
      stages: prev.stages.includes(stage)
        ? prev.stages.filter((s) => s !== stage)
        : [...prev.stages, stage],
    }))
  }

  const submitTemplate = () => {
    if (!form.name.trim()) {
      setFormError('Name is required.')
      return
    }

    if (form.stages.length === 0) {
      setFormError('At least one stage is required.')
      return
    }

    const templateId = editingTemplateId ?? (form.id.trim() || form.name.toLowerCase().replace(/\s+/g, '-'))

    const payload = {
      id: templateId,
      name: form.name,
      description: form.description,
      appType: 'web-backend' as CicdTemplate['appType'],
      stages: form.stages,
    }

    if (editingTemplateId) {
      updateTemplate.mutate(payload, {
        onSuccess: () => closeFormModal(),
        onError: () => setFormError('Failed to update template.'),
      })
      return
    }

    createTemplate.mutate(payload, {
      onSuccess: () => {
        closeFormModal()
        navigate(`/cicd/create?template=${payload.id}`)
      },
      onError: () => setFormError('Failed to create template.'),
    })
  }

  const handleDeleteTemplate = () => {
    if (!deleteTemplateId) {
      return
    }

    deleteTemplate.mutate(deleteTemplateId, {
      onSuccess: () => setDeleteTemplateId(null),
    })
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: 'CI/CD List', path: '/cicd/list' },
        { label: 'CI/CD Template' },
      ]} />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between gap-4">
        <div className="mb-2 flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <GitBranch size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              CI/CD Template
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              파이프라인 템플릿을 선택하여 빠르게 시작하세요.
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
      <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
        {filtered.map((template) => {
          const typeColor = APP_TYPE_COLOR[template.appType] ?? APP_TYPE_COLOR['web-backend']
          return (
            <div
              key={template.id}
              className="flex h-full flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-150"
            >
              {/* Card header */}
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
                    {template.name}
                  </h3>
                  <span
                    className="rounded-[5px] px-2 py-0.5 text-[11px] font-semibold"
                    style={{ backgroundColor: typeColor.bg, color: typeColor.color }}
                  >
                    {template.appType}
                  </span>
                </div>
                <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                  {template.description}
                </p>
              </div>

              {/* Stages */}
              <div className="flex flex-wrap items-center gap-1">
                {template.stages.map((stage, idx) => (
                  <div key={stage} className="flex items-center gap-1">
                    <span
                      className="rounded-md bg-[rgba(99,102,241,0.12)] px-2.5 py-[3px] text-[11px] font-semibold text-[#a5b4fc]"
                    >
                      {stage}
                    </span>
                    {idx < template.stages.length - 1 && (
                      <span className="text-[11px] text-[var(--color-text-muted)]">→</span>
                    )}
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div className="mt-auto flex items-center justify-between border-t border-[var(--color-border-default)] pt-2.5">
                <div className="flex items-center gap-[5px] text-xs text-[var(--color-text-muted)]">
                  {template.createdBy && <User size={12} />}
                  <span>{template.createdBy ?? ''}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  {isAdmin && (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        onClick={() => openEditModal(template)}
                      >
                        <Pencil size={13} />
                        Edit
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        type="button"
                        onClick={() => setDeleteTemplateId(template.id)}
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
                    onClick={() => navigate(`/cicd/create?template=${template.id}`)}
                  >
                    Use Base Template
                  </Button>
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          검색 결과가 없습니다.
        </div>
      )}

      {/* Create Template Modal */}
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
            placeholder="예: web-backend-standard"
            value={editingTemplateId ?? form.id}
            onChange={(e) => handleFormChange('id', e.target.value)}
            disabled={editingTemplateId !== null}
          />
          <Input
            label="Name"
            placeholder="예: Standard Web Backend"
            value={form.name}
            onChange={(e) => handleFormChange('name', e.target.value)}
          />
          <Input
            label="Description"
            value={form.description}
            onChange={(e) => handleFormChange('description', e.target.value)}
          />
          <div className="flex flex-col gap-2">
            <span className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              Stages
            </span>
            <div className="flex flex-wrap gap-2">
              {STAGE_OPTIONS.map((stage) => {
                const checked = form.stages.includes(stage)
                return (
                  <label
                    key={stage}
                    className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors duration-150 ${
                      checked
                        ? 'border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.1)] text-[#a5b4fc]'
                        : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-secondary)]'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleStage(stage)}
                      className="accent-[#6366f1]"
                    />
                    {stage}
                  </label>
                )
              })}
            </div>
          </div>
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
