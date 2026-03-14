import { useState } from 'react'
import { Settings, Plus, Trash2, Mail } from 'lucide-react'
import { useOrganization, useUpdateOrganization, useMembers, useInviteMember, useRemoveMember } from '../api/admin-api'
import type { OrgStatus, MemberRole, MemberStatus, UpdateOrgRequest, InviteMemberRequest } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'

const MOCK_ORG = {
  id: 'org-1',
  name: 'Cloud Nullus',
  slug: 'cloud-nullus',
  domain: 'nullus.io',
  status: 'active' as OrgStatus,
  clusterAccessScope: ['prod-cluster', 'staging-cluster'],
  createdAt: '2026-01-01T00:00:00Z',
}

const MOCK_MEMBERS = [
  { id: 'm1', name: 'Alice Kim', email: 'alice@nullus.io', role: 'admin' as MemberRole, status: 'active' as MemberStatus, joinedAt: '2026-01-05T00:00:00Z' },
  { id: 'm2', name: 'Bob Lee', email: 'bob@nullus.io', role: 'devops' as MemberRole, status: 'active' as MemberStatus, joinedAt: '2026-01-10T00:00:00Z' },
  { id: 'm3', name: 'Carol Park', email: 'carol@nullus.io', role: 'developer' as MemberRole, status: 'pending' as MemberStatus, joinedAt: '2026-03-01T00:00:00Z' },
]

const STATUS_BADGE: Record<MemberStatus, { bg: string; color: string }> = {
  active: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b' },
  inactive: { bg: 'rgba(100,116,139,0.15)', color: '#64748b' },
}

const ROLE_BADGE: Record<MemberRole, { bg: string; color: string }> = {
  admin: { bg: 'rgba(239,68,68,0.15)', color: '#f87171' },
  devops: { bg: 'rgba(99,102,241,0.15)', color: '#a5b4fc' },
  developer: { bg: 'rgba(34,197,94,0.15)', color: '#34d399' },
}

const ALL_CLUSTERS = ['prod-cluster', 'staging-cluster', 'dev-cluster', 'test-cluster']

