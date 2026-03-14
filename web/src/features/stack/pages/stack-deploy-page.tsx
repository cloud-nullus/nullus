import { useEffect, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { CheckCircle, XCircle, Loader, Terminal } from 'lucide-react'
import { useDeployLog } from '../hooks/use-deploy-log'
import type { LogLevel, DeployStatus } from '../hooks/use-deploy-log'

const PHASES = ['Initializing', 'Building', 'Deploying']

const LOG_LEVEL_STYLE: Record<LogLevel, { bg: string; color: string }> = {
  info: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa' },
  warn: { bg: 'rgba(245,158,11,0.15)', color: '#fbbf24' },
  error: { bg: 'rgba(239,68,68,0.15)', color: '#f87171' },
  success: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' },
}

function PhaseStep({ label, index, progress }: { label: string; index: number; progress: number }) {
  const phaseProgress = 100 / PHASES.length
  const phaseStart = index * phaseProgress
  const isDone = progress >= phaseStart + phaseProgress
  const isActive = progress >= phaseStart && !isDone

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
      <div
        style={{
          width: '28px',
          height: '28px',
          borderRadius: '50%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: isDone
            ? 'rgba(34,197,94,0.15)'
            : isActive
            ? 'rgba(99,102,241,0.15)'
            : 'rgba(255,255,255,0.05)',
          color: isDone ? '#22c55e' : isActive ? '#818cf8' : 'var(--color-text-secondary)',
          flexShrink: 0,
          transition: 'all 0.3s ease',
        }}
      >
        {isDone ? (
          <CheckCircle size={15} />
        ) : isActive ? (
          <Loader size={15} style={{ animation: 'spin 1s linear infinite' }} />
        ) : (
          <span style={{ fontSize: '12px', fontWeight: 700 }}>{index + 1}</span>
        )}
      </div>
      <span
        style={{
          fontSize: '13px',
          fontWeight: 600,
          color: isDone ? '#22c55e' : isActive ? '#a5b4fc' : 'var(--color-text-secondary)',
        }}
      >
        {label}
      </span>
      {index < PHASES.length - 1 && (
        <div
          style={{
            flex: 1,
            height: '1px',
            background: isDone ? 'rgba(34,197,94,0.4)' : 'var(--color-border-default)',
            margin: '0 4px',
            transition: 'background 0.3s ease',
          }}
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
      style={{
        background: isSuccess ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
        border: `1px solid ${isSuccess ? 'rgba(34,197,94,0.3)' : 'rgba(239,68,68,0.3)'}`,
        borderRadius: 'var(--card-radius)',
        padding: '20px',
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        marginTop: '20px',
      }}
    >
      {isSuccess ? <CheckCircle size={24} color="#22c55e" /> : <XCircle size={24} color="#f87171" />}
      <div>
        <div style={{ fontSize: '15px', fontWeight: 700, color: isSuccess ? '#22c55e' : '#f87171', marginBottom: '2px' }}>
          {isSuccess ? '배포 완료' : '배포 실패'}
        </div>
        <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>
          {isSuccess ? '모든 단계가 성공적으로 완료되었습니다.' : '배포 중 오류가 발생했습니다. 로그를 확인하세요.'}
        </div>
      </div>
    </div>
  )
}

export function StackDeployPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { logs, status, progress, isConnected } = useDeployLog(id)
  const logEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  const cardStyle: React.CSSProperties = {
    background: 'var(--color-surface-card)',
    border: '1px solid var(--color-border-default)',
    borderRadius: 'var(--card-radius)',
    padding: 'var(--card-padding)',
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '24px' }}>
        <div
          style={{
            width: 'var(--icon-size)',
            height: 'var(--icon-size)',
            background: 'rgba(99,102,241,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#818cf8',
          }}
        >
          <Terminal size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Deployment Log
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            Deployment ID: {id}
            {' · '}
            <span style={{ color: isConnected ? '#22c55e' : '#f59e0b' }}>
              {isConnected ? 'Connected' : 'Connecting...'}
            </span>
          </p>
        </div>
      </div>

      {/* Phase steps */}
      <div style={{ ...cardStyle, marginBottom: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0', marginBottom: '16px', flexWrap: 'wrap' }}>
          {PHASES.map((phase, idx) => (
            <PhaseStep key={phase} label={phase} index={idx} progress={progress} />
          ))}
        </div>

        {/* Progress bar */}
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px' }}>
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>전체 진행률</span>
            <span style={{ fontSize: '12px', fontWeight: 700, color: 'var(--color-text-primary)' }}>{progress}%</span>
          </div>
          <div style={{ width: '100%', height: '8px', background: 'rgba(255,255,255,0.08)', borderRadius: '4px', overflow: 'hidden' }}>
            <div
              style={{
                width: `${progress}%`,
                height: '100%',
                background: status === 'failed' ? '#ef4444' : 'linear-gradient(90deg, #6366f1, #8b5cf6)',
                borderRadius: '4px',
                transition: 'width 0.4s ease',
              }}
            />
          </div>
        </div>
      </div>

      {/* Log console */}
      <div
        style={{
          background: '#0d1117',
          border: '1px solid var(--color-border-default)',
          borderRadius: 'var(--card-radius)',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            padding: '10px 16px',
            borderBottom: '1px solid var(--color-border-default)',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            background: 'rgba(255,255,255,0.02)',
          }}
        >
          <Terminal size={14} color="var(--color-text-secondary)" />
          <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            Logs ({logs.length})
          </span>
        </div>
        <div
          style={{
            padding: '12px',
            height: '400px',
            overflowY: 'auto',
            fontFamily: 'Fira Code, Cascadia Code, monospace',
            fontSize: '12px',
            lineHeight: 1.7,
          }}
        >
          {logs.length === 0 && (
            <div style={{ color: 'var(--color-text-secondary)', padding: '8px 4px' }}>
              {isConnected ? '로그를 기다리는 중...' : 'WebSocket에 연결 중...'}
            </div>
          )}
          {logs.map((log) => {
            const lvl = LOG_LEVEL_STYLE[log.level]
            return (
              <div key={log.id} style={{ display: 'flex', gap: '10px', padding: '2px 4px', alignItems: 'flex-start' }}>
                <span style={{ color: '#475569', whiteSpace: 'nowrap', flexShrink: 0 }}>
                  {new Date(log.timestamp).toISOString().slice(11, 19)}
                </span>
                <span
                  style={{
                    padding: '0px 6px',
                    borderRadius: '4px',
                    background: lvl.bg,
                    color: lvl.color,
                    fontSize: '10px',
                    fontWeight: 700,
                    whiteSpace: 'nowrap',
                    flexShrink: 0,
                    lineHeight: '20px',
                  }}
                >
                  {log.level.toUpperCase()}
                </span>
                <span style={{ color: '#e2e8f0', wordBreak: 'break-word' }}>{log.message}</span>
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
