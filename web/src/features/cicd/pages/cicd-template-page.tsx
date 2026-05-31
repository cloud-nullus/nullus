import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { BookOpen, Pencil, Plus, Search, Trash2, User } from 'lucide-react'
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
import { resolveLocale } from '../../../lib/locale'

const CAPABILITY_OPTIONS = ['CI', 'CD', 'Test', 'Security'] as const
const PRIORITY_TEMPLATE_IDS = ['nullus-sample-backend-v1', 'nullus-sample-frontend-v1'] as const
type CicdType = 'default' | 'helm' | 'cronjobJob'

const CICD_TYPES: Array<{ id: CicdType; label: string; description: string }> = [
  { id: 'default', label: 'Default', description: 'Deployment, Service, Ingress' },
  { id: 'helm', label: 'Helm', description: 'Helm chart release' },
  { id: 'cronjobJob', label: 'Cronjob/Job', description: 'Scheduled and one-time workloads' },
]

const TEMPLATE_DESCRIPTION_I18N: Record<string, { ko: string; en: string }> = {
  'web-frontend': {
    ko: 'React/Next.js 웹 프론트엔드 앱 템플릿',
    en: 'React/Next.js web frontend app template',
  },
  'web-backend': {
    ko: 'REST API 백엔드 서비스 템플릿',
    en: 'REST API backend service template',
  },
  'batch-job': {
    ko: '배치 잡 템플릿',
    en: 'Batch job template',
  },
  'web-frontend-standard': {
    ko: 'React/Next.js 웹 프론트엔드 앱 템플릿',
    en: 'React/Next.js web frontend app template',
  },
  'web-backend-standard': {
    ko: 'REST API 백엔드 서비스 템플릿',
    en: 'REST API backend service template',
  },
  'batch-job-standard': {
    ko: '배치 잡 템플릿',
    en: 'Batch job template',
  },
  'web-backend-v1': {
    ko: '백엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 이미지 빌드, 배포 단계를 포함합니다.',
    en: 'CI/CD pipeline for backend services. Includes build, test, image build, and deploy stages.',
  },
  'web-frontend-v1': {
    ko: '프론트엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 정적 빌드, 배포 단계를 포함합니다.',
    en: 'CI/CD pipeline for frontend services. Includes build, test, static build, and deploy stages.',
  },
  'batch-job-v1': {
    ko: '배치 작업을 위한 CI/CD 파이프라인. 빌드, 이미지 빌드, CronJob 배포 단계를 포함합니다.',
    en: 'CI/CD pipeline for batch workloads. Includes build, image build, and CronJob deploy stages.',
  },
  'nullus-sample-backend-v1': {
    ko: 'Nullus 플랫폼 데모용 Go API 서버입니다. backend/Dockerfile로 빌드하고 Kubernetes에 배포합니다.',
    en: 'Go API server for the Nullus platform demo. Builds from backend/Dockerfile and deploys to Kubernetes.',
  },
  'nullus-sample-frontend-v1': {
    ko: 'Nullus 플랫폼 데모용 React SPA입니다. frontend/Dockerfile(Nginx)로 빌드하고 Kubernetes에 배포합니다.',
    en: 'React SPA for the Nullus platform demo. Builds from frontend/Dockerfile (Nginx) and deploys to Kubernetes.',
  },
}

const TEMPLATE_DESCRIPTION_KO_TO_EN: Record<string, string> = {
  'React/Next.js 웹 프론트엔드 앱 템플릿': 'React/Next.js web frontend app template',
  'REST API 백엔드 서비스 템플릿': 'REST API backend service template',
  '배치 잡 템플릿': 'Batch job template',
  '백엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 이미지 빌드, 배포 단계를 포함합니다.':
    'CI/CD pipeline for backend services. Includes build, test, image build, and deploy stages.',
  '프론트엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 정적 빌드, 배포 단계를 포함합니다.':
    'CI/CD pipeline for frontend services. Includes build, test, static build, and deploy stages.',
  '배치 작업을 위한 CI/CD 파이프라인. 빌드, 이미지 빌드, CronJob 배포 단계를 포함합니다.':
    'CI/CD pipeline for batch workloads. Includes build, image build, and CronJob deploy stages.',
  'Nullus 플랫폼 데모용 Go API 서버입니다. backend/Dockerfile로 빌드하고 Kubernetes에 배포합니다.':
    'Go API server for the Nullus platform demo. Builds from backend/Dockerfile and deploys to Kubernetes.',
  'Nullus 플랫폼 데모용 React SPA입니다. frontend/Dockerfile(Nginx)로 빌드하고 Kubernetes에 배포합니다.':
    'React SPA for the Nullus platform demo. Builds from frontend/Dockerfile (Nginx) and deploys to Kubernetes.',
}

