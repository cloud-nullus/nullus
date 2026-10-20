import { useEffect, useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'
import { toast } from 'sonner'
import { PageHeader } from '../../../components/layout/page-header'
import { Button } from '../../../components/ui/button'
import { iconProps } from '../../../components/ui/icon'
import { textInputClass } from '../../../components/ui/text-input'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { cn } from '../../../lib/utils'
import {
  useAssistantSettings,
  useDeleteAssistantSettings,
  useSaveAssistantSettings,
  useTestAssistantSettings,
  type AssistantProvider,
  type AssistantProviderId,
  type AssistantSettings,
  type AssistantSettingsInput,
  type AssistantTestResult,
} from '../api/assistant-api'

// AI 어시스턴트 설정(nullus-plan#94). 조직마다 모델 공급자(벤더) · 모델 · API 키를 등록한다. 키는 서버가 암호화해
// 두고 다시 보여 주지 않는다 — 끝 네 자리만. 키 칸을 비워 두면 저장된 키를 그대로 쓴다(같은 공급자일 때).

interface FormState {
  enabled: boolean
  provider: AssistantProviderId
  baseURL: string
  model: string
  apiKey: string
  removeKey: boolean
}

function formFrom(s: AssistantSettings): FormState {
  return { enabled: s.enabled, provider: s.provider, baseURL: s.base_url, model: s.model, apiKey: '', removeKey: false }
}

function toInput(form: FormState): AssistantSettingsInput {
  const input: AssistantSettingsInput = {
    enabled: form.enabled,
    provider: form.provider,
    base_url: form.baseURL.trim(),
    model: form.model.trim(),
  }
  if (form.apiKey.trim() !== '') input.api_key = form.apiKey.trim()
  else if (form.removeKey) input.api_key = ''
  return input
}

function trimSlash(url: string): string {
  return url.replace(/\/+$/, '')
}

function errorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'message' in err && typeof err.message === 'string') return err.message
  return String(err)
}

