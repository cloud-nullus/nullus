import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'
import { RotateCcw, Send, Sparkles, X } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { cn } from '../../../lib/utils'
import {
  useAssistantChat,
  useAssistantStatus,
  type AssistantToolTrace,
  type AssistantTurn,
} from '../api/assistant-api'

// AI 어시스턴트 패널(nullus-plan#94). 서버에 키가 없으면(또는 엔드포인트가 없는 옛 서버면) 아무것도 그리지 않는다.
// 대화는 이 컴포넌트의 상태에만 있다 — 새로 고치면 사라진다(1단계).

interface ChatEntry extends AssistantTurn {
  tools?: AssistantToolTrace[]
}

const SUGGESTIONS = ['assistant.suggestion1', 'assistant.suggestion2', 'assistant.suggestion3'] as const

export function AssistantPanel() {
  const status = useAssistantStatus()
  if (!status.data?.enabled) return null
  return <AssistantLauncher provider={status.data.provider ?? ''} model={status.data.model ?? ''} />
}

// AssistantLauncher 가 대화를 들고 있다 — 패널을 닫았다 열어도 이어지고, 새로 고치면 사라진다.
function AssistantLauncher({ provider, model }: { provider: string; model: string }) {
  const { t } = useTranslation()
  const location = useLocation()
  const chat = useAssistantChat()
  const [open, setOpen] = useState(false)
  const [entries, setEntries] = useState<ChatEntry[]>([])
  const [error, setError] = useState('')
  // 새 대화를 시작하면 이전 대화의 늦은 답을 버리고 요청을 끊는다.
  const conversation = useRef(0)
  const inflight = useRef<AbortController | null>(null)

  const send = (text: string) => {
    const content = text.trim()
    if (!content || chat.isPending) return
    const history: ChatEntry[] = [...entries, { role: 'user', content }]
    const id = conversation.current
    const controller = new AbortController()
    inflight.current = controller
    setEntries(history)
    setError('')
    chat.mutate(
      {
        input: { messages: history.map(({ role, content: c }) => ({ role, content: c })), context: { path: location.pathname } },
        signal: controller.signal,
      },
      {
        onSuccess: (r) => {
          if (conversation.current !== id) return
          setEntries((prev) => [...prev, { role: 'assistant', content: r.reply, tools: r.tools }])
        },
        onError: (e: unknown) => {
          if (conversation.current !== id) return
          setError(errorText(e, t))
        },
      },
    )
  }

  const reset = () => {
    conversation.current += 1
    inflight.current?.abort()
    inflight.current = null
    chat.reset()
    setEntries([])
    setError('')
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-label={t('assistant.open')}
        aria-expanded={open}
        className="fixed bottom-14 right-4 z-40 flex h-10 w-10 items-center justify-center rounded-full bg-[linear-gradient(135deg,var(--color-primary),var(--color-accent-alt))] text-white shadow-lg hover:opacity-90"
      >
        <Sparkles {...iconProps('sm')} />
      </button>
      {open && (
        <AssistantDialog
          provider={provider}
          model={model}
          entries={entries}
          error={error}
          pending={chat.isPending}
          onSend={send}
          onReset={reset}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  )
}

interface AssistantDialogProps {
  provider: string
  model: string
  entries: ChatEntry[]
  error: string
  pending: boolean
  onSend: (text: string) => void
  onReset: () => void
  onClose: () => void
}

function AssistantDialog({ provider, model, entries, error, pending, onSend, onReset, onClose }: AssistantDialogProps) {
  const { t } = useTranslation()
  const [input, setInput] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = listRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [entries, pending, error])

  const submit = (text: string) => {
    if (!text.trim() || pending) return
    onSend(text)
    setInput('')
  }

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    // Enter 로 보내고 Shift+Enter 로 줄을 바꾼다. 한글 조합 중의 Enter 는 조합을 끝내는 것이다.
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault()
      submit(input)
    }
  }

  return (
    <section
      role="dialog"
      aria-label={t('assistant.title')}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose()
      }}
      className="fixed bottom-28 right-4 z-50 flex h-[min(600px,calc(100vh-9rem))] w-[min(420px,calc(100vw-2rem))] flex-col overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] shadow-xl"
    >
      <header className="flex items-center gap-2 border-b border-[var(--color-border-default)] px-4 py-3">
        <Sparkles {...iconProps('sm')} className="text-[var(--color-primary)]" />
        <div className="min-w-0 flex-1">
          <div className="text-[14px] font-bold text-[var(--color-text-primary)]">{t('assistant.title')}</div>
          <div className="truncate text-[11px] text-[var(--color-text-secondary)]">
            {model} · {t('assistant.readOnly')}
          </div>
        </div>
        <button
          type="button"
          onClick={() => {
            onReset()
            setInput('')
          }}
          aria-label={t('assistant.newChat')}
          title={t('assistant.newChat')}
          className="rounded p-1.5 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        >
          <RotateCcw {...iconProps('xs')} />
        </button>
        <button
          type="button"
          onClick={onClose}
          aria-label={t('assistant.close')}
          className="rounded p-1.5 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        >
          <X {...iconProps('xs')} />
        </button>
      </header>

      {/* 답이 오면 스크린 리더가 읽는다. */}
      <div ref={listRef} aria-live="polite" className="flex flex-1 flex-col gap-3 overflow-y-auto px-4 py-3 text-[13px]">
        {entries.length === 0 && (
          <div className="flex flex-col gap-2">
            <p className="m-0 text-[var(--color-text-secondary)]">{t('assistant.empty')}</p>
            {SUGGESTIONS.map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => submit(t(key))}
                className="rounded-lg border border-[var(--color-border-default)] px-3 py-2 text-left text-[var(--color-text-primary)] hover:border-[var(--color-primary)]"
              >
                {t(key)}
              </button>
            ))}
          </div>
        )}
        {entries.map((entry, i) => (
          <ChatBubble key={i} entry={entry} />
        ))}
        {pending && <div className="text-[var(--color-text-secondary)]">{t('assistant.thinking')}</div>}
        {error && (
          <div role="alert" className="text-[var(--color-error)]">
            {error}
          </div>
        )}
      </div>

      <footer className="border-t border-[var(--color-border-default)] px-3 py-2">
        <div className="flex items-end gap-2">
          <textarea
            rows={2}
            autoFocus
            value={input}
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={onKeyDown}
            aria-label={t('assistant.messageLabel')}
            placeholder={t('assistant.placeholder')}
            className="min-h-[40px] flex-1 resize-none rounded-lg border border-[var(--color-border-default)] bg-transparent px-2.5 py-1.5 text-[13px] text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]"
          />
          <button
            type="button"
            onClick={() => submit(input)}
            disabled={pending || input.trim() === ''}
            aria-label={t('assistant.send')}
            className="flex h-9 w-9 items-center justify-center rounded-lg bg-[var(--color-primary)] text-white disabled:opacity-40"
          >
            <Send {...iconProps('xs')} />
          </button>
        </div>
        <p className="m-0 mt-1.5 text-[10px] leading-snug text-[var(--color-text-secondary)]">
          {t('assistant.disclaimer', { provider: t(providerLabelKey(provider)) })}
        </p>
      </footer>
    </section>
  )
}

