import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { AlertTriangle, CheckCircle, ChevronDown, ChevronRight, Circle, Loader, PlayCircle, Terminal, XCircle } from 'lucide-react'
import { toast } from 'sonner'
import { useDeployLog } from '../hooks/use-deploy-log'
import type { LogEntry, LogLevel, DeployStatus } from '../hooks/use-deploy-log'
import { usePodWatch } from '../hooks/use-pod-watch'
import type { PodWatchRow } from '../hooks/use-pod-watch'
import { useContinueStack } from '../api/stack-api'
import { api } from '../../../lib/api'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const PROGRESS_SEGMENTS = Array.from({ length: 100 }, (_, i) => i + 1)

const LOG_LEVEL_STYLE: Record<LogLevel, string> = {
  info: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  warn: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',
  error: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  success: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
}

const LOG_ROW_STYLE: Record<LogLevel, string> = {
  info: 'border-transparent bg-transparent',
  success: 'border-[rgba(34,197,94,0.25)] bg-[rgba(34,197,94,0.06)]',
  warn: 'border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)]',
  error: 'border-[rgba(239,68,68,0.4)] bg-[rgba(239,68,68,0.1)]',
}

type TimelineStatus = 'pending' | 'running' | 'success' | 'warn' | 'error'

interface DeployStage {
  key: string
  label: string
  steps: string[]
  progressAt: number
}

const DEPLOY_STAGES: DeployStage[] = [
  { key: 'validate', label: 'Validate', steps: ['validate'], progressAt: 5 },
  { key: 'install', label: 'Install', steps: ['installing_'], progressAt: 15 },
  { key: 'configure', label: 'Configure', steps: ['integration_check', 'configuring'], progressAt: 90 },
  { key: 'health', label: 'Health Check', steps: ['health_check'], progressAt: 96 },
  { key: 'complete', label: 'Complete', steps: ['completed'], progressAt: 100 },
]

const TERMINAL_FAILURE_STEPS = new Set(['failed', 'rolling_back', 'rolled_back', 'delete_failed'])

function stepMatches(stage: DeployStage, step?: string): boolean {
  if (!step) return false
  return stage.steps.some((candidate) => candidate.endsWith('_') ? step.startsWith(candidate) : step === candidate)
}

function deriveTimeline(logs: LogEntry[], progress: number, status: DeployStatus): Array<DeployStage & { status: TimelineStatus }> {
  const failedLog = logs.find((log) => log.level === 'error' || TERMINAL_FAILURE_STEPS.has(log.step ?? ''))
  const failedStageIndex = failedLog
    ? Math.max(0, DEPLOY_STAGES.findIndex((stage) => stepMatches(stage, failedLog.step)) || 0)
    : -1
  const latestStageIndex = logs.reduce((latest, log) => {
    const index = DEPLOY_STAGES.findIndex((stage) => stepMatches(stage, log.step))
    return index >= 0 ? Math.max(latest, index) : latest
  }, -1)
  const progressStageIndex = DEPLOY_STAGES.reduce((latest, stage, index) => (
    progress >= stage.progressAt ? index : latest
  ), -1)
  const activeIndex = Math.max(latestStageIndex, progressStageIndex, status === 'running' ? 0 : -1)

  return DEPLOY_STAGES.map((stage, index) => {
    const stageLogs = logs.filter((log) => stepMatches(stage, log.step))
    if (failedStageIndex === index || stageLogs.some((log) => log.level === 'error')) {
      return { ...stage, status: 'error' }
    }
    if (stageLogs.some((log) => log.level === 'warn')) {
      return { ...stage, status: 'warn' }
    }
    if (status === 'success' || progress >= stage.progressAt || index < activeIndex) {
      return { ...stage, status: 'success' }
    }
    if (index === activeIndex && status !== 'failed') {
      return { ...stage, status: 'running' }
    }
    return { ...stage, status: 'pending' }
  })
}

