import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, Circle, Clock, Loader2, Terminal, XCircle } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
import { useStacks } from '../api/stack-api'
import type { Stack } from '../api/stack-api'
import { RetryStackButton } from '../components/retry-stack-button'
import type { StackStatus as RetryStackStatus } from '../utils/retry-policy'
import { getStatusStyle } from '../utils/status-style'

type LogLevel = 'info' | 'success' | 'warn' | 'error' | 'dim'

interface LogLine {
  time: string
  level: LogLevel
  text: string
}

interface DeploymentMeta {
  version: string
  reason: string
  who: string
  when: string
  result: 'success' | 'failed' | 'running'
  duration: string
}

const DEPLOYMENT_DATA: Record<string, { meta: DeploymentMeta; logs: LogLine[] }> = {
  'deploy-v3-20260302': {
    meta: {
      version: 'v3',
      reason: 'Grafana v10.2 → v10.3 upgrade',
      who: 'admin@nullus.io',
      when: '2026-03-02 14:30',
      result: 'success',
      duration: '42 min',
    },
    logs: [
      { time: '14:30:01', level: 'info', text: '[Stack] Starting deployment: Tool Upgrade (v2 → v3)' },
      { time: '14:30:02', level: 'dim', text: '[Helm] Fetching chart repository index...' },
      { time: '14:30:04', level: 'info', text: '[Helm] Resolved grafana/grafana@10.3.1' },
      { time: '14:30:05', level: 'info', text: '[Helm] Running: helm upgrade grafana grafana/grafana --version 10.3.1' },
      { time: '14:32:10', level: 'success', text: '[Helm] Release "grafana" has been upgraded. Happy Helming!' },
      { time: '14:32:11', level: 'info', text: '[K8s] Waiting for rollout: deployment/grafana ...' },
      { time: '14:33:45', level: 'dim', text: '[K8s] Pod grafana-7d9f8c6b4-xkqp2 → Pending' },
      { time: '14:34:12', level: 'dim', text: '[K8s] Pod grafana-7d9f8c6b4-xkqp2 → Running' },
      { time: '14:35:00', level: 'success', text: '[K8s] Rollout complete. 2/2 pods healthy.' },
      { time: '14:35:01', level: 'info', text: '[Health] Probing Grafana at http://grafana.monitoring.svc:3000/api/health' },
      { time: '14:35:06', level: 'success', text: '[Health] {"commit":"abc123","database":"ok","version":"10.3.1"}' },
      { time: '14:35:07', level: 'info', text: '[ArgoCD] Syncing application: grafana-stack' },
      { time: '14:40:18', level: 'success', text: '[ArgoCD] Application synced. Status: Healthy' },
      { time: '14:40:19', level: 'info', text: '[Snapshot] Saving stack version snapshot v3...' },
      { time: '14:40:20', level: 'success', text: '[Stack] Deployment completed successfully in 42m 19s' },
    ],
  },
  'deploy-v2-20260228': {
    meta: {
      version: 'v2',
      reason: 'Storage: AWS S3 → MinIO',
      who: 'kim@nullus.io',
      when: '2026-02-28 09:15',
      result: 'success',
      duration: '58 min',
    },
    logs: [
      { time: '09:15:00', level: 'info', text: '[Stack] Starting deployment: Config Change (v1 → v2)' },
      { time: '09:15:01', level: 'info', text: '[Plan] Replacing storage backend: s3 → minio' },
      { time: '09:15:03', level: 'dim', text: '[Helm] Rendering values for minio chart...' },
      { time: '09:15:05', level: 'info', text: '[Helm] Running: helm install minio bitnami/minio -f values.yaml' },
      { time: '09:22:14', level: 'success', text: '[Helm] Release "minio" installed.' },
      { time: '09:22:15', level: 'info', text: '[Migration] Starting data migration S3 → MinIO...' },
      { time: '09:45:33', level: 'warn', text: '[Migration] 3 objects skipped (size > 5GB). Manual copy required.' },
      { time: '09:58:01', level: 'success', text: '[Migration] Data migration complete. 2,847 objects transferred.' },
      { time: '09:58:02', level: 'info', text: '[Config] Updating artifact registry config to use MinIO endpoint...' },
      { time: '10:00:44', level: 'success', text: '[Config] Reconfigured: harbor, gitlab-runner → minio.storage.svc' },
      { time: '10:00:45', level: 'info', text: '[Health] Verifying all services with new storage...' },
      { time: '10:13:09', level: 'success', text: '[Stack] Deployment completed successfully in 58m 9s' },
    ],
  },
  'deploy-v1-20260220': {
    meta: {
      version: 'v1',
      reason: 'Initial stack deployment',
      who: 'admin@nullus.io',
      when: '2026-02-20 16:00',
      result: 'failed',
      duration: '12 min (aborted)',
    },
    logs: [
      { time: '16:00:00', level: 'info', text: '[Stack] Starting deployment: Initial Deploy' },
      { time: '16:00:01', level: 'info', text: '[Pre-flight] Checking cluster connectivity...' },
      { time: '16:00:03', level: 'success', text: '[Pre-flight] Cluster prod-k8s reachable. Nodes: 3/3 Ready.' },
      { time: '16:00:04', level: 'info', text: '[Helm] Installing argo-cd...' },
      { time: '16:02:30', level: 'success', text: '[Helm] Release "argo-cd" installed.' },
      { time: '16:02:31', level: 'info', text: '[Helm] Installing gitlab-ce...' },
      { time: '16:06:15', level: 'info', text: '[K8s] Waiting for gitlab-webservice rollout...' },
      { time: '16:08:41', level: 'warn', text: '[K8s] Pod gitlab-webservice-5d8f9b-zqr1x stuck in Pending (Insufficient memory)' },
      { time: '16:09:00', level: 'error', text: '[K8s] PodSchedulingError: 0/3 nodes available — insufficient memory.' },
      { time: '16:10:22', level: 'error', text: '[Health] Timeout: gitlab-webservice did not become Ready within 4m' },
      { time: '16:11:00', level: 'warn', text: '[Rollback] Initiating automatic rollback...' },
      { time: '16:11:45', level: 'info', text: '[Rollback] Uninstalling gitlab-ce...' },
      { time: '16:12:10', level: 'info', text: '[Rollback] Uninstalling argo-cd...' },
      { time: '16:12:30', level: 'error', text: '[Stack] Deployment FAILED after 12m 30s. Rollback complete.' },
      { time: '16:12:31', level: 'dim', text: '[Hint] Increase node memory or reduce gitlab.resources.requests.memory' },
    ],
  },
}