export function AssistantSettingsPage() {
  const { t } = useTranslation()
  const settings = useAssistantSettings()
  const save = useSaveAssistantSettings()
  const test = useTestAssistantSettings()
  const remove = useDeleteAssistantSettings()
  const [form, setForm] = useState<FormState | null>(null)
  const [error, setError] = useState('')
  const [result, setResult] = useState<AssistantTestResult | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (settings.data) setForm(formFrom(settings.data))
  }, [settings.data])

  if (!settings.data || !form) {
    return (
      <div>
        <PageHeader
          breadcrumb={[{ label: t('assistantSettings.breadcrumb') }]}
          icon={<Sparkles {...iconProps('sm')} />}
          title={t('assistantSettings.title')}
          subtitle={t('assistantSettings.description')}
        />
        <p className="text-[13px] text-[var(--color-text-secondary)]">
          {settings.isError ? t('assistantSettings.loadError') : t('assistantSettings.loading')}
        </p>
      </div>
    )
  }

  const saved = settings.data
  const provider: AssistantProvider | undefined = saved.providers.find((p) => p.id === form.provider)
  // 저장된 키는 같은 곳(공급자 · 실제 주소)으로만 이어 쓴다 — 서버도 같은 규칙이다.
  const effectiveBaseURL = trimSlash(form.baseURL.trim() || provider?.default_base_url || '')
  const sameEndpoint = form.provider === saved.provider && effectiveBaseURL === trimSlash(saved.effective_base_url)
  const keepsSavedKey = sameEndpoint && saved.has_key && !form.removeKey
  const addressChanged = form.provider === saved.provider && saved.has_key && !sameEndpoint
  const update = (patch: Partial<FormState>) => {
    setForm({ ...form, ...patch })
    setResult(null)
    setError('')
  }

  const onSave = () => {
    setError('')
    save.mutate(toInput(form), {
      onSuccess: () => toast.success(t('assistantSettings.saved')),
      onError: (e: unknown) => setError(errorMessage(e)),
    })
  }

  const onTest = () => {
    setError('')
    setResult(null)
    test.mutate(toInput(form), {
      onSuccess: (r) => setResult(r),
      onError: (e: unknown) => setError(errorMessage(e)),
    })
  }

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('assistantSettings.breadcrumb') }]}
        icon={<Sparkles {...iconProps('sm')} />}
        title={t('assistantSettings.title')}
        subtitle={t('assistantSettings.description')}
      />

      <div className="flex max-w-[760px] flex-col gap-5 rounded-lg border border-[var(--color-border-default)] p-5 text-[13px]">
        <div className="text-[var(--color-text-secondary)]">
          {saved.enabled
            ? t('assistantSettings.statusOn', { provider: t(`assistantSettings.providers.${saved.provider}`), model: saved.effective_model })
            : t('assistantSettings.statusOff')}
        </div>

        <fieldset className="m-0 flex flex-col gap-2 border-0 p-0">
          <legend className="mb-2 text-[12px] font-semibold text-[var(--color-text-secondary)]">{t('assistantSettings.provider')}</legend>
          {saved.providers.map((p) => (
            <label
              key={p.id}
              className={cn(
                'flex cursor-pointer items-start gap-2.5 rounded-lg border px-3 py-2',
                form.provider === p.id ? 'border-[var(--color-primary)]' : 'border-[var(--color-border-default)]',
              )}
            >
              <input
                type="radio"
                name="assistant-provider"
                value={p.id}
                checked={form.provider === p.id}
                onChange={() => update({ provider: p.id, apiKey: '', removeKey: false })}
                className="mt-0.5"
              />
              <span>
                <span className="block font-semibold text-[var(--color-text-primary)]">{t(`assistantSettings.providers.${p.id}`)}</span>
                <span className="block text-[12px] text-[var(--color-text-secondary)]">
                  {p.default_model
                    ? t('assistantSettings.defaultModel', { model: p.default_model })
                    : t('assistantSettings.modelRequired')}
                </span>
              </span>
            </label>
          ))}
        </fieldset>

        <Field
          label={t('assistantSettings.model')}
          hint={
            provider?.default_model
              ? t('assistantSettings.modelHintDefault', { model: provider.default_model })
              : t('assistantSettings.modelHintRequired')
          }
        >
          {(id) => (
            <input
              id={id}
              value={form.model}
              onChange={(e) => update({ model: e.target.value })}
              placeholder={provider?.default_model ?? ''}
              className={textInputClass()}
            />
          )}
        </Field>

        <Field
          label={t('assistantSettings.baseURL')}
          hint={provider?.default_base_url ? t('assistantSettings.baseURLHintDefault') : t('assistantSettings.baseURLHintRequired')}
        >
          {(id) => (
            <input
              id={id}
              value={form.baseURL}
              onChange={(e) => update({ baseURL: e.target.value })}
              placeholder={provider?.default_base_url || 'https://llm.internal/v1'}
              className={textInputClass()}
            />
          )}
        </Field>

        <Field
          label={t('assistantSettings.apiKey')}
          hint={
            keepsSavedKey
              ? t('assistantSettings.keySaved', { hint: saved.key_hint })
              : addressChanged
                ? t('assistantSettings.keyAddressChanged')
                : provider && !provider.key_required
                ? t('assistantSettings.keyOptional')
                : t('assistantSettings.keyRequired')
          }
        >
          {(id) => (
            <input
              id={id}
              type="password"
              autoComplete="off"
              value={form.apiKey}
              onChange={(e) => update({ apiKey: e.target.value, removeKey: false })}
              placeholder={keepsSavedKey ? saved.key_hint : ''}
              className={textInputClass()}
            />
          )}
        </Field>
        {saved.key_unreadable && form.provider === saved.provider && (
          <div role="alert" className="-mt-3 text-[12px] text-[var(--color-warning)]">
            {t('assistantSettings.keyUnreadable')}
          </div>
        )}
        {keepsSavedKey && (
          <div className="-mt-3">
            <Button variant="ghost" size="sm" onClick={() => update({ removeKey: true, apiKey: '' })}>
              {t('assistantSettings.removeKey')}
            </Button>
          </div>
        )}

        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.enabled} onChange={(e) => update({ enabled: e.target.checked })} />
          <span className="text-[var(--color-text-primary)]">{t('assistantSettings.enabled')}</span>
        </label>

        <div className="rounded-lg border border-[var(--color-warning)] p-3 text-[12px] text-[var(--color-text-secondary)]">
          {t('assistantSettings.dataNotice')}
        </div>

        {error && (
          <div role="alert" className="text-[var(--color-error)]">
            {error}
          </div>
        )}
        {result && (
          <div role="status" className={result.ok ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'}>
            {result.ok
              ? t('assistantSettings.testOk', { model: result.model, ms: result.latency_ms })
              : t('assistantSettings.testFailed', { error: result.error })}
          </div>
        )}

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="primary" loading={save.isPending} onClick={onSave}>
            {t('assistantSettings.save')}
          </Button>
          <Button variant="outline" loading={test.isPending} onClick={onTest}>
            {t('assistantSettings.test')}
          </Button>
          <span className="flex-1" />
          <Button variant="danger" onClick={() => setConfirmDelete(true)}>
            {t('assistantSettings.delete')}
          </Button>
        </div>
      </div>

      <ConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={() =>
          remove.mutate(undefined, {
            onSuccess: () => {
              setConfirmDelete(false)
              toast.success(t('assistantSettings.deleted'))
            },
            onError: (e: unknown) => setError(errorMessage(e)),
          })
        }
        title={t('assistantSettings.deleteTitle')}
        description={t('assistantSettings.deleteDescription')}
        confirmLabel={t('assistantSettings.deleteConfirm')}
        loading={remove.isPending}
      />
    </div>
  )
}

// Field 는 라벨 · 입력 · 안내 한 줄이다. 라벨은 입력에만 걸어 접근 이름에 안내 문구가 섞이지 않게 한다.
function Field({ label, hint, children }: { label: string; hint: string; children: (id: string) => ReactNode }) {
  const id = useId()
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-[12px] font-semibold text-[var(--color-text-secondary)]">
        {label}
      </label>
      {children(id)}
      <span className="text-[12px] text-[var(--color-text-secondary)]">{hint}</span>
    </div>
  )
}