function ChatBubble({ entry }: { entry: ChatEntry }) {
  const { t } = useTranslation()
  const mine = entry.role === 'user'
  return (
    <div className={cn('flex flex-col', mine ? 'items-end' : 'items-start')}>
      <div
        className={cn(
          'max-w-[85%] whitespace-pre-wrap break-words rounded-lg px-3 py-2',
          mine
            ? 'bg-[var(--color-primary)] text-white'
            : 'bg-[color-mix(in_srgb,_var(--color-text-primary)_6%,_transparent)] text-[var(--color-text-primary)]',
        )}
      >
        {entry.content}
      </div>
      {entry.tools && entry.tools.length > 0 && (
        <div className="mt-1 text-[11px] text-[var(--color-text-secondary)]">
          {t('assistant.toolsUsed', {
            tools: entry.tools.map((tool) => (tool.ok ? tool.name : t('assistant.toolFailed', { name: tool.name }))).join(' · '),
          })}
        </div>
      )}
    </div>
  )
}

// providerLabelKey 는 공급자 id 를 읽기 좋은 이름의 i18n 키로 바꾼다.
function providerLabelKey(provider: string): string {
  return provider === 'upstage' || provider === 'openai-compatible'
    ? `assistant.providers.${provider}`
    : 'assistant.providers.unknown'
}

function errorText(error: unknown, t: (key: string) => string): string {
  const status = typeof error === 'object' && error !== null && 'status' in error ? Number(error.status) : 0
  if (status === 502) return t('assistant.errors.upstream')
  if (status === 504) return t('assistant.errors.timeout')
  if (status === 503) return t('assistant.errors.notConfigured')
  return t('assistant.errors.generic')
}
