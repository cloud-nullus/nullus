import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { CheckCircle, XCircle, Loader, Terminal } from 'lucide-react'
import { useDeployLog } from '../hooks/use-deploy-log'
import type { LogLevel, DeployStatus } from '../hooks/use-deploy-log'
import { api } from '../../../lib/api'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const PHASES = ['Initializing', 'Building', 'Deploying']
const PROGRESS_SEGMENTS = Array.from({ length: 100 }, (_, i) => i + 1)

const LOG_LEVEL_STYLE: Record<LogLevel, string> = {
  info: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  warn: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',
  error: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  success: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
}

function PhaseStep({ label, index, progress }: { label: string; index: number; progress: number }) {
  const phaseProgress = 100 / PHASES.length
  const phaseStart = index * phaseProgress
  const isDone = progress >= phaseStart + phaseProgress
  const isActive = progress >= phaseStart && !isDone

  return (
    <div className="flex items-center gap-2">
      <div
        className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-all duration-300',
          isDone
            ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]'
            : isActive
              ? 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]'
              : 'bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]'
        )}
      >
        {isDone ? (
          <CheckCircle size={15} />
        ) : isActive ? (
          <Loader size={15} className="animate-spin" />
        ) : (
          <span className="text-xs font-bold">{index + 1}</span>
        )}
      </div>
      <span
        className={cn(
          'text-[13px] font-semibold',
          isDone ? 'text-[#22c55e]' : isActive ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]'
        )}
      >
        {label}
      </span>
      {index < PHASES.length - 1 && (
        <div
          className={cn(
            'mx-1 h-px flex-1 transition-colors duration-300',
            isDone ? 'bg-[rgba(34,197,94,0.4)]' : 'bg-[var(--color-border-default)]'
          )}
        />
      )}
    </div>
  )
}

function StatusSummary({ status }: { status: DeployStatus }) {
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
      <div>
        <div className={cn('mb-0.5 text-[15px] font-bold', isSuccess ? 'text-[#22c55e]' : 'text-[#f87171]')}>
          {isSuccess ? '배포 완료' : '배포 실패'}
        </div>
        <div className="text-[13px] text-[var(--color-text-secondary)]">
          {isSuccess ? '모든 단계가 성공적으로 완료되었습니다.' : '배포 중 오류가 발생했습니다. 로그를 확인하세요.'}
        </div>
      </div>
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
  const logEndRef = useRef<HTMLDivElement>(null)
  const [apiState, setApiState] = useState<{ status: DeployStatus; progress: number } | null>(null)

  useEffect(() => {
    if (!id) return
    api.get<{ data: { state: string } }>(`/stacks/${id}/status`).then((r) => {
      const state = r.data?.data?.state ?? ''
      if (state) {
        setApiState({
          status: STATE_TO_STATUS[state] ?? 'connecting',
          progress: STATE_TO_PROGRESS[state] ?? 0,
        })
      }
    }).catch(() => {})
  }, [id])

  const hasWsData = logs.length > 0 || (wsStatus !== 'connecting' && wsStatus !== 'running')
  const status = hasWsData ? wsStatus : (apiState?.status ?? wsStatus)
  const progress = wsProgress > 0 ? wsProgress : (apiState?.progress ?? 0)

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  })

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
          {PHASES.map((phase, idx) => (
            <PhaseStep key={phase} label={phase} index={idx} progress={progress} />
          ))}
        </div>

        {/* Progress bar */}
        <div>
          <div className="mb-1.5 flex justify-between">
            <span className="text-xs text-[var(--color-text-secondary)]">전체 진행률</span>
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

      {/* Log console */}
      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d1117]">
        <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-2.5">
          <Terminal size={14} color="var(--color-text-secondary)" />
          <span className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
            Logs ({logs.length})
          </span>
        </div>
        <div
          className="h-[400px] overflow-y-auto p-3 font-mono text-xs leading-[1.7]"
        >
          {logs.length === 0 && (
            <div className="px-1 py-2 text-[var(--color-text-secondary)]">
              {isConnected ? '로그를 기다리는 중...' : 'WebSocket에 연결 중...'}
            </div>
          )}
          {logs.map((log) => {
            const lvl = LOG_LEVEL_STYLE[log.level]
            return (
              <div key={log.id} className="flex items-start gap-2.5 px-1 py-0.5">
                <span className="shrink-0 whitespace-nowrap text-[#475569]">
                  {new Date(log.timestamp).toISOString().slice(11, 19)}
                </span>
                <span
                  className={cn('shrink-0 whitespace-nowrap rounded px-1.5 text-[10px] font-bold leading-5', lvl)}
                >
                  {log.level.toUpperCase()}
                </span>
                <span className="break-words text-[#e2e8f0]">{log.message}</span>
              </div>
            )
          })}
          <div ref={logEndRef} />
        </div>
      </div>

      {/* Result summary */}
      <StatusSummary status={status} />
    </div>
  )
}
