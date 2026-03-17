import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { GitBranch, Mail, Plus, Server, Shield, Users } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useMembers, useInviteMember, useUpdateUserRole, useDeactivateUser, useOrganization } from '../api/admin-api'
import type { MemberRole, MemberStatus } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { DataTable } from '../../../components/shared/data-table'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

type ActiveRoleTab = 'all' | MemberRole

type MenuCategory = 'stack' | 'cicd' | 'observability' | 'admin'

const MENU_CATEGORY_LABEL: Record<MenuCategory, string> = {
  stack: 'DevSecOps Stack',
  cicd: 'CI/CD',
  observability: 'Observability',
  admin: 'Admin',
}

interface MenuAccess {
  name: string
  access: 'View' | 'Edit'
  category: MenuCategory
}

interface ButtonGroup {
  category: MenuCategory
  items: string[]
}

interface RolePermission {
  role: MemberRole
  color: { bg: string; text: string; border: string; accessEdit: string; accessView: string }
  icon: typeof Shield
  menus: MenuAccess[]
  buttons: ButtonGroup[]
}

const ROLE_PERMISSIONS: RolePermission[] = [
  {
    role: 'admin',
    color: {
      bg: 'rgba(239,68,68,0.07)',
      text: '#f87171',
      border: 'rgba(239,68,68,0.22)',
      accessEdit: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
      accessView: 'bg-[rgba(239,68,68,0.08)] text-[#fca5a5]',
    },
    icon: Shield,
    menus: [
      { name: 'Stack Template', access: 'Edit', category: 'stack' },
      { name: 'Stack List', access: 'Edit', category: 'stack' },
      { name: 'Stack Install', access: 'Edit', category: 'stack' },
      { name: 'Stack History', access: 'View', category: 'stack' },
      { name: 'CI/CD Template', access: 'Edit', category: 'cicd' },
      { name: 'CI/CD List', access: 'Edit', category: 'cicd' },
      { name: 'CI/CD History', access: 'View', category: 'cicd' },
      { name: 'Monitoring', access: 'View', category: 'observability' },
      { name: 'Alert Rules', access: 'Edit', category: 'observability' },
      { name: 'Organization', access: 'Edit', category: 'admin' },
      { name: 'User Management', access: 'Edit', category: 'admin' },
      { name: 'Clusters', access: 'Edit', category: 'admin' },
      { name: 'Known Issues', access: 'Edit', category: 'admin' },
    ],
    buttons: [
      { category: 'stack', items: ['Install Stack', 'Deploy Stack'] },
      { category: 'cicd', items: ['Create / Edit / Delete Template', 'Create Pipeline', 'Deploy Pipeline'] },
      { category: 'admin', items: ['Invite / Remove Member', 'Create / Delete Cluster', 'New Organization'] },
    ],
  },
  {
    role: 'devops',
    color: {
      bg: 'rgba(99,102,241,0.07)',
      text: '#a5b4fc',
      border: 'rgba(99,102,241,0.22)',
      accessEdit: 'bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]',
      accessView: 'bg-[rgba(99,102,241,0.08)] text-[#c7d2fe]',
    },
    icon: Server,
    menus: [
      { name: 'Stack Template', access: 'View', category: 'stack' },
      { name: 'Stack List', access: 'Edit', category: 'stack' },
      { name: 'Stack Install', access: 'Edit', category: 'stack' },
      { name: 'Stack History', access: 'View', category: 'stack' },
      { name: 'CI/CD Template', access: 'View', category: 'cicd' },
      { name: 'CI/CD List', access: 'Edit', category: 'cicd' },
      { name: 'CI/CD History', access: 'View', category: 'cicd' },
      { name: 'Monitoring', access: 'View', category: 'observability' },
      { name: 'Alert Rules', access: 'View', category: 'observability' },
    ],
    buttons: [
      { category: 'stack', items: ['Install Stack', 'Deploy Stack'] },
      { category: 'cicd', items: ['Create Pipeline', 'Deploy Pipeline'] },
    ],
  },
  {
    role: 'developer',
    color: {
      bg: 'rgba(34,197,94,0.07)',
      text: '#34d399',
      border: 'rgba(34,197,94,0.22)',
      accessEdit: 'bg-[rgba(34,197,94,0.2)] text-[#34d399]',
      accessView: 'bg-[rgba(34,197,94,0.08)] text-[#6ee7b7]',
    },
    icon: GitBranch,
    menus: [
      { name: 'Developer Deploy', access: 'Edit', category: 'cicd' },
      { name: 'CI/CD List', access: 'View', category: 'cicd' },
      { name: 'CI/CD History', access: 'View', category: 'cicd' },
      { name: 'Monitoring', access: 'View', category: 'observability' },
      { name: 'Alert History', access: 'View', category: 'observability' },
    ],
    buttons: [
      { category: 'cicd', items: ['Developer Self-Service Deploy'] },
    ],
  },
]