const TEMPLATE_DESCRIPTION_EN_TO_KO = Object.fromEntries(
  Object.entries(TEMPLATE_DESCRIPTION_KO_TO_EN).map(([ko, en]) => [en, ko])
) as Record<string, string>

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

function resolveCicdType(template: CicdTemplate): CicdType {
  const searchable = `${template.id} ${template.name} ${template.stages.join(' ')}`.toLowerCase()
  if (searchable.includes('helm')) return 'helm'
  if (searchable.includes('cronjob') || searchable.includes('cron job') || searchable.includes('job')) return 'cronjobJob'
  return 'default'
}

function resolveCapabilities(stages: string[]): string[] {
  const hasCapability = (capability: (typeof CAPABILITY_OPTIONS)[number]) => stages.some((stage) => {
    const normalized = stage.toLowerCase().replace(/[\s_-]/g, '')
    if (capability === 'CI') {
      return normalized === 'ci'
        || normalized.includes('build')
        || normalized.includes('gitclone')
        || normalized.includes('imageload')
        || normalized.includes('package')
        || normalized.includes('lint')
    }
    if (capability === 'CD') {
      return normalized === 'cd'
        || normalized.includes('deploy')
        || normalized.includes('release')
        || normalized.includes('rollout')
        || normalized.includes('sync')
        || normalized.includes('apply')
    }
    if (capability === 'Test') return normalized.includes('test')
    return normalized.includes('security') || normalized.includes('scan')
  })

  return CAPABILITY_OPTIONS.filter(hasCapability)
}