export function OrganizationPage() {
  const { data: orgData } = useOrganization()
  const org = orgData ?? MOCK_ORG
  const orgId = org.id

  const { data: membersData } = useMembers(orgId)
  const members = membersData?.items ?? MOCK_MEMBERS

  const updateOrg = useUpdateOrganization()
  const inviteMember = useInviteMember(orgId)
  const removeMember = useRemoveMember(orgId)

  const [form, setForm] = useState<UpdateOrgRequest>({
    name: org.name,
    slug: org.slug,
    domain: org.domain,
    status: org.status,
    clusterAccessScope: [...org.clusterAccessScope],
  })

  const [inviteModal, setInviteModal] = useState(false)
  const [inviteForm, setInviteForm] = useState<InviteMemberRequest>({ email: '', role: 'developer' })

  const handleSave = () => {
    updateOrg.mutate(form)
  }

  const handleInvite = () => {
    inviteMember.mutate(inviteForm, {
      onSuccess: () => {
        setInviteModal(false)
        setInviteForm({ email: '', role: 'developer' })
      },
    })
  }

  const handleScopeToggle = (cluster: string) => {
    const current = form.clusterAccessScope ?? []
    setForm((prev) => ({
      ...prev,
      clusterAccessScope: current.includes(cluster)
        ? current.filter((c) => c !== cluster)
        : [...current, cluster],
    }))
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
            background: 'rgba(139,92,246,0.15)',
            borderRadius: 'var(--icon-radius)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#c4b5fd',
          }}
        >
          <Settings size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Organization
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            조직 정보 및 멤버를 관리합니다.
          </p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px', marginBottom: '24px' }}>
        {/* Org info form */}
        <div
          style={{
            background: 'var(--color-surface-card)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
            padding: '20px',
          }}
        >
          <h2 style={{ margin: '0 0 16px 0', fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
            조직 정보
          </h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <Input
              label="조직 이름"
              value={form.name ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
            />
            <Input
              label="슬러그 (Slug)"
              value={form.slug ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, slug: e.target.value }))}
            />
            <Input
              label="도메인"
              value={form.domain ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, domain: e.target.value }))}
            />
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>상태</label>
              <select
                value={form.status ?? 'active'}
                onChange={(e) => setForm((p) => ({ ...p, status: e.target.value as OrgStatus }))}
                style={{
                  background: 'rgba(255,255,255,0.04)',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '8px',
                  padding: '9px 12px',
                  fontSize: '14px',
                  color: 'var(--color-text-primary)',
                }}
              >
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
                <option value="suspended">Suspended</option>
              </select>
            </div>
            <Button variant="primary" size="sm" loading={updateOrg.isPending} onClick={handleSave}>
              저장
            </Button>
          </div>
        </div>

        {/* Cluster access scope */}
        <div
          style={{
            background: 'var(--color-surface-card)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
            padding: '20px',
          }}
        >
          <h2 style={{ margin: '0 0 16px 0', fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
            클러스터 접근 범위
          </h2>
          <p style={{ margin: '0 0 14px 0', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
            이 조직에서 접근 가능한 클러스터를 선택하세요.
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {ALL_CLUSTERS.map((cluster) => {
              const checked = (form.clusterAccessScope ?? []).includes(cluster)
              return (
                <label
                  key={cluster}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '10px',
                    cursor: 'pointer',
                    padding: '8px 10px',
                    borderRadius: '6px',
                    background: checked ? 'rgba(99,102,241,0.08)' : 'transparent',
                    border: `1px solid ${checked ? 'rgba(99,102,241,0.3)' : 'transparent'}`,
                    transition: 'all var(--transition-fast)',
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => handleScopeToggle(cluster)}
                    style={{ accentColor: '#6366f1', width: '15px', height: '15px' }}
                  />
                  <span style={{ fontSize: '14px', color: checked ? '#a5b4fc' : 'var(--color-text-primary)' }}>
                    {cluster}
                  </span>
                </label>
              )
            })}
          </div>
        </div>
      </div>

      {/* Members table */}
      <div
        style={{
          background: 'var(--color-surface-card)',
          border: '1px solid var(--color-border-default)',
          borderRadius: 'var(--card-radius)',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 18px',
            borderBottom: '1px solid var(--color-border-default)',
          }}
        >
          <h2 style={{ margin: 0, fontSize: '14px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
            멤버 관리
          </h2>
          <Button variant="secondary" size="sm" onClick={() => setInviteModal(true)}>
            <Plus size={13} />
            Invite Member
          </Button>
        </div>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.02)' }}>
              {['이름', '이메일', '역할', '상태', 'Actions'].map((h) => (
                <th
                  key={h}
                  style={{
                    padding: '10px 14px',
                    textAlign: 'left',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--color-text-secondary)',
                    textTransform: 'uppercase',
                    letterSpacing: '0.06em',
                  }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {members.map((m) => (
              <tr key={m.id}>
                <td style={tdStyle}>
                  <span style={{ fontWeight: 600 }}>{m.name}</span>
                </td>
                <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{m.email}</td>
                <td style={tdStyle}>
                  <span
                    style={{
                      padding: '2px 8px',
                      borderRadius: '5px',
                      background: ROLE_BADGE[m.role].bg,
                      color: ROLE_BADGE[m.role].color,
                      fontSize: '12px',
                      fontWeight: 600,
                    }}
                  >
                    {m.role}
                  </span>
                </td>
                <td style={tdStyle}>
                  <span
                    style={{
                      padding: '2px 8px',
                      borderRadius: '5px',
                      background: STATUS_BADGE[m.status].bg,
                      color: STATUS_BADGE[m.status].color,
                      fontSize: '12px',
                      fontWeight: 600,
                    }}
                  >
                    {m.status}
                  </span>
                </td>
                <td style={tdStyle}>
                  <Button
                    variant="danger"
                    size="sm"
                    loading={removeMember.isPending}
                    onClick={() => removeMember.mutate(m.id)}
                  >
                    <Trash2 size={13} />
                    Remove
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Invite modal */}
      <Modal
        open={inviteModal}
        onClose={() => setInviteModal(false)}
        title="Invite Member"
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setInviteModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" size="sm" loading={inviteMember.isPending} onClick={handleInvite}>
              <Mail size={13} />
              Send Invite
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Input
            label="이메일"
            type="email"
            placeholder="member@example.com"
            value={inviteForm.email}
            onChange={(e) => setInviteForm((p) => ({ ...p, email: e.target.value }))}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>역할</label>
            <select
              value={inviteForm.role}
              onChange={(e) => setInviteForm((p) => ({ ...p, role: e.target.value as MemberRole }))}
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '8px',
                padding: '9px 12px',
                fontSize: '14px',
                color: 'var(--color-text-primary)',
              }}
            >
              <option value="developer">Developer</option>
              <option value="devops">DevOps</option>
              <option value="admin">Admin</option>
            </select>
          </div>
        </div>
      </Modal>
    </div>
  )
}