function TimelineStep({ stage, isLast }: { stage: DeployStage & { status: TimelineStatus }; isLast: boolean }) {
  const isDone = stage.status === 'success'
  const isRunning = stage.status === 'running'
  const isWarn = stage.status === 'warn'
  const isError = stage.status === 'error'

  return (
    <div className="flex min-w-[130px] flex-1 items-center gap-2">
      <div
        className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-all duration-300',
          isError && 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
          isWarn && 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',
          isDone && 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
          isRunning && 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]',
          stage.status === 'pending' && 'bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]'
        )}
      >
        {isError ? (
          <XCircle size={15} />
        ) : isWarn ? (
          <AlertTriangle size={15} />
        ) : isDone ? (
          <CheckCircle size={15} />
        ) : isRunning ? (
          <Loader size={15} className="animate-spin" />
        ) : (
          <Circle size={13} />
        )}
      </div>
      <span
        className={cn(
          'text-[13px] font-semibold',
          isError && 'text-[#f87171]',
          isWarn && 'text-[#fbbf24]',
          isDone && 'text-[#22c55e]',
          isRunning && 'text-[#a5b4fc]',
          stage.status === 'pending' && 'text-[var(--color-text-secondary)]'
        )}
      >
        {stage.label}
      </span>
      {!isLast && (
        <div
          className={cn(
            'mx-1 h-px flex-1 transition-colors duration-300',
            isError ? 'bg-[rgba(239,68,68,0.45)]' : isDone ? 'bg-[rgba(34,197,94,0.4)]' : 'bg-[var(--color-border-default)]'
          )}
        />
      )}
    </div>
  )
}

function LogLineRow({ log }: { log: LogEntry }) {
  return (
    <div className={cn('flex items-start gap-2.5 rounded border px-2 py-1', LOG_ROW_STYLE[log.level])}>
      <span className="shrink-0 whitespace-nowrap text-[#475569]">
        {new Date(log.timestamp).toISOString().slice(11, 19)}
      </span>
      <span
        className={cn('shrink-0 whitespace-nowrap rounded px-1.5 text-[10px] font-bold leading-5', LOG_LEVEL_STYLE[log.level])}
      >
        {log.level.toUpperCase()}
      </span>
      {log.step && (
        <span className="shrink-0 whitespace-nowrap rounded bg-[rgba(148,163,184,0.12)] px-1.5 text-[10px] font-semibold leading-5 text-[#94a3b8]">
          {log.step.replace(/_/g, ' ')}
        </span>
      )}
      <span className={cn('break-words', log.level === 'error' ? 'font-semibold text-[#fecaca]' : log.level === 'warn' ? 'font-semibold text-[#fde68a]' : 'text-[#e2e8f0]')}>
        {log.message}
      </span>
    </div>
  )
}

function podStatusClass(status: string): string {
  switch (status) {
    case 'Running':
      return 'text-[#34d399]'
    case 'Pending':
    case 'ContainerCreating':
    case 'Waiting':
      return 'text-[#fbbf24]'
    case 'Error':
      return 'text-[#f87171]'
    default:
      return 'text-[#e2e8f0]'
  }
}

