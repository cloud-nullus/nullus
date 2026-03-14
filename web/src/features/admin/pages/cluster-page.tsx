import { useState } from 'react'
import { Network, Plus, CheckCircle, Clock, AlertCircle, MinusCircle } from 'lucide-react'
import { useClusters, useCreateCluster } from '../api/admin-api'
import type { Cluster, ClusterType, ClusterStatus, CreateClusterRequest } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'

const MOCK_CLUSTERS: Cluster[] = [
  {
    id: 'c1',
    name: 'prod-cluster',
    type: 'eks',
    endpoint: 'https://prod.k8s.nullus.io',
    status: 'connected',
    organizationIds: ['org-1'],
    createdAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'c2',
    name: 'staging-cluster',
    type: 'kubernetes',
    endpoint: 'https://staging.k8s.nullus.io',
    status: 'connected',
    organizationIds: ['org-1'],
    createdAt: '2026-01-15T00:00:00Z',
  },
  {
    id: 'c3',
    name: 'dev-cluster',
    type: 'k3s',
    endpoint: 'https://dev.k8s.nullus.io',
    status: 'pending',
    organizationIds: ['org-1'],
    createdAt: '2026-03-01T00:00:00Z',
  },
]

const STATUS_CONFIG: Record<ClusterStatus, { icon: React.ReactNode; bg: string; color: string; label: string }> = {
  connected: { icon: <CheckCircle size={14} />, bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Connected' },
  pending: { icon: <Clock size={14} />, bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  error: { icon: <AlertCircle size={14} />, bg: 'rgba(239,68,68,0.15)', color: '#ef4444', label: 'Error' },
  inactive: { icon: <MinusCircle size={14} />, bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Inactive' },
}

export function ClusterPage() {
  const { data: clustersData } = useClusters()
  const clusters = clustersData?.items ?? MOCK_CLUSTERS
  const createCluster = useCreateCluster()

  const [selected, setSelected] = useState<Cluster | null>(clusters[0] ?? null)
  const [registerModal, setRegisterModal] = useState(false)
  const [form, setForm] = useState<CreateClusterRequest>({ name: '', type: 'kubernetes', kubeconfig: '' })

  const handleRegister = () => {
    createCluster.mutate(form, {
      onSuccess: () => {
        setRegisterModal(false)
        setForm({ name: '', type: 'kubernetes', kubeconfig: '' })
      },
    })
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
              background: 'rgba(59,130,246,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#60a5fa',
            }}
          >
            <Network size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Cluster Management
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              쿠버네티스 클러스터를 등록하고 관리합니다.
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setRegisterModal(true)}>
          <Plus size={15} />
          Register Cluster
        </Button>
      </div>

      {/* Split layout */}
      <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
        {/* Left: Cluster list */}
        <div
          style={{
            width: '280px',
            flexShrink: 0,
            background: 'var(--color-surface-card)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--color-border-default)',
              fontSize: '12px',
              fontWeight: 600,
              color: 'var(--color-text-secondary)',
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
            }}
          >
            Clusters ({clusters.length})
          </div>
          {clusters.map((cluster) => {
            const st = STATUS_CONFIG[cluster.status]
            const isSelected = selected?.id === cluster.id
            return (
              <div
                key={cluster.id}
                onClick={() => setSelected(cluster)}
                style={{
                  padding: '14px 16px',
                  borderBottom: '1px solid var(--color-border-default)',
                  cursor: 'pointer',
                  background: isSelected ? 'rgba(99,102,241,0.1)' : 'transparent',
                  borderLeft: `3px solid ${isSelected ? '#6366f1' : 'transparent'}`,
                  transition: 'all var(--transition-fast)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 600, color: isSelected ? '#a5b4fc' : 'var(--color-text-primary)' }}>
                    {cluster.name}
                  </span>
                  <span
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '4px',
                      padding: '2px 7px',
                      borderRadius: '5px',
                      background: st.bg,
                      color: st.color,
                      fontSize: '11px',
                      fontWeight: 600,
                    }}
                  >
                    {st.icon}
                    {st.label}
                  </span>
                </div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
                  {cluster.type.toUpperCase()}
                </div>
              </div>
            )
          })}
        </div>

        {/* Right: Cluster detail */}
        {selected ? (
          <div style={{ flex: 1, minWidth: 0 }}>
            <div
              style={{
                background: 'var(--color-surface-card)',
                border: '1px solid var(--color-border-default)',
                borderRadius: 'var(--card-radius)',
                padding: '20px',
                marginBottom: '16px',
              }}
            >
              <h2 style={{ margin: '0 0 18px 0', fontSize: '16px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                {selected.name}
              </h2>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                {[
                  ['클러스터 이름', selected.name],
                  ['타입', selected.type.toUpperCase()],
                  ['엔드포인트', selected.endpoint],
                  ['등록일', new Date(selected.createdAt).toLocaleDateString('ko-KR')],
                ].map(([label, val]) => (
                  <div key={label}>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                      {label}
                    </div>
                    <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--color-text-primary)', wordBreak: 'break-all' }}>
                      {val}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Connection status card */}
            <div
              style={{
                background: 'var(--color-surface-card)',
                border: '1px solid var(--color-border-default)',
                borderRadius: 'var(--card-radius)',
                padding: '20px',
                marginBottom: '16px',
              }}
            >
              <h3 style={{ margin: '0 0 14px 0', fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                연결 상태
              </h3>
              {(() => {
                const st = STATUS_CONFIG[selected.status]
                return (
                  <div
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '8px',
                      padding: '10px 16px',
                      background: st.bg,
                      border: `1px solid ${st.color}40`,
                      borderRadius: '8px',
                      color: st.color,
                      fontSize: '14px',
                      fontWeight: 600,
                    }}
                  >
                    {st.icon}
                    {st.label}
                  </div>
                )
              })()}
            </div>

            {/* Organization access */}
            <div
              style={{
                background: 'var(--color-surface-card)',
                border: '1px solid var(--color-border-default)',
                borderRadius: 'var(--card-radius)',
                padding: '20px',
              }}
            >
              <h3 style={{ margin: '0 0 12px 0', fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                Organization Access
              </h3>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                {selected.organizationIds.map((oid) => (
                  <span
                    key={oid}
                    style={{
                      padding: '4px 10px',
                      borderRadius: '6px',
                      background: 'rgba(139,92,246,0.12)',
                      color: '#c4b5fd',
                      fontSize: '12px',
                      fontWeight: 500,
                    }}
                  >
                    {oid}
                  </span>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <div
            style={{
              flex: 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '200px',
              color: 'var(--color-text-secondary)',
              fontSize: '14px',
            }}
          >
            클러스터를 선택하세요.
          </div>
        )}
      </div>

      {/* Register Cluster modal */}
      <Modal
        open={registerModal}
        onClose={() => setRegisterModal(false)}
        title="Register Cluster"
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setRegisterModal(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={createCluster.isPending}
              onClick={handleRegister}
              disabled={!form.name || !form.kubeconfig}
            >
              Register
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Input
            label="클러스터 이름"
            placeholder="예: prod-cluster"
            value={form.name}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>
              클러스터 타입
            </label>
            <select
              value={form.type}
              onChange={(e) => setForm((p) => ({ ...p, type: e.target.value as ClusterType }))}
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '8px',
                padding: '9px 12px',
                fontSize: '14px',
                color: 'var(--color-text-primary)',
              }}
            >
              <option value="kubernetes">Kubernetes</option>
              <option value="eks">AWS EKS</option>
              <option value="gke">GCP GKE</option>
              <option value="aks">Azure AKS</option>
              <option value="k3s">K3s</option>
            </select>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>
              kubeconfig (YAML)
            </label>
            <textarea
              value={form.kubeconfig}
              onChange={(e) => setForm((p) => ({ ...p, kubeconfig: e.target.value }))}
              placeholder="kubeconfig 내용을 붙여넣으세요..."
              rows={8}
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '12px',
                color: 'var(--color-text-primary)',
                fontFamily: 'Fira Code, monospace',
                resize: 'vertical',
                outline: 'none',
              }}
            />
          </div>
        </div>
      </Modal>
    </div>
  )
}