export function CicdTemplatePage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const role = useAuthStore((state) => state.role)
  const isAdmin = role === 'admin'

  const { data: apiTemplates } = useCicdTemplates()
  const createTemplate = useCreateCicdTemplate()
  const updateTemplate = useUpdateCicdTemplate()
  const deleteTemplate = useDeleteCicdTemplate()
  const templates = (Array.isArray(apiTemplates) ? apiTemplates : [])
    .filter((template) => template.id !== 'web-frontend-v1')
    .map((template) => template.id === 'web-backend-v1' ? { ...template, name: 'User Custom Pipeline' } : template)

  const [search, setSearch] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [deleteTemplateId, setDeleteTemplateId] = useState<string | null>(null)
  const [form, setForm] = useState<TemplateFormState>(EMPTY_FORM)
  const [formError, setFormError] = useState<string | null>(null)
  const isKorean = resolveLocale(i18n.resolvedLanguage || i18n.language) === 'ko-KR'

  const resolveTemplateDescription = (template: CicdTemplate) => {
    const localized = TEMPLATE_DESCRIPTION_I18N[template.id]
    if (localized) {
      return isKorean ? localized.ko : localized.en
    }

    if (!isKorean) {
      const enFallback = TEMPLATE_DESCRIPTION_KO_TO_EN[template.description]
      if (enFallback) return enFallback
    } else {
      const koFallback = TEMPLATE_DESCRIPTION_EN_TO_KO[template.description]
      if (koFallback) return koFallback
    }

    return template.description
  }

  const filtered = templates.filter(
    (template) =>
      template.name.toLowerCase().includes(search.toLowerCase()) ||
      resolveTemplateDescription(template).toLowerCase().includes(search.toLowerCase())
  )
  const prioritizedFiltered = filtered.slice().sort((a, b) => {
    const aPriority = PRIORITY_TEMPLATE_IDS.includes(a.id as (typeof PRIORITY_TEMPLATE_IDS)[number])
    const bPriority = PRIORITY_TEMPLATE_IDS.includes(b.id as (typeof PRIORITY_TEMPLATE_IDS)[number])
    if (aPriority === bPriority) return 0
    return aPriority ? -1 : 1
  })
  const templatesByType = CICD_TYPES.map((type) => ({
    ...type,
    templates: prioritizedFiltered.filter((template) => resolveCicdType(template) === type.id),
  }))

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
      stages: resolveCapabilities(template.stages),
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
      setFormError(t('cicdTemplatePage.errors.nameRequired', 'Name is required.'))
      return
    }

    if (form.stages.length === 0) {
      setFormError(t('cicdTemplatePage.errors.stageRequired', 'At least one stage is required.'))
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
        onError: () => setFormError(t('cicdTemplatePage.errors.updateFailed', 'Failed to update template.')),
      })
      return
    }

    createTemplate.mutate(payload, {
      onSuccess: () => {
        closeFormModal()
        navigate('/cicd/developer-deploy')
      },
      onError: () => setFormError(t('cicdTemplatePage.errors.createFailed', 'Failed to create template.')),
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
        { label: t('cicdTemplatePage.breadcrumb.list', 'CI/CD List'), path: '/cicd/list' },
        { label: t('cicdTemplatePage.breadcrumb.current', 'CI/CD Template') },
      ]} />

      {/* Page header */}
      <div className="mb-7 flex items-start justify-between gap-4">
        <div className="mb-2 flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <BookOpen size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              {t('cicdTemplatePage.title', 'CI/CD Template')}
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              {t('cicdTemplatePage.description', 'Choose a pipeline template to get started quickly.')}
            </p>
          </div>
        </div>
        {isAdmin && (
          <Button variant="primary" size="md" type="button" onClick={openCreateModal}>
            <Plus size={15} />
            {t('cicdTemplatePage.actions.createTemplate', 'Create Template')}
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
            placeholder={t('cicdTemplatePage.searchPlaceholder', 'Search templates...')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
          />
        </div>
      </div>

      {/* Template cards grouped by CI/CD workload type. */}
      {filtered.length > 0 && (
        <div>
          <p className="mb-4 mt-0 text-xs font-semibold uppercase tracking-[0.08em] text-[var(--color-text-secondary)]">
            {t('cicdTemplatePage.cicdType', 'CI/CD Type')}
          </p>
          <div className="flex flex-col gap-8">
            {templatesByType.map((type) => (
              <section key={type.id} aria-label={type.label}>
                <div className="mb-3 flex items-baseline gap-3">
                  <h2 className="m-0 text-lg font-bold text-[var(--color-text-primary)]">
                    {t(`cicdTemplatePage.types.${type.id}.label`, type.label)}
                  </h2>
                  <span className="text-sm text-[var(--color-text-secondary)]">
                    {t(`cicdTemplatePage.types.${type.id}.description`, type.description)}
                  </span>
                </div>
                {type.templates.length === 0 ? (
                  <div className="rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] px-4 py-5 text-sm text-[var(--color-text-muted)]">
                    {t('cicdTemplatePage.typeEmpty', 'No templates in this type.')}
                  </div>
                ) : (
                  <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
                    {type.templates.map((template) => {
                      const capabilities = resolveCapabilities(template.stages)
                      return (
                        <div
                          key={template.id}
                          className="flex h-full flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-150 hover:border-[var(--color-border-hover)]"
                        >
                          <div>
                            <h3 className="m-0 mb-1 text-[15px] font-bold text-[var(--color-text-primary)]">
                              {template.name}
                            </h3>
                            <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                              {resolveTemplateDescription(template)}
                            </p>
                          </div>

                          <div className="grid grid-cols-2 gap-3 rounded-lg bg-[rgba(255,255,255,0.02)] p-3">
                            <div>
                              <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                                {t('cicdTemplatePage.card.workloadType', 'CI/CD Type')}
                              </span>
                              <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                                {t(`cicdTemplatePage.types.${type.id}.label`, type.label)}
                              </p>
                            </div>
                            <div>
                              <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                                {t('cicdTemplatePage.card.appType', 'Application Type')}
                              </span>
                              <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                                {template.appType}
                              </p>
                            </div>
                          </div>

                          <div>
                            <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                              {t('cicdTemplatePage.card.capabilities', 'Capabilities')}
                            </span>
                            <div className="mt-2 flex flex-wrap gap-1.5">
                              {capabilities.map((capability) => (
                                <span key={capability} className="rounded-md bg-[rgba(99,102,241,0.12)] px-2 py-1 text-[11px] font-semibold text-[#a5b4fc]">
                                  {capability}
                                </span>
                              ))}
                              {capabilities.length === 0 && (
                                <span className="text-xs text-[var(--color-text-muted)]">
                                  {t('cicdTemplatePage.card.noCapabilities', 'No capabilities selected')}
                                </span>
                              )}
                            </div>
                          </div>

                          <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-[var(--color-border-default)] pt-2.5">
                            <div className="flex items-center gap-[5px] text-xs text-[var(--color-text-muted)]">
                              {template.createdBy && <User size={12} />}
                              <span>{template.createdBy ?? ''}</span>
                            </div>
                            <div className="ml-auto flex flex-wrap items-center justify-end gap-1.5">
                              {isAdmin && (
                                <>
                                  <Button variant="ghost" size="sm" type="button" onClick={() => openEditModal(template)}>
                                    <Pencil size={13} />
                                    {t('cicdTemplatePage.actions.edit', 'Edit')}
                                  </Button>
                                  <Button variant="danger" size="sm" type="button" onClick={() => setDeleteTemplateId(template.id)}>
                                    <Trash2 size={13} />
                                    {t('cicdTemplatePage.actions.delete', 'Delete')}
                                  </Button>
                                </>
                              )}
                              <Button
                                variant="primary"
                                size="sm"
                                type="button"
                                className="w-auto max-w-full bg-[linear-gradient(135deg,#facc15,#eab308)] text-[#111827]"
                                onClick={() => navigate(`/cicd/developer-deploy?template=${encodeURIComponent(template.id)}&appType=${encodeURIComponent(template.appType)}`)}
                              >
                                {t('cicdTemplatePage.actions.useBaseTemplate', 'Use Base Template')}
                              </Button>
                            </div>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                )}
              </section>
            ))}
          </div>
        </div>
      )}

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t('cicdTemplatePage.empty', 'No search results found.')}
        </div>
      )}

      {/* Create Template Modal */}
      <Modal
        open={formOpen}
        onClose={closeFormModal}
        title={editingTemplateId ? t('cicdTemplatePage.modal.editTitle', 'Edit Template') : t('cicdTemplatePage.modal.createTitle', 'Create Template')}
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
              {editingTemplateId ? t('common.save', 'Save') : t('cicdTemplatePage.actions.create', 'Create')}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <Input
            label={t('cicdTemplatePage.form.templateId', 'Template ID')}
            placeholder={t('cicdTemplatePage.form.templateIdPlaceholder', 'e.g. web-backend-standard')}
            value={editingTemplateId ?? form.id}
            onChange={(e) => handleFormChange('id', e.target.value)}
            disabled={editingTemplateId !== null}
          />
          <Input
            label={t('cicdTemplatePage.form.name', 'Name')}
            placeholder={t('cicdTemplatePage.form.namePlaceholder', 'e.g. Standard Web Backend')}
            value={form.name}
            onChange={(e) => handleFormChange('name', e.target.value)}
          />
          <Input
            label={t('cicdTemplatePage.form.description', 'Description')}
            value={form.description}
            onChange={(e) => handleFormChange('description', e.target.value)}
          />
          <div className="flex flex-col gap-2">
            <span className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              {t('cicdTemplatePage.form.stages', 'Capabilities')}
            </span>
            <div className="flex flex-wrap gap-2">
              {CAPABILITY_OPTIONS.map((stage) => {
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
        title={t('cicdTemplatePage.confirm.deleteTitle', 'Delete Template')}
        description={t('cicdTemplatePage.confirm.deleteDescription', 'This template will no longer be shown in the list. Continue?')}
        confirmLabel={t('cicdTemplatePage.confirm.deleteConfirmLabel', 'Delete Template')}
        loading={deleteTemplate.isPending}
      />
    </div>
  )
}
