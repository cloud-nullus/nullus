import { useState } from 'react'
import { Bell, Plus } from 'lucide-react'
import { useAlertRules, useCreateAlertRule, useUpdateAlertRule } from '../api/observability-api'
import type { AlertRule, AlertChannel, CreateAlertRuleRequest } from '../api/observability-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'

const MOCK_ALERT_RULES: AlertRule[] = [
  { id: 'r1', name: 'High CPU', condition: 'cpu_usage > threshold', threshold: '80%', channel: 'slack', enabled: true, createdAt: '2026-02-01T00:00:00Z' },
  { id: 'r2', name: 'Memory Warning', condition: 'memory_usage > threshold', threshold: '90%', channel: 'email', enabled: true, createdAt: '2026-02-05T00:00:00Z' },
  { id: 'r3', name: 'Pod CrashLoop', condition: 'pod_restart_count > threshold', threshold: '5', channel: 'slack', enabled: false, createdAt: '2026-03-01T00:00:00Z' },
]

const CHANNEL_BADGE: Record<AlertChannel, { bg: string; color: string }> = {
  slack: { bg: 'rgba(99,102,241,0.12)', color: '#a5b4fc' },
  email: { bg: 'rgba(16,185,129,0.12)', color: '#34d399' },
}

const selectStyle: React.CSSProperties = {
  background: 'rgba(255,255,255,0.04)',
  border: '1px solid var(--color-border-default)',
  borderRadius: '8px',
  padding: '9px 12px',
  fontSize: '14px',
  color: 'var(--color-text-primary)',
  cursor: 'pointer',
}

export function AlertRulesPage() {
  const { data: apiData } = useAlertRules()
  const rules = apiData?.items ?? MOCK_ALERT_RULES
  const createRule = useCreateAlertRule()
  const updateRule = useUpdateAlertRule()

  const [createModal, setCreateModal] = useState(false)
  const [form, setForm] = useState<CreateAlertRuleRequest>({ name: '', condition: '', threshold: '', channel: 'slack' })

  const handleCreate = () => {
    createRule.mutate(form, {
      onSuccess: () => {
        setCreateModal(false)
        setForm({ name: '', condition: '', threshold: '', channel: 'slack' })
      },
    })
  }

  const handleToggle = (rule: AlertRule) => {
    updateRule.mutate({ id: rule.id, data: { enabled: !rule.enabled } })
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
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              width: 'var(--icon-size)',
              height: 'var(--icon-size)',
              background: 'rgba(239,68,68,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#f87171',
            }}
          >
            <Bell size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Alert Rules
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              알림 규칙 목록 및 관리
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setCreateModal(true)}>
          <Plus size={15} />
          New Rule
        </Button>
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
              {['이름', '조건', '임계값', '채널', '활성', 'Actions'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rules.map((rule) => {
              const ch = CHANNEL_BADGE[rule.channel]
              return (
                <tr
                  key={rule.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)' }}
                  onMouseLeave={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent' }}
                >
                  <td style={tdStyle}><span style={{ fontWeight: 600 }}>{rule.name}</span></td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)', fontFamily: 'Fira Code, monospace', fontSize: '12px' }}>{rule.condition}</td>
                  <td style={{ ...tdStyle, fontFamily: 'Fira Code, monospace', fontSize: '13px' }}>{rule.threshold}</td>
                  <td style={tdStyle}>
                    <span style={{ padding: '2px 8px', borderRadius: '5px', background: ch.bg, color: ch.color, fontSize: '12px', fontWeight: 600 }}>
                      {rule.channel}
                    </span>
                  </td>
                  <td style={tdStyle}>
                    <label style={{ display: 'inline-flex', alignItems: 'center', cursor: 'pointer', gap: '8px' }}>
                      <div
                        onClick={() => handleToggle(rule)}
                        style={{
                          width: '36px',
                          height: '20px',
                          background: rule.enabled ? '#6366f1' : 'rgba(255,255,255,0.12)',
                          borderRadius: '10px',
                          position: 'relative',
                          cursor: 'pointer',
                          transition: 'background var(--transition-fast)',
                        }}
                      >
                        <div
                          style={{
                            position: 'absolute',
                            top: '2px',
                            left: rule.enabled ? '18px' : '2px',
                            width: '16px',
                            height: '16px',
                            background: '#fff',
                            borderRadius: '50%',
                            transition: 'left var(--transition-fast)',
                          }}
                        />
                      </div>
                      <span style={{ fontSize: '12px', color: rule.enabled ? '#a5b4fc' : 'var(--color-text-secondary)' }}>
                        {rule.enabled ? 'On' : 'Off'}
                      </span>
                    </label>
                  </td>
                  <td style={tdStyle}>
                    <Button variant="ghost" size="sm">Edit</Button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {rules.length === 0 && (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
            알림 규칙이 없습니다.
          </div>
        )}
      </div>

      {/* Create Rule Modal */}
      <Modal
        open={createModal}
        onClose={() => setCreateModal(false)}
        title="New Alert Rule"
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setCreateModal(false)}>Cancel</Button>
            <Button
              variant="primary"
              size="sm"
              loading={createRule.isPending}
              onClick={handleCreate}
              disabled={!form.name || !form.condition || !form.threshold}
            >
              Create
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Input
            label="규칙 이름"
            placeholder="예: High CPU"
            value={form.name}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
          />
          <Input
            label="조건"
            placeholder="예: cpu_usage > threshold"
            value={form.condition}
            onChange={(e) => setForm((p) => ({ ...p, condition: e.target.value }))}
          />
          <Input
            label="임계값"
            placeholder="예: 80%"
            value={form.threshold}
            onChange={(e) => setForm((p) => ({ ...p, threshold: e.target.value }))}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>채널</label>
            <select
              value={form.channel}
              onChange={(e) => setForm((p) => ({ ...p, channel: e.target.value as AlertChannel }))}
              style={selectStyle}
            >
              <option value="slack">Slack</option>
              <option value="email">Email</option>
            </select>
          </div>
        </div>
      </Modal>
    </div>
  )
}