function PodWatchPanel({
  rows,
  error,
  isConnected,
  namespace,
}: {
  rows: PodWatchRow[]
  error: string | null
  isConnected: boolean
  namespace: string
}) {
  const hasRows = rows.length > 0
  return (
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d1117]">
      <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-2.5">
        <Terminal size={14} color="var(--color-text-secondary)" />
        <span className="font-mono text-xs font-semibold text-[var(--color-text-secondary)]">
          $ kubectl get pods -n {namespace} -w
        </span>
        <span className={cn('ml-auto text-[11px]', isConnected ? 'text-[#34d399]' : 'text-[#fbbf24]')}>
          {isConnected ? (hasRows ? `${rows.length} pods` : 'Watching') : 'Connecting...'}
        </span>
      </div>
      <div className="h-[1200px] overflow-y-auto p-3 font-mono text-xs leading-[1.7]">
        {error && (
          <div className="mb-3 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-2 py-1.5 text-[#fca5a5]">
            {error}
          </div>
        )}
        <div className="grid grid-cols-[minmax(220px,1fr)_70px_120px_80px_60px] gap-3 border-b border-[rgba(255,255,255,0.08)] pb-1 text-[10px] font-bold uppercase tracking-[0.06em] text-[#64748b]">
          <span>Name</span>
          <span>Ready</span>
          <span>Status</span>
          <span>Restarts</span>
          <span>Age</span>
        </div>
        <div className="mt-2 space-y-1">
          {rows.length === 0 && !error && (
            <div className="px-1 py-2 text-[var(--color-text-secondary)]">
              {isConnected
                ? `No pods in namespace ${namespace} yet. Pods appear here as they are scheduled.`
                : 'Connecting to pod watch...'}
            </div>
          )}
          {rows.map((row) => (
            <div key={row.name} className="grid grid-cols-[minmax(220px,1fr)_70px_120px_80px_60px] gap-3 rounded px-1 py-0.5 text-[#cbd5e1] hover:bg-[rgba(255,255,255,0.04)]">
              <span className="truncate">{row.name}</span>
              <span>{row.ready}</span>
              <span className={cn('font-semibold', podStatusClass(row.status))}>{row.status}</span>
              <span>{row.restarts}</span>
              <span>{row.age}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function StatusSummary({
  status,
  latestFailureMessage,
  onContinue,
  isContinuing,
}: {
  status: DeployStatus
  latestFailureMessage?: string
  onContinue: () => void
  isContinuing: boolean
}) {
  if (status !== 'success' && status !== 'failed') return null

  const isSuccess = status === 'success'
  return (
    <div
      className={cn(
        'mt-5 flex items-center gap-3 rounded-[var(--card-radius)] border p-5',
        isSuccess
          ? 'border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.08)]'
          : 'border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.08)]'
      )}
    >
      {isSuccess ? <CheckCircle size={24} color="#22c55e" /> : <XCircle size={24} color="#f87171" />}
      <div className="min-w-0 flex-1">
        <div className={cn('mb-0.5 text-[15px] font-bold', isSuccess ? 'text-[#22c55e]' : 'text-[#f87171]')}>
          {isSuccess ? 'Deployment Completed' : 'Deployment Failed'}
        </div>
        <div className="text-[13px] text-[var(--color-text-secondary)]">
          {isSuccess
            ? 'All stages completed successfully.'
            : 'An error occurred during deployment. Check ERROR/failed lines in the Logs console below. If logs are empty, check recent failures in Stack List > selected stack > History.'}
        </div>
        {!isSuccess && latestFailureMessage && (
          <div className="mt-2 rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-2.5 py-2 text-[12px] text-[#fca5a5]">
            Latest failure reason: {latestFailureMessage}
          </div>
        )}
      </div>
      {!isSuccess && (
        <button
          type="button"
          onClick={onContinue}
          disabled={isContinuing}
          className="inline-flex shrink-0 items-center gap-1.5 rounded border border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.12)] px-3 py-2 text-xs font-bold text-[#86efac] hover:bg-[rgba(34,197,94,0.18)] disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isContinuing ? <Loader size={14} className="animate-spin" /> : <PlayCircle size={14} />}
          Continue
        </button>
      )}
    </div>
  )
}

const STATE_TO_STATUS: Record<string, DeployStatus> = {
  completed: 'success',
  failed: 'failed',
  rolled_back: 'failed',
  rolling_back: 'running',
  installing: 'running',
  configuring: 'running',
  health_check: 'running',
  validating: 'running',
}

const STATE_TO_PROGRESS: Record<string, number> = {
  pending: 0, validating: 5, installing: 40, configuring: 80,
  health_check: 90, completed: 100, failed: 0, rolling_back: 0, rolled_back: 0,
}

export function StackDeployPage() {
  const params = useParams<{ id?: string; deploymentId?: string }>()
  const id = params.id ?? params.deploymentId ?? ''
  const { logs, status: wsStatus, progress: wsProgress, isConnected } = useDeployLog(id)
  const { pods, error: podWatchError, isConnected: isPodWatchConnected, namespace: podWatchNamespace } = usePodWatch(id)
  const continueStack = useContinueStack()
  const logContainerRef = useRef<HTMLDivElement>(null)
  const shouldFollowLogsRef = useRef(true)
  const failureToastRef = useRef('')
  const [apiState, setApiState] = useState<{ status: DeployStatus; progress: number; namespace?: string } | null>(null)
  const [rawLogsOpen, setRawLogsOpen] = useState(true)

  useEffect(() => {
    if (!id) return
    api.get<{ data: { state: string; namespace?: string } }>(`/stacks/${id}/status`).then((r) => {
      const state = r.data?.data?.state ?? ''
      if (state) {
        setApiState({
          status: STATE_TO_STATUS[state] ?? 'connecting',
          progress: STATE_TO_PROGRESS[state] ?? 0,
          namespace: r.data?.data?.namespace,
        })
      }
    }).catch(() => {})
  }, [id])

  const hasWsData = logs.length > 0 || (wsStatus !== 'connecting' && wsStatus !== 'running')
  const status = hasWsData ? wsStatus : (apiState?.status ?? wsStatus)
  const progress = wsProgress > 0 ? wsProgress : (apiState?.progress ?? 0)
  const podNamespace = podWatchNamespace || apiState?.namespace || '...'
  const latestFailureLog = [...logs].reverse().find((log) => {
    if (log.level === 'error') return true
    const normalized = log.message.toLowerCase()
    return normalized.includes('failed') || normalized.includes('error')
  })
  const highlightedLogs = logs.filter((log) => log.level === 'warn' || log.level === 'error')
  const timeline = deriveTimeline(logs, progress, status)

  useEffect(() => {
    if (status !== 'failed' || !latestFailureLog) return
    const toastKey = `${id}:${latestFailureLog.id}:${latestFailureLog.message}`
    if (failureToastRef.current === toastKey) return
    failureToastRef.current = toastKey
    toast.error('Deployment failed', {
      description: latestFailureLog.message,
    })
  }, [id, latestFailureLog, status])

  const handleContinue = () => {
    if (!id) return
    const toastId = toast.loading('Continuing deployment...')
    continueStack.mutate(
      { stackId: id },
      {
        onSuccess: () => {
          toast.success('Deployment continued.', { id: toastId })
          setApiState({ status: 'running', progress: Math.max(progress, 5) })
        },
        onError: (err) => {
          const message = err instanceof Error ? err.message : 'Failed to continue deployment.'
          toast.error(message, { id: toastId })
        },
      }
    )
  }

  const handleLogScroll = () => {
    const el = logContainerRef.current
    if (!el) return
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    shouldFollowLogsRef.current = distanceFromBottom < 48
  }

  useEffect(() => {
    const el = logContainerRef.current
    if (!el || !rawLogsOpen || !shouldFollowLogsRef.current) return
    el.scrollTop = el.scrollHeight
  }, [logs.length, rawLogsOpen])

  return (
    <div>
      <Breadcrumb items={[{ label: 'Stack List', path: '/stack/list' }, { label: 'Deployment Log' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-center gap-2.5">
        <div
          className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
        >
          <Terminal size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Deployment Log
          </h1>
          <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
            Deployment ID: {id}
            {' · '}
            <span className={cn(isConnected ? 'text-[#22c55e]' : 'text-[#f59e0b]')}>
              {isConnected ? 'Connected' : 'Connecting...'}
            </span>
          </p>
        </div>
      </div>

      {/* Phase steps */}
      <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
        <div className="mb-4 flex flex-wrap items-center gap-0">
          {timeline.map((stage, idx) => (
            <TimelineStep key={stage.key} stage={stage} isLast={idx === timeline.length - 1} />
          ))}
        </div>

        {/* Progress bar */}
        <div>
          <div className="mb-1.5 flex justify-between">
            <span className="text-xs text-[var(--color-text-secondary)]">Overall Progress</span>
            <span className="text-xs font-bold text-[var(--color-text-primary)]">{progress}%</span>
          </div>
          <div className="flex h-2 w-full overflow-hidden rounded bg-[rgba(255,255,255,0.08)]">
            {PROGRESS_SEGMENTS.map((segment) => (
              <div
                key={segment}
                className={cn(
                  'h-full w-[1%] transition-colors duration-300',
                  segment <= progress
                    ? status === 'failed'
                      ? 'bg-[#ef4444]'
                      : 'bg-[linear-gradient(90deg,#6366f1,#8b5cf6)]'
                    : 'bg-transparent'
                )}
              />
            ))}
          </div>
        </div>
      </div>

      {status === 'failed' && latestFailureLog && (
        <div className="mb-4 rounded-[var(--card-radius)] border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] p-4">
          <div className="mb-1 flex items-center gap-2 text-sm font-bold text-[#f87171]">
            <XCircle size={16} />
            Deployment error
          </div>
          <div className="font-mono text-xs leading-[1.7] text-[#fecaca]">
            {latestFailureLog.message}
          </div>
        </div>
      )}

      {highlightedLogs.length > 0 && (
        <div className="mb-4 overflow-hidden rounded-[var(--card-radius)] border border-[rgba(245,158,11,0.25)] bg-[rgba(245,158,11,0.04)]">
          <div className="flex items-center gap-2 border-b border-[rgba(245,158,11,0.18)] px-4 py-2.5">
            <AlertTriangle size={14} className="text-[#fbbf24]" />
            <span className="text-xs font-semibold uppercase tracking-[0.06em] text-[#fbbf24]">
              Attention ({highlightedLogs.length})
            </span>
          </div>
          <div className="space-y-1.5 p-3 font-mono text-xs leading-[1.7]">
            {highlightedLogs.map((log) => (
              <LogLineRow key={log.id} log={log} />
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        {/* Log console */}
        <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d1117]">
          <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-2.5">
            <Terminal size={14} color="var(--color-text-secondary)" />
            <span className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
              Raw Logs ({logs.length})
            </span>
            {status === 'failed' && (
              <button
                type="button"
                onClick={handleContinue}
                disabled={continueStack.isPending}
                className="ml-auto inline-flex items-center gap-1.5 rounded border border-[rgba(34,197,94,0.35)] bg-[rgba(34,197,94,0.12)] px-2.5 py-1 text-[11px] font-bold text-[#86efac] hover:bg-[rgba(34,197,94,0.18)] disabled:cursor-not-allowed disabled:opacity-60"
              >
                {continueStack.isPending ? <Loader size={13} className="animate-spin" /> : <PlayCircle size={13} />}
                Continue
              </button>
            )}
            <button
              type="button"
              onClick={() => setRawLogsOpen((open) => !open)}
              className={cn(
                'inline-flex items-center gap-1 rounded border border-[var(--color-border-default)] px-2 py-1 text-[11px] font-semibold text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
                status !== 'failed' && 'ml-auto'
              )}
            >
              {rawLogsOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
              {rawLogsOpen ? 'Hide' : 'Show'}
            </button>
          </div>
          {rawLogsOpen && (
            <div
              ref={logContainerRef}
              onScroll={handleLogScroll}
              className="h-[1200px] overflow-y-auto p-3 font-mono text-xs leading-[1.7]"
            >
              {logs.length === 0 && (
                <div className="px-1 py-2 text-[var(--color-text-secondary)]">
                  {status === 'failed'
                    ? 'Unable to receive live logs. Check recent failures in Stack List > selected stack > History.'
                    : isConnected
                      ? 'Waiting for logs...'
                      : 'Connecting to WebSocket...'}
                </div>
              )}
              <div className="space-y-1">
                {logs.map((log) => (
                  <LogLineRow key={log.id} log={log} />
                ))}
              </div>
            </div>
          )}
        </div>

        <PodWatchPanel rows={pods} error={podWatchError} isConnected={isPodWatchConnected} namespace={podNamespace} />
      </div>

      {/* Result summary */}
      <StatusSummary
        status={status}
        latestFailureMessage={latestFailureLog?.message}
        onContinue={handleContinue}
        isContinuing={continueStack.isPending}
      />
    </div>
  )
}
