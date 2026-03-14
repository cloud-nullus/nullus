import { Cpu, HardDrive, MemoryStick, Box, CheckCircle, AlertCircle, XCircle } from 'lucide-react'
import { useDashboard } from '../api/observability-api'
import type { MonitoringDashboard, ToolHealthStatus } from '../api/observability-api'

const MOCK_DASHBOARD: MonitoringDashboard = {
  kpi: {
    cpuUsage: 42,
    memoryUsage: 67,
    storageUsage: 55,
    podCount: 24,
    podRunning: 22,
  },
  pipeline: {
    successRate: 87,
    totalRuns: 142,
    avgBuildSeconds: 183,
  },
  tools: [
    { name: 'GitLab', version: '16.10.1', status: 'running' },
    { name: 'ArgoCD', version: '2.10.0', status: 'running' },
    { name: 'Prometheus', version: '2.50.0', status: 'warning' },
    { name: 'Grafana', version: '10.3.1', status: 'running' },
    { name: 'OpenSearch', version: '2.12.0', status: 'running' },
    { name: 'Harbor', version: '2.10.0', status: 'error' },
  ],
}

const TOOL_STATUS_CONFIG: Record<ToolHealthStatus, { icon: React.ReactNode; bg: string; color: string; label: string }> = {
  running: { icon: <CheckCircle size={13} />, bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Running' },
  warning: { icon: <AlertCircle size={13} />, bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Warning' },
  error: { icon: <XCircle size={13} />, bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Error' },
}

function UsageBar({ value, color }: { value: number; color: string }) {
  return (
    <div style={{ width: '100%', height: '6px', background: 'rgba(255,255,255,0.08)', borderRadius: '3px', overflow: 'hidden', marginTop: '8px' }}>
      <div style={{ width: `${value}%`, height: '100%', background: color, borderRadius: '3px', transition: 'width 0.4s ease' }} />
    </div>
  )
}

export function MonitoringPage() {
  const { data: apiData } = useDashboard(5000)
  const dashboard = apiData ?? MOCK_DASHBOARD
  const { kpi, pipeline, tools } = dashboard

  const kpiCards = [
    { label: 'CPU 사용률', value: `${kpi.cpuUsage}%`, icon: <Cpu size={18} />, color: '#60a5fa', bg: 'rgba(59,130,246,0.15)', bar: kpi.cpuUsage },
    { label: '메모리 사용률', value: `${kpi.memoryUsage}%`, icon: <MemoryStick size={18} />, color: '#a78bfa', bg: 'rgba(139,92,246,0.15)', bar: kpi.memoryUsage },
    { label: '스토리지', value: `${kpi.storageUsage}%`, icon: <HardDrive size={18} />, color: '#34d399', bg: 'rgba(16,185,129,0.15)', bar: kpi.storageUsage },
    { label: 'Pod 수', value: `${kpi.podRunning} / ${kpi.podCount}`, icon: <Box size={18} />, color: '#fbbf24', bg: 'rgba(245,158,11,0.15)', bar: Math.round((kpi.podRunning / kpi.podCount) * 100) },
  ]

  const cardStyle: React.CSSProperties = {
    background: 'var(--color-surface-card)',
    border: '1px solid var(--color-border-default)',
    borderRadius: 'var(--card-radius)',
    padding: 'var(--card-padding)',
  }

  const sectionTitle: React.CSSProperties = {
    margin: '0 0 16px 0',
    fontSize: '15px',
    fontWeight: 700,
    color: 'var(--color-text-primary)',
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '28px' }}>
        <div
          style={{
            width: 'var(--icon-size)',
            height: 'var(--icon-size)',
            background: 'rgba(59,130,246,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#60a5fa',
          }}
        >
          <Cpu size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Monitoring Dashboard
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            클러스터 및 도구 상태 실시간 모니터링 (5초 자동 갱신)
          </p>
        </div>
      </div>

      {/* KPI Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
          gap: '16px',
          marginBottom: '24px',
        }}
      >
        {kpiCards.map((card) => (
          <div key={card.label} style={cardStyle}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '10px' }}>
              <div
                style={{
                  width: '36px',
                  height: '36px',
                  background: card.bg,
                  borderRadius: '8px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: card.color,
                  flexShrink: 0,
                }}
              >
                {card.icon}
              </div>
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', fontWeight: 500 }}>{card.label}</span>
            </div>
            <div style={{ fontSize: '28px', fontWeight: 800, color: 'var(--color-text-primary)', lineHeight: 1 }}>
              {card.value}
            </div>
            <UsageBar value={card.bar} color={card.color} />
          </div>
        ))}
      </div>

      {/* Pipeline Status */}
      <div style={{ ...cardStyle, marginBottom: '24px' }}>
        <h2 style={sectionTitle}>Pipeline Status</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '20px' }}>
          <div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', marginBottom: '8px', fontWeight: 500 }}>성공률</div>
            <div style={{ fontSize: '26px', fontWeight: 800, color: '#22c55e', marginBottom: '8px' }}>
              {pipeline.successRate}%
            </div>
            <UsageBar value={pipeline.successRate} color="#22c55e" />
          </div>
          <div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', marginBottom: '8px', fontWeight: 500 }}>총 실행 수</div>
            <div style={{ fontSize: '26px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              {pipeline.totalRuns}
            </div>
          </div>
          <div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', marginBottom: '8px', fontWeight: 500 }}>평균 빌드 시간</div>
            <div style={{ fontSize: '26px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              {Math.floor(pipeline.avgBuildSeconds / 60)}m {pipeline.avgBuildSeconds % 60}s
            </div>
          </div>
        </div>
      </div>

      {/* Tool Health */}
      <div style={cardStyle}>
        <h2 style={sectionTitle}>Tool Health</h2>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
            gap: '12px',
          }}
        >
          {tools.map((tool) => {
            const cfg = TOOL_STATUS_CONFIG[tool.status]
            return (
              <div
                key={tool.name}
                style={{
                  padding: '14px',
                  background: 'rgba(255,255,255,0.02)',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '10px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>{tool.name}</span>
                  <span
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '4px',
                      padding: '2px 8px',
                      borderRadius: '5px',
                      background: cfg.bg,
                      color: cfg.color,
                      fontSize: '11px',
                      fontWeight: 600,
                    }}
                  >
                    {cfg.icon}
                    {cfg.label}
                  </span>
                </div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>v{tool.version}</div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
