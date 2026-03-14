import { useState } from 'react'
import { Layers, ChevronDown, ChevronRight, ShieldCheck } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Modal } from '../../../components/ui/modal'
import type { CompatibilityMatrix, CompatibilityValidationResult } from '../api/stack-api'

const MOCK_MATRIX: CompatibilityMatrix[] = [
  {
    id: 'm1',
    name: 'GitLab All-in-One v2.0',
    status: 'verified',
    k8sRange: '1.27 – 1.30',
    tools: [
      { name: 'GitLab', helmVersion: '7.9.0', appVersion: '16.9.0' },
      { name: 'ArgoCD', helmVersion: '6.7.0', appVersion: '2.10.0' },
      { name: 'Harbor', helmVersion: '1.14.0', appVersion: '2.10.0' },
      { name: 'Prometheus', helmVersion: '25.8.0', appVersion: '2.50.0' },
    ],
  },
  {
    id: 'm2',
    name: 'GitHub + ArgoCD v1.5',
    status: 'verified',
    k8sRange: '1.26 – 1.30',
    tools: [
      { name: 'ArgoCD', helmVersion: '6.5.0', appVersion: '2.9.0' },
      { name: 'Harbor', helmVersion: '1.13.0', appVersion: '2.9.0' },
      { name: 'Grafana', helmVersion: '7.3.0', appVersion: '10.3.0' },
    ],
  },
  {
    id: 'm3',
    name: 'Minimal Stack v0.9',
    status: 'untested',
    k8sRange: '1.28 – 1.30',
    tools: [
      { name: 'ArgoCD', helmVersion: '6.6.0', appVersion: '2.9.5' },
      { name: 'Loki', helmVersion: '5.42.0', appVersion: '2.9.0' },
    ],
  },
]

const MOCK_VALIDATION: CompatibilityValidationResult = {
  compatible: true,
  issues: [
    { tool: 'ArgoCD', message: 'Minor version mismatch with K8s 1.30 — upgrade recommended', severity: 'warning' },
  ],
  checkedAt: '2026-03-14T10:00:00Z',
}

const STATUS_BADGE: Record<string, { bg: string; color: string; label: string }> = {
  verified: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Verified' },
  untested: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Untested' },
}

