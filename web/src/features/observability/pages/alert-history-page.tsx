import { useState } from 'react'
import { BellRing } from 'lucide-react'
import { useAlertHistory } from '../api/observability-api'
import type { AlertHistoryEntry, AlertSeverity } from '../api/observability-api'

const MOCK_ALERT_HISTORY: AlertHistoryEntry[] = [
  { id: 'ah1', ruleName: 'High CPU', severity: 'critical', message: 'CPU usage exceeded 80% on prod-cluster', firedAt: '2026-03-14T07:30:00Z', resolvedAt: '2026-03-14T07:45:00Z' },
  { id: 'ah2', ruleName: 'Memory Warning', severity: 'warning', message: 'Memory usage at 85% on staging-cluster', firedAt: '2026-03-13T15:00:00Z', resolvedAt: '2026-03-13T15:20:00Z' },
  { id: 'ah3', ruleName: 'Pod CrashLoop', severity: 'critical', message: 'Pod api-server-xyz in CrashLoopBackOff', firedAt: '2026-03-12T11:00:00Z', resolvedAt: null },
  { id: 'ah4', ruleName: 'High CPU', severity: 'info', message: 'CPU usage returned to normal', firedAt: '2026-03-11T09:00:00Z', resolvedAt: '2026-03-11T09:01:00Z' },
]

const SEVERITY_BADGE: Record<AlertSeverity, { bg: string; color: string; label: string }> = {
  critical: { bg: 'rgba(239,68,68,0.15)', color: '#f87171', label: 'Critical' },
  warning: { bg: 'rgba(245,158,11,0.15)', color: '#fbbf24', label: 'Warning' },
  info: { bg: 'rgba(59,130,246,0.15)', color: '#60a5fa', label: 'Info' },
}

function formatDate(iso: string | null) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function AlertHistoryPage() {
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('')

  const { data: apiData } = useAlertHistory(severityFilter ? { severity: severityFilter } : undefined)
  const history = apiData?.items ?? MOCK_ALERT_HISTORY

  const filtered = severityFilter ? history.filter((h) => h.severity === severityFilter) : history

  const selectStyle: React.CSSProperties = {
    background: 'rgba(255,255,255,0.04)',
    border: '1px solid var(--color-border-default)',
    borderRadius: '8px',
    padding: '9px 12px',
    fontSize: '14px',
    color: 'var(--color-text-primary)',
    cursor: 'pointer',
  }

  const thStyle: React.CSSProperties = {
    padding: '10px 14px',
    textAlign: 'left',
    fontSize: '11px',
    fontWeight: 600,
    color: 'var(--color-text-secondary)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    whiteSpace: 'nowrap',
  }

  const tdStyle: React.CSSProperties = {
    padding: '12px 14px',
    fontSize: '14px',
    color: 'var(--color-text-primary)',
    borderTop: '1px solid var(--color-border-default)',
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '28px' }}>
        <div
          style={{
            width: 'var(--icon-size)',
            height: 'var(--icon-size)',
            background: 'rgba(245,158,11,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fbbf24',
          }}
        >
          <BellRing size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Alert History
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            알림 발생 이력
          </p>
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '10px', marginBottom: '16px' }}>
        <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')} style={selectStyle}>
          <option value="">All Severity</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
      </div>

      {/* Table */}
      <div
        style={{
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          borderRadius: 'var(--card-radius)',
          overflow: 'hidden',
        }}
      >
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.02)' }}>
              {['알림명', '심각도', '메시지', '발생 시간', '해결 시간'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((entry) => {
              const sev = SEVERITY_BADGE[entry.severity]
              return (
                <tr
                  key={entry.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)' }}
                  onMouseLeave={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent' }}
                >
                  <td style={tdStyle}><span style={{ fontWeight: 600 }}>{entry.ruleName}</span></td>
                  <td style={tdStyle}>
                    <span style={{ padding: '3px 9px', borderRadius: '6px', background: sev.bg, color: sev.color, fontSize: '12px', fontWeight: 600 }}>
                      {sev.label}
                    </span>
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px', maxWidth: '360px' }}>{entry.message}</td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontSize: '13px', whiteSpace: 'nowrap' }}>{formatDate(entry.firedAt)}</td>
                  <td style={{ ...tdStyle, fontSize: '13px', whiteSpace: 'nowrap' }}>
                    {entry.resolvedAt ? (
                      <span style={{ color: '#22c55e' }}>{formatDate(entry.resolvedAt)}</span>
                    ) : (
                      <span style={{ color: '#f87171' }}>미해결</span>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {filtered.length === 0 && (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
            알림 이력이 없습니다.
          </div>
        )}
      </div>
    </div>
  )
}
