import { useState } from 'react'
import { Users, Plus, Mail } from 'lucide-react'
import { useMembers, useInviteMember } from '../api/admin-api'
import type { MemberRole, MemberStatus, InviteMemberRequest } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'

const ORG_ID = 'org-1'

const MOCK_USERS = [
  { id: 'u1', name: 'Alice Kim', email: 'alice@nullus.io', role: 'admin' as MemberRole, status: 'active' as MemberStatus, joinedAt: '2026-01-05T00:00:00Z' },
  { id: 'u2', name: 'Bob Lee', email: 'bob@nullus.io', role: 'devops' as MemberRole, status: 'active' as MemberStatus, joinedAt: '2026-01-10T00:00:00Z' },
  { id: 'u3', name: 'Carol Park', email: 'carol@nullus.io', role: 'developer' as MemberRole, status: 'pending' as MemberStatus, joinedAt: '2026-03-01T00:00:00Z' },
  { id: 'u4', name: 'David Choi', email: 'david@nullus.io', role: 'developer' as MemberRole, status: 'inactive' as MemberStatus, joinedAt: '2026-02-15T00:00:00Z' },
]

const STATUS_BADGE: Record<MemberStatus, { bg: string; color: string; label: string }> = {
  active: { bg: 'rgba(34,197,94,0.15)', color: '#22c55e', label: 'Active' },
  pending: { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b', label: 'Pending' },
  inactive: { bg: 'rgba(100,116,139,0.15)', color: '#64748b', label: 'Inactive' },
}

const ROLE_BADGE: Record<MemberRole, { bg: string; color: string }> = {
  admin: { bg: 'rgba(239,68,68,0.15)', color: '#f87171' },
  devops: { bg: 'rgba(99,102,241,0.15)', color: '#a5b4fc' },
  developer: { bg: 'rgba(34,197,94,0.15)', color: '#34d399' },
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

export function UserManagementPage() {
  const { data: membersData } = useMembers(ORG_ID)
  const users = membersData?.items ?? MOCK_USERS
  const inviteMember = useInviteMember(ORG_ID)

  const [inviteModal, setInviteModal] = useState(false)
  const [inviteForm, setInviteForm] = useState<InviteMemberRequest>({ email: '', role: 'developer' })

  const handleInvite = () => {
    inviteMember.mutate(inviteForm, {
      onSuccess: () => {
        setInviteModal(false)
        setInviteForm({ email: '', role: 'developer' })
      },
    })
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
              background: 'rgba(139,92,246,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#c4b5fd',
            }}
          >
            <Users size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              User Management
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              사용자 목록 및 역할 관리
            </p>
          </div>
        </div>
        <Button variant="primary" size="md" onClick={() => setInviteModal(true)}>
          <Plus size={15} />
          Invite User
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
              {['이름', '이메일', '역할', '상태', 'Actions'].map((h) => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {users.map((user) => {
              const st = STATUS_BADGE[user.status]
              const role = ROLE_BADGE[user.role]
              return (
                <tr
                  key={user.id}
                  style={{ transition: 'background var(--transition-fast)' }}
                  onMouseEnter={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'rgba(255,255,255,0.02)' }}
                  onMouseLeave={(e) => { ;(e.currentTarget as HTMLTableRowElement).style.background = 'transparent' }}
                >
                  <td style={tdStyle}><span style={{ fontWeight: 600 }}>{user.name}</span></td>
                  <td style={{ ...tdStyle, color: 'var(--color-text-secondary)' }}>{user.email}</td>
                  <td style={tdStyle}>
                    <select
                      defaultValue={user.role}
                      style={{
                        ...selectStyle,
                        padding: '4px 10px',
                        background: role.bg,
                        color: role.color,
                        border: 'none',
                        fontWeight: 600,
                        fontSize: '12px',
                      }}
                    >
                      <option value="admin">Admin</option>
                      <option value="devops">DevOps</option>
                      <option value="developer">Developer</option>
                    </select>
                  </td>
                  <td style={tdStyle}>
                    <span style={{ padding: '3px 9px', borderRadius: '6px', background: st.bg, color: st.color, fontSize: '12px', fontWeight: 600 }}>
                      {st.label}
                    </span>
                  </td>
                  <td style={tdStyle}>
                    <Button variant="danger" size="sm">
                      비활성화
                    </Button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {users.length === 0 && (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
            사용자가 없습니다.
          </div>
        )}
      </div>

      {/* Invite modal */}
      <Modal
        open={inviteModal}
        onClose={() => setInviteModal(false)}
        title="Invite User"
        footer={
          <>
            <Button variant="outline" size="sm" onClick={() => setInviteModal(false)}>Cancel</Button>
            <Button
              variant="primary"
              size="sm"
              loading={inviteMember.isPending}
              onClick={handleInvite}
              disabled={!inviteForm.email}
            >
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
            placeholder="user@example.com"
            value={inviteForm.email}
            onChange={(e) => setInviteForm((p) => ({ ...p, email: e.target.value }))}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)' }}>역할</label>
            <select
              value={inviteForm.role}
              onChange={(e) => setInviteForm((p) => ({ ...p, role: e.target.value as MemberRole }))}
              style={selectStyle}
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