export function StackVersionPage() {
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [validationOpen, setValidationOpen] = useState(false)
  const [validating, setValidating] = useState(false)
  const [validationResult, setValidationResult] = useState<CompatibilityValidationResult | null>(null)

  const handleValidate = () => {
    setValidating(true)
    setValidationOpen(true)
    setTimeout(() => {
      setValidationResult(MOCK_VALIDATION)
      setValidating(false)
    }, 1200)
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
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '28px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              width: 'var(--icon-size)',
              height: 'var(--icon-size)',
              background: 'rgba(34,197,94,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#4ade80',
            }}
          >
            <Layers size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Compatibility Matrix
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              스택 버전 호환성 매트릭스
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={handleValidate}>
          <ShieldCheck size={15} />
          Validate Current Stack
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
              <th style={{ ...thStyle, width: '32px' }} />
              {['매트릭스 이름', '상태', 'K8s 호환 범위'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {MOCK_MATRIX.map((matrix) => {
              const isExpanded = expandedId === matrix.id
              const badge = STATUS_BADGE[matrix.status] ?? STATUS_BADGE.untested
              return (
                <>
                  <tr
                    key={matrix.id}
                    style={{ transition: 'background var(--transition-fast)', cursor: 'pointer' }}
                    onClick={() => setExpandedId(isExpanded ? null : matrix.id)}
                    onMouseEnter={(e) => {
                      ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)'
                    }}
                    onMouseLeave={(e) => {
                      ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent'
                    }}
                  >
                    <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>
                      {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                    </td>
                    <td style={tdStyle}>
                      <span style={{ fontWeight: 600 }}>{matrix.name}</span>
                    </td>
                    <td style={tdStyle}>
                      <span
                        style={{
                          padding: '3px 9px',
                          borderRadius: '6px',
                          background: badge.bg,
                          color: badge.color,
                          fontSize: '12px',
                          fontWeight: 600,
                        }}
                      >
                        {badge.label}
                      </span>
                    </td>
                    <td style={{ ...tdStyle, fontFamily: 'Fira Code, monospace', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
                      {matrix.k8sRange}
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr key={`${matrix.id}-detail`}>
                      <td colSpan={4} style={{ borderTop: '1px solid var(--color-border-default)', padding: 0 }}>
                        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '16px 20px' }}>
                          <p
                            style={{
                              margin: '0 0 12px',
                              fontSize: '12px',
                              fontWeight: 600,
                              color: 'var(--color-text-secondary)',
                              textTransform: 'uppercase',
                              letterSpacing: '0.06em',
                            }}
                          >
                            도구별 버전
                          </p>
                          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '10px' }}>
                            {matrix.tools.map((tool) => (
                              <div
                                key={tool.name}
                                style={{
                                  background: 'rgba(255,255,255,0.04)',
                                  border: '1px solid var(--color-border-default)',
                                  borderRadius: '8px',
                                  padding: '12px 14px',
                                }}
                              >
                                <p style={{ margin: '0 0 6px', fontSize: '13px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                                  {tool.name}
                                </p>
                                <div style={{ display: 'flex', gap: '14px', fontSize: '12px', fontFamily: 'Fira Code, monospace' }}>
                                  <span>
                                    <span style={{ color: 'var(--color-text-secondary)' }}>helm: </span>
                                    <span style={{ color: '#a5b4fc' }}>{tool.helmVersion}</span>
                                  </span>
                                  <span>
                                    <span style={{ color: 'var(--color-text-secondary)' }}>app: </span>
                                    <span style={{ color: '#4ade80' }}>{tool.appVersion}</span>
                                  </span>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Validation Result Modal */}
      <Modal
        open={validationOpen}
        onClose={() => {
          setValidationOpen(false)
          setValidationResult(null)
        }}
        title="호환성 검증 결과"
      >
        {validating && (
          <div style={{ textAlign: 'center', padding: '32px 0', color: 'var(--color-text-secondary)' }}>
            <div
              style={{
                width: '32px',
                height: '32px',
                border: '3px solid rgba(255,255,255,0.1)',
                borderTopColor: '#a5b4fc',
                borderRadius: '50%',
                animation: 'spin 0.6s linear infinite',
                margin: '0 auto 12px',
              }}
            />
            검증 중...
          </div>
        )}
        {!validating && validationResult && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                padding: '12px 16px',
                borderRadius: '8px',
                background: validationResult.compatible ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)',
                border: `1px solid ${validationResult.compatible ? 'rgba(34,197,94,0.3)' : 'rgba(239,68,68,0.3)'}`,
              }}
            >
              <ShieldCheck size={20} color={validationResult.compatible ? '#22c55e' : '#ef4444'} />
              <span
                style={{
                  fontSize: '14px',
                  fontWeight: 700,
                  color: validationResult.compatible ? '#22c55e' : '#ef4444',
                }}
              >
                {validationResult.compatible ? '호환성 검증 통과' : '호환성 문제 발견'}
              </span>
            </div>

            {validationResult.issues.length > 0 && (
              <div>
                <p style={{ margin: '0 0 10px', fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                  이슈
                </p>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {validationResult.issues.map((issue, i) => (
                    <div
                      key={i}
                      style={{
                        padding: '10px 14px',
                        borderRadius: '8px',
                        background: issue.severity === 'error' ? 'rgba(239,68,68,0.08)' : 'rgba(245,158,11,0.08)',
                        border: `1px solid ${issue.severity === 'error' ? 'rgba(239,68,68,0.25)' : 'rgba(245,158,11,0.25)'}`,
                      }}
                    >
                      <span
                        style={{
                          fontSize: '12px',
                          fontWeight: 700,
                          color: issue.severity === 'error' ? '#f87171' : '#fbbf24',
                          marginRight: '8px',
                        }}
                      >
                        [{issue.tool}]
                      </span>
                      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)' }}>{issue.message}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <p style={{ margin: 0, fontSize: '12px', color: 'var(--color-text-secondary)' }}>
              검증 시각: {new Date(validationResult.checkedAt).toLocaleString('ko-KR')}
            </p>
          </div>
        )}
      </Modal>
    </div>
  )
}