const STATUS_BADGE: Record<MemberStatus, { className: string; label: string }> = {
  active: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Active' },
  pending: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Pending' },
  inactive: { className: 'bg-[rgba(100,116,139,0.15)] text-[#64748b]', label: 'Inactive' },
}

const ROLE_BADGE: Record<MemberRole, { className: string }> = {
  admin: { className: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]' },
  devops: { className: 'bg-[rgba(99,102,241,0.15)] text-[#a5b4fc]' },
  developer: { className: 'bg-[rgba(34,197,94,0.15)] text-[#34d399]' },
}

const selectClassName = 'cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'

const inviteUserSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
  role: z.enum(['admin', 'devops', 'developer']),
})

type InviteUserFormData = z.infer<typeof inviteUserSchema>

const INVITE_USER_DEFAULTS: InviteUserFormData = {
  name: '',
  email: '',
  role: 'developer',
}

export function UserManagementPage() {
  const { data: orgData } = useOrganization()
  const ORG_ID = orgData?.id ?? ''
  const { data: membersData, isLoading } = useMembers(ORG_ID)
  const users = membersData?.items ?? []
  const inviteMember = useInviteMember(ORG_ID)
  const updateUserRole = useUpdateUserRole(ORG_ID)
  const deactivateUser = useDeactivateUser(ORG_ID)

  const [activeMainTab, setActiveMainTab] = useState<'roles' | 'users'>('roles')
  const [activeRoleTab, setActiveRoleTab] = useState<ActiveRoleTab>('all')
  const [inviteModal, setInviteModal] = useState(false)
  const [menuAccessOverrides, setMenuAccessOverrides] = useState<
    Partial<Record<MemberRole, Record<string, 'View' | 'Edit'>>>
  >({})

  const hasRoleChanges = Object.values(menuAccessOverrides).some(
    (r) => Object.keys(r ?? {}).length > 0
  )

  const getMenuAccess = (role: MemberRole, menuName: string, defaultAccess: 'View' | 'Edit'): 'View' | 'Edit' =>
    menuAccessOverrides[role]?.[menuName] ?? defaultAccess

  const toggleMenuAccess = (role: MemberRole, menuName: string, currentAccess: 'View' | 'Edit') => {
    setMenuAccessOverrides((prev) => ({
      ...prev,
      [role]: {
        ...prev[role],
        [menuName]: currentAccess === 'Edit' ? 'View' : 'Edit',
      },
    }))
  }

  const handleSaveRoleChanges = () => {
    setMenuAccessOverrides({})
  }
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isValid, isSubmitting },
  } = useForm<InviteUserFormData>({
    resolver: zodResolver(inviteUserSchema),
    defaultValues: INVITE_USER_DEFAULTS,
    mode: 'onChange',
  })
  const [search, setSearch] = useState('')
  const [deactivateUserId, setDeactivateUserId] = useState<string | null>(null)
  const [roleOverrides, setRoleOverrides] = useState<Record<string, MemberRole>>({})

  const filteredUsers = users.filter((user) => {
    const query = search.trim().toLowerCase()
    const matchesSearch = !query || user.name.toLowerCase().includes(query) || user.email.toLowerCase().includes(query)
    const matchesRole = activeRoleTab === 'all' || user.role === activeRoleTab
    return matchesSearch && matchesRole
  })

  const handleInvite = (data: InviteUserFormData) => {
    inviteMember.mutate({ name: data.name, email: data.email, role: data.role }, {
      onSuccess: () => {
        setInviteModal(false)
        reset(INVITE_USER_DEFAULTS)
      },
    })
  }

  const handleRoleChange = (memberId: string, role: MemberRole) => {
    setRoleOverrides((prev) => ({ ...prev, [memberId]: role }))
    updateUserRole.mutate({ memberId, role })
  }

  const handleDeactivate = () => {
    if (!deactivateUserId) return
    deactivateUser.mutate(deactivateUserId, {
      onSuccess: () => {
        setDeactivateUserId(null)
      },
    })
  }

  const columns: ColumnDef<(typeof filteredUsers)[number], unknown>[] = [
    {
      accessorKey: 'name',
      header: '이름',
      cell: ({ row }) => <span className="font-semibold">{row.original.name}</span>,
    },
    {
      accessorKey: 'email',
      header: '이메일',
      cell: ({ row }) => <span className="text-[var(--color-text-secondary)]">{row.original.email}</span>,
    },
    {
      accessorKey: 'role',
      header: '역할',
      cell: ({ row }) => {
        const selectedRole = roleOverrides[row.original.id] ?? row.original.role
        const role = ROLE_BADGE[selectedRole]
        return (
          <select
            value={selectedRole}
            onChange={(event) => handleRoleChange(row.original.id, event.target.value as MemberRole)}
            className={cn('cursor-pointer rounded-[5px] border-0 px-2.5 py-1 text-xs font-semibold', role.className)}
          >
            <option value="admin">Admin</option>
            <option value="devops">DevOps</option>
            <option value="developer">Developer</option>
          </select>
        )
      },
    },
    {
      accessorKey: 'status',
      header: '상태',
      cell: ({ row }) => {
        const st = STATUS_BADGE[row.original.status]
        return (
          <span className={cn('rounded-md px-[9px] py-[3px] text-xs font-semibold', st.className)}>
            {st.label}
          </span>
        )
      },
    },
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      cell: ({ row }) => (
        <Button variant="danger" size="sm" type="button" onClick={() => setDeactivateUserId(row.original.id)}>
          비활성화
        </Button>
      ),
    },
  ]

  return (
    <div>
      <Breadcrumb items={[{ label: 'User Management' }]} />

      {/* Page header */}
      <div className="mb-6 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]">
          <Users size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">User Management</h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">사용자 목록 및 역할 관리</p>
        </div>
      </div>

      {/* Main tabs */}
      <div className="mb-6 flex items-center justify-between border-b border-[var(--color-border-default)]">
        <div className="flex">
          {(['roles', 'users'] as const).map((tab) => {
            const active = activeMainTab === tab
            return (
              <button
                key={tab}
                type="button"
                onClick={() => setActiveMainTab(tab)}
                className={cn(
                  '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-5 py-2.5 text-sm font-medium transition-all duration-150',
                  active ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]'
                )}
              >
                {tab === 'roles' ? 'Role Permissions' : 'Users'}
              </button>
            )
          })}
        </div>
        <div className="pb-2">
          {activeMainTab === 'roles' && (
            <Button
              variant="primary"
              size="sm"
              type="button"
              disabled={!hasRoleChanges}
              onClick={handleSaveRoleChanges}
            >
              Save Changes
            </Button>
          )}
          {activeMainTab === 'users' && (
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={() => {
                reset(INVITE_USER_DEFAULTS)
                setInviteModal(true)
              }}
            >
              <Plus size={13} />
              Invite User
            </Button>
          )}
        </div>
      </div>

      {/* Role Permissions tab */}
      {activeMainTab === 'roles' && (
        <div className="grid grid-cols-3 gap-4">
          {ROLE_PERMISSIONS.map((perm) => {
            const Icon = perm.icon
            const menuCategories = (Object.keys(MENU_CATEGORY_LABEL) as MenuCategory[]).filter(
              (cat) => perm.menus.some((m) => m.category === cat)
            )
            return (
              <div
                key={perm.role}
                className="flex flex-col gap-3 rounded-[var(--card-radius)] border p-4"
                style={{ backgroundColor: perm.color.bg, borderColor: perm.color.border }}
              >
                <div className="flex items-center gap-2">
                  <Icon size={13} style={{ color: perm.color.text }} />
                  <span className="text-sm font-bold capitalize" style={{ color: perm.color.text }}>
                    {perm.role}
                  </span>
                  <span className="ml-auto text-[11px] text-[var(--color-text-muted)]">
                    {users.filter((u) => u.role === perm.role).length} users
                  </span>
                </div>

                <div className="h-px bg-[rgba(255,255,255,0.06)]" />

                <div className="flex flex-col gap-2">
                  <span className="text-[10px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-muted)]">
                    메뉴
                  </span>
                  {menuCategories.map((cat) => (
                    <div key={cat} className="flex flex-col gap-1">
                      <span className="text-[10px] font-semibold uppercase tracking-[0.04em] text-[var(--color-text-muted)] opacity-60">
                        {MENU_CATEGORY_LABEL[cat]}
                      </span>
                      <div className="flex flex-wrap gap-1">
                        {perm.menus.filter((m) => m.category === cat).map((menu) => {
                          const effectiveAccess = getMenuAccess(perm.role, menu.name, menu.access)
                          return (
                            <button
                              key={menu.name}
                              type="button"
                              title={`Toggle to ${effectiveAccess === 'Edit' ? 'View' : 'Edit'}`}
                              onClick={() => toggleMenuAccess(perm.role, menu.name, effectiveAccess)}
                              className={cn(
                                'flex cursor-pointer items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium transition-all duration-150 hover:opacity-80',
                                effectiveAccess === 'Edit'
                                  ? `${perm.color.accessEdit} border-transparent`
                                  : `${perm.color.accessView} border-dashed border-current opacity-70`
                              )}
                            >
                              {menu.name}
                              <span className={cn('rounded px-[3px] py-px text-[9px] font-bold', effectiveAccess === 'Edit' ? 'bg-[rgba(255,255,255,0.2)]' : 'bg-[rgba(255,255,255,0.12)]')}>
                                {effectiveAccess}
                              </span>
                            </button>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>

                <div className="h-px bg-[rgba(255,255,255,0.06)]" />

                <div className="flex flex-col gap-2">
                  <span className="text-[10px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-muted)]">
                    버튼
                  </span>
                  {perm.buttons.length === 0 ? (
                    <span className="text-[11px] text-[var(--color-text-muted)]">—</span>
                  ) : (
                    perm.buttons.map((group) => (
                      <div key={group.category} className="flex flex-col gap-0.5">
                        <span className="text-[10px] font-semibold uppercase tracking-[0.04em] text-[var(--color-text-muted)] opacity-60">
                          {MENU_CATEGORY_LABEL[group.category]}
                        </span>
                        {group.items.map((btn) => (
                          <div key={btn} className="flex items-center gap-1.5">
                            <span className="h-1 w-1 shrink-0 rounded-full" style={{ backgroundColor: perm.color.text }} />
                            <span className="text-[12px] text-[var(--color-text-secondary)]">{btn}</span>
                          </div>
                        ))}
                      </div>
                    ))
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Users tab */}
      {activeMainTab === 'users' && (
        <div>
          <div className="mb-4 flex items-center justify-between gap-3">
            <div className="flex gap-0 border-b border-[var(--color-border-default)]">
              {(['all', 'admin', 'devops', 'developer'] as ActiveRoleTab[]).map((tab) => {
                const count = tab === 'all' ? users.length : users.filter((u) => u.role === tab).length
                const active = activeRoleTab === tab
                return (
                  <button
                    key={tab}
                    type="button"
                    onClick={() => setActiveRoleTab(tab)}
                    className={cn(
                      '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-3.5 py-2 text-sm transition-all duration-150',
                      active ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]' : 'font-normal text-[var(--color-text-secondary)]'
                    )}
                  >
                    <span className="capitalize">{tab === 'all' ? 'All' : tab}</span>
                    <span className={cn('ml-1.5 rounded-full px-1.5 py-0.5 text-[11px] font-bold', active ? 'bg-[rgba(99,102,241,0.2)] text-[#a5b4fc]' : 'bg-[rgba(255,255,255,0.06)] text-[var(--color-text-muted)]')}>
                      {count}
                    </span>
                  </button>
                )
              })}
            </div>
            <Input
              placeholder="이름/이메일 검색..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className="max-w-[220px]"
            />
          </div>

          {isLoading ? (
            <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
              Loading users...
            </div>
          ) : (
            <DataTable columns={columns} data={filteredUsers} getRowKey={(row) => row.id} emptyMessage="사용자가 없습니다." />
          )}
        </div>
      )}

      {/* Invite modal */}
      <Modal
        open={inviteModal}
        onClose={() => {
          setInviteModal(false)
          reset(INVITE_USER_DEFAULTS)
        }}
        title="Invite User"
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setInviteModal(false)
                reset(INVITE_USER_DEFAULTS)
              }}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={inviteMember.isPending || isSubmitting}
              onClick={handleSubmit(handleInvite)}
              disabled={!isValid || isSubmitting}
              type="button"
            >
              <Mail size={13} />
              Send Invite
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          <Input
            label="이름"
            placeholder="User name"
            {...register('name')}
          />
          {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
          <Input
            label="이메일"
            type="email"
            placeholder="user@example.com"
            {...register('email')}
          />
          {errors.email && <span className="text-xs text-[#ef4444]">{errors.email.message}</span>}
          <div className="flex flex-col gap-1">
            <label htmlFor="invite-role" className="text-xs font-medium text-[var(--color-text-secondary)]">역할</label>
            <select
              id="invite-role"
              {...register('role')}
              className={selectClassName}
            >
              <option value="developer">Developer</option>
              <option value="devops">DevOps</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          {errors.role && <span className="text-xs text-[#ef4444]">{errors.role.message}</span>}
        </div>
      </Modal>

      <ConfirmDialog
        open={deactivateUserId !== null}
        onClose={() => setDeactivateUserId(null)}
        onConfirm={handleDeactivate}
        title="Deactivate User"
        description="선택한 사용자를 비활성화하면 로그인 및 배포 작업이 제한됩니다. 계속하시겠습니까?"
        confirmLabel="Deactivate"
        loading={deactivateUser.isPending}
      />
    </div>
  )
}