const LOG_LEVEL_STYLE: Record<LogLevel, string> = {
  info: 'text-[var(--color-text-primary)]',
  success: 'text-[#34d399]',
  warn: 'text-[#fbbf24]',
  error: 'text-[#f87171]',
  dim: 'text-[var(--color-text-muted)]',
}

const LOG_LEVEL_PREFIX: Record<LogLevel, string> = {
  info: '',
  success: '',
  warn: '⚠ ',
  error: '✗ ',
  dim: '',
}

type StageStatus = 'done' | 'failed' | 'pending'

interface Stage {
  label: string
  status: StageStatus
}

function getStages(result: 'success' | 'failed' | 'running'): Stage[] {
  if (result === 'success') {
    return [
      { label: 'Pre-flight', status: 'done' },
      { label: 'Helm Install', status: 'done' },
      { label: 'K8s Rollout', status: 'done' },
      { label: 'Health Check', status: 'done' },
      { label: 'Snapshot', status: 'done' },
    ]
  }
  if (result === 'failed') {
    return [
      { label: 'Pre-flight', status: 'done' },
      { label: 'Helm Install', status: 'done' },
      { label: 'K8s Rollout', status: 'failed' },
      { label: 'Health Check', status: 'pending' },
      { label: 'Snapshot', status: 'pending' },
    ]
  }
  return [
    { label: 'Pre-flight', status: 'done' },
    { label: 'Helm Install', status: 'done' },
    { label: 'K8s Rollout', status: 'pending' },
    { label: 'Health Check', status: 'pending' },
    { label: 'Snapshot', status: 'pending' },
  ]
}

export function StackDeploymentLogsPage() {
  const { deploymentId } = useParams<{ deploymentId: string }>()
  const navigate = useNavigate()
  const logEndRef = useRef<HTMLDivElement>(null)
  const [visibleCount, setVisibleCount] = useState(0)

  const entry = deploymentId ? DEPLOYMENT_DATA[deploymentId] : undefined

  // When the id does not match a fixture, fall back to the real stack list
  // and render a condensed live view with the Retry button.
  const { data: stacksData } = useStacks()
  const realStack = useMemo<Stack | undefined>(() => {
    if (entry || !deploymentId) return undefined
    return stacksData?.items?.find((s) => s.id === deploymentId)
  }, [stacksData, deploymentId, entry])

  const allLogs = entry?.logs ?? []
  const meta = entry?.meta

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset animation when deploymentId changes; allLogs is derived from deploymentId
  useEffect(() => {
    if (allLogs.length === 0) return
    setVisibleCount(0)
    const interval = setInterval(() => {
      setVisibleCount((prev) => {
        if (prev >= allLogs.length) {
          clearInterval(interval)
          return prev
        }
        return prev + 1
      })
    }, 80)
    return () => clearInterval(interval)
  }, [allLogs.length])

  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll to bottom whenever a new log line appears
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [visibleCount])

  const visibleLogs = allLogs.slice(0, visibleCount)
  const isStreaming = visibleCount < allLogs.length
  const stages = meta ? getStages(meta.result) : []

  if (!entry && realStack) {
    return (
      <RealStackView
        stack={realStack}
        onBack={() => navigate('/stack/list')}
        onRetried={() => navigate('/stack/list')}
      />
    )
  }

  return (
    <div>
      <Breadcrumb
        items={[
          { label: 'Stack List', path: '/stack/list' },
          { label: 'Deployment Logs' },
        ]}
      />

      <div className="mb-5 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(16,185,129,0.12)] text-[#34d399]">
            <Terminal size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Deployment Logs
            </h1>
            {meta && (
              <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
                {meta.version} · {meta.reason} · {meta.who} · {meta.when}
              </p>
            )}
          </div>
        </div>
        <Button variant="outline" size="md" type="button" onClick={() => navigate('/stack/list')}>
          <ArrowLeft size={14} />
          Back to Stack List
        </Button>
      </div>

      {meta && (
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <span
            className={cn(
              'flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold',
              meta.result === 'success'
                ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]'
                : meta.result === 'failed'
                  ? 'bg-[rgba(239,68,68,0.15)] text-[#f87171]'
                  : 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]'
            )}
          >
            {meta.result === 'success' ? (
              <CheckCircle2 size={12} />
            ) : meta.result === 'failed' ? (
              <XCircle size={12} />
            ) : (
              <Loader2 size={12} className="animate-spin" />
            )}
            {meta.result === 'success' ? 'Success' : meta.result === 'failed' ? 'Failed' : 'Running'}
          </span>
          <span className="flex items-center gap-1 text-[12px] text-[var(--color-text-secondary)]">
            <Clock size={12} />
            {meta.duration}
          </span>

          <div className="ml-2 flex items-center gap-0">
            {stages.map((stage, i) => (
              <div key={stage.label} className="flex items-center">
                {i > 0 && (
                  <div
                    className={cn(
                      'h-px w-6',
                      stage.status === 'pending' ? 'bg-[rgba(255,255,255,0.1)]' : 'bg-[rgba(34,197,94,0.4)]'
                    )}
                  />
                )}
                <div className="flex flex-col items-center gap-0.5">
                  {stage.status === 'done' ? (
                    <CheckCircle2 size={14} className="text-[#34d399]" />
                  ) : stage.status === 'failed' ? (
                    <XCircle size={14} className="text-[#f87171]" />
                  ) : (
                    <Circle size={14} className="text-[rgba(255,255,255,0.15)]" />
                  )}
                  <span
                    className={cn(
                      'text-[10px] font-medium whitespace-nowrap',
                      stage.status === 'done'
                        ? 'text-[#34d399]'
                        : stage.status === 'failed'
                          ? 'text-[#f87171]'
                          : 'text-[rgba(255,255,255,0.25)]'
                    )}
                  >
                    {stage.label}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d0f17]">
        <div className="flex items-center gap-2 border-b border-[rgba(255,255,255,0.06)] px-4 py-2.5">
          <div className="flex gap-1.5">
            <span className="h-3 w-3 rounded-full bg-[#ef4444]" />
            <span className="h-3 w-3 rounded-full bg-[#fbbf24]" />
            <span className="h-3 w-3 rounded-full bg-[#34d399]" />
          </div>
          <span className="ml-2 text-[11px] text-[rgba(255,255,255,0.3)]">
            deployment/{deploymentId}
          </span>
          {isStreaming && (
            <span className="ml-auto flex items-center gap-1 text-[11px] text-[#fbbf24]">
              <Loader2 size={11} className="animate-spin" />
              Streaming...
            </span>
          )}
          {!isStreaming && allLogs.length > 0 && (
            <span className="ml-auto text-[11px] text-[rgba(255,255,255,0.3)]">
              {allLogs.length} lines
            </span>
          )}
        </div>

        <div className="h-[560px] overflow-y-auto p-4 font-mono text-[13px] leading-[1.7]">
          {!entry && (
            <p className="text-[#f87171]">Deployment not found: {deploymentId}</p>
          )}
          {visibleLogs.map((line) => (
            <div key={`${line.time}-${line.text}`} className="flex gap-3">
              <span className="shrink-0 select-none text-[rgba(255,255,255,0.2)]">{line.time}</span>
              <span className={cn(LOG_LEVEL_STYLE[line.level], "whitespace-pre-wrap break-words")}>
                {LOG_LEVEL_PREFIX[line.level]}{line.text}
              </span>
            </div>
          ))}
          {isStreaming && (
            <div className="flex gap-3">
              <span className="shrink-0 select-none text-[rgba(255,255,255,0.2)]">
                {allLogs[visibleCount]?.time ?? ''}
              </span>
              <span className="inline-block h-[1em] w-2 animate-pulse bg-[#a5b4fc]" />
            </div>
          )}
          <div ref={logEndRef} />
        </div>
      </div>
    </div>
  )
}

interface RealStackViewProps {
  stack: Stack
  onBack: () => void
  onRetried: () => void
}

function RealStackView({ stack, onBack, onRetried }: RealStackViewProps) {
  const isFailed = stack.status === 'failed' || stack.status === 'rolled_back'
  const style = getStatusStyle(stack.status)
  return (
    <div>
      <Breadcrumb items={[{ label: 'Stack List', path: '/stack/list' }, { label: 'Deployment Logs' }]} />

      <div className="mb-5 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(16,185,129,0.12)] text-[#34d399]">
            <Terminal size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Deployment Logs
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {stack.name} · {stack.templateName || stack.templateId} · {stack.clusterName || stack.clusterId}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <RetryStackButton
            stackId={stack.id}
            status={stack.status as RetryStackStatus}
            onRetried={onRetried}
          />
          <Button variant="outline" size="md" type="button" onClick={onBack}>
            <ArrowLeft size={14} />
            Back to Stack List
          </Button>
        </div>
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <span
          className="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold"
          style={{ backgroundColor: style.bg, color: style.color }}
          data-testid="real-stack-status-badge"
        >
          {isFailed ? <XCircle size={12} /> : <CheckCircle2 size={12} />}
          {style.label}
        </span>
        <span className="flex items-center gap-1 text-[12px] text-[var(--color-text-secondary)]">
          <Clock size={12} />
          {stack.namespace ?? 'nullus'}
        </span>
      </div>

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d0f17]">
        <div className="flex items-center gap-2 border-b border-[rgba(255,255,255,0.06)] px-4 py-2.5">
          <div className="flex gap-1.5">
            <span className="h-3 w-3 rounded-full bg-[#ef4444]" />
            <span className="h-3 w-3 rounded-full bg-[#fbbf24]" />
            <span className="h-3 w-3 rounded-full bg-[#34d399]" />
          </div>
          <span className="ml-2 text-[11px] text-[rgba(255,255,255,0.3)]">deployment/{stack.id}</span>
        </div>
        <div className="p-6 text-[13px] text-[var(--color-text-secondary)]">
          Live log streaming is not yet connected. See the Stack List view for deployment events and metrics.
        </div>
      </div>
    </div>
  )
}
