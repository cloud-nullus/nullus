import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { GitBranch, Check, Copy, Link2, Mail, Plus, Search, Server, Shield, Trash2, Users, UserPlus, Loader2 } from 'lucide-react'
import type { ColumnDef } from '@tanstack/react-table'
import { useMembers, useInviteMember, useUpdateUserRole, useDeactivateUser, useOrganization, useSearchUser, useCreateInviteLink, useInviteLinks, useRevokeInviteLink } from '../api/admin-api'
import type { MemberRole, MemberStatus, InviteLink } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
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

const selectClassName = 'cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]'

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

const MOCK_INVITES: InviteLink[] = [
  { token: 'inv-1', role: 'devops', expiresAt: '2026-04-01T00:00:00Z', status: 'active' },
  { token: 'inv-2', role: 'developer', expiresAt: '2026-03-10T00:00:00Z', status: 'expired' },
]

const EXPIRY_OPTIONS = [
  { value: 1, label: '1 day' },
  { value: 3, label: '3 days' },
  { value: 7, label: '7 days' },
  { value: 30, label: '30 days' },
]

export function UserManagementPage() {
  const { data: orgData } = useOrganization()
  const ORG_ID = orgData?.id ?? ''
  const { data: membersData, isLoading } = useMembers(ORG_ID)
  const users = membersData?.items ?? []
  const inviteMember = useInviteMember(ORG_ID)
  const updateUserRole = useUpdateUserRole(ORG_ID)
  const deactivateUser = useDeactivateUser(ORG_ID)
  const createInviteLink = useCreateInviteLink(ORG_ID)
  const { data: inviteLinksData } = useInviteLinks(ORG_ID)
  const revokeInviteLink = useRevokeInviteLink(ORG_ID)

  const inviteLinks: InviteLink[] = (inviteLinksData?.items ?? []).length > 0
    ? inviteLinksData!.items
    : MOCK_INVITES

  const [activeMainTab, setActiveMainTab] = useState<'roles' | 'users'>('roles')
  const [activeRoleTab, setActiveRoleTab] = useState<ActiveRoleTab>('all')
  const [inviteModal, setInviteModal] = useState(false)
  const [inviteLinkModal, setInviteLinkModal] = useState(false)
  const [inviteLinkRole, setInviteLinkRole] = useState<MemberRole>('developer')
  const [inviteLinkExpiry, setInviteLinkExpiry] = useState(7)
  const [generatedLink, setGeneratedLink] = useState<string | null>(null)
  const [linkCopied, setLinkCopied] = useState(false)
  const [revokeToken, setRevokeToken] = useState<string | null>(null)
  const [menuAccessOverrides, setMenuAccessOverrides] = useState<
    Partial<Record<MemberRole, Record<string, 'View' | 'Edit'>>>
  >({})
  const [permissionSaved, setPermissionSaved] = useState(false)

  const hasRoleChanges = Object.values(menuAccessOverrides).some(
    (r) => Object.keys(r ?? {}).length > 0
  )

  useEffect(() => {
    const stored = localStorage.getItem('nullus-role-permission-overrides')
    if (stored) {
      try {
        setMenuAccessOverrides(JSON.parse(stored))
      } catch {
        // Ignore parse errors
      }
    }
  }, [])

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
    if (Object.keys(menuAccessOverrides).length > 0) {
      localStorage.setItem('nullus-role-permission-overrides', JSON.stringify(menuAccessOverrides))
    }
    setPermissionSaved(true)
    setTimeout(() => setPermissionSaved(false), 3000)
  }
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors, isValid, isSubmitting },
  } = useForm<InviteUserFormData>({
    resolver: zodResolver(inviteUserSchema),
    defaultValues: INVITE_USER_DEFAULTS,
    mode: 'onChange',
  })
  const [search, setSearch] = useState('')

  const watchedEmail = watch('email')
  const [debouncedEmail, setDebouncedEmail] = useState('')
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedEmail(watchedEmail), 300)
    return () => clearTimeout(timer)
  }, [watchedEmail])
  const { data: searchResult, isFetching: isSearching } = useSearchUser(debouncedEmail)
  const existingUser = searchResult?.found ? searchResult.user : null

  useEffect(() => {
    if (existingUser) {
      setValue('name', existingUser.name, { shouldValidate: true })
    }
  }, [existingUser, setValue])
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
        setDebouncedEmail('')
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

  const handleGenerateLink = () => {
    createInviteLink.mutate(
      { role: inviteLinkRole, expiresInDays: inviteLinkExpiry },
      {
        onSuccess: (data) => {
          setGeneratedLink(data.url ?? `${window.location.origin}/invite/${data.token}`)
        },
        onError: () => {
          const mockToken = `inv-${Date.now()}`
          setGeneratedLink(`${window.location.origin}/invite/${mockToken}`)
        },
      }
    )
  }

  const handleCopyLink = async () => {
    if (!generatedLink) return
    await navigator.clipboard.writeText(generatedLink)
    setLinkCopied(true)
    setTimeout(() => setLinkCopied(false), 2000)
  }

  const handleCloseInviteLinkModal = () => {
    setInviteLinkModal(false)
    setGeneratedLink(null)
    setLinkCopied(false)
    setInviteLinkRole('developer')
    setInviteLinkExpiry(7)
  }

  const handleRevokeInvite = () => {
    if (!revokeToken) return
    revokeInviteLink.mutate(revokeToken, {
      onSuccess: () => setRevokeToken(null),
    })
  }

  const isInviteExpired = (expiresAt: string) => new Date(expiresAt) < new Date()

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
          <NativeSelect
            value={selectedRole}
            onChange={(event) => handleRoleChange(row.original.id, event.target.value as MemberRole)}
            className={cn('cursor-pointer rounded-[5px] border-0 px-2.5 py-1 text-xs font-semibold', role.className)}
          >
            <option value="admin">Admin</option>
            <option value="devops">DevOps</option>
            <option value="developer">Developer</option>
          </NativeSelect>
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
        <div className="flex items-center gap-3 pb-2">
           {activeMainTab === 'roles' && (
             <>
               <Button
                 variant="primary"
                 size="sm"
                 type="button"
                 disabled={!hasRoleChanges}
                 onClick={handleSaveRoleChanges}
               >
                 Save Changes
               </Button>
               {permissionSaved && (
                 <span className="text-xs font-medium text-[#22c55e]">Changes saved</span>
               )}
             </>
           )}
           {activeMainTab === 'users' && (
             <div className="flex gap-2 pb-0">
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={() => setInviteLinkModal(true)}
              >
                <Link2 size={13} />
                Generate Invite Link
              </Button>
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
            </div>
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
            <div className="relative">
              <Search
                size={13}
                className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
              />
              <input
                placeholder="이름/이메일 검색..."
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
              />
            </div>
          </div>

          {isLoading ? (
            <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
              Loading users...
            </div>
          ) : (
            <DataTable columns={columns} data={filteredUsers} getRowKey={(row) => row.id} emptyMessage="사용자가 없습니다." />
          )}

          {/* Pending Invites */}
          <div className="mt-6">
            <h3 className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">Pending Invites</h3>
            <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
              <table className="w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]">
                    <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">Role</th>
                    <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">Expires At</th>
                    <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">Status</th>
                    <th className="px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {inviteLinks.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-6 text-center text-[var(--color-text-muted)]">
                        대기 중인 초대가 없습니다.
                      </td>
                    </tr>
                  ) : (
                    inviteLinks.map((invite) => {
                      const expired = isInviteExpired(invite.expiresAt)
                      return (
                        <tr key={invite.token} className="border-b border-[var(--color-border-default)] last:border-b-0">
                          <td className="px-4 py-2.5">
                            <span className={cn('rounded-md px-2.5 py-1 text-xs font-semibold capitalize', ROLE_BADGE[invite.role].className)}>
                              {invite.role}
                            </span>
                          </td>
                          <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">
                            {new Date(invite.expiresAt).toLocaleDateString('ko-KR', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                          </td>
                          <td className="px-4 py-2.5">
                            {expired ? (
                              <span className="rounded-md px-2.5 py-1 text-xs font-semibold bg-[rgba(100,116,139,0.15)] text-[#64748b]">Expired</span>
                            ) : (
                              <span className="rounded-md px-2.5 py-1 text-xs font-semibold bg-[rgba(34,197,94,0.15)] text-[#22c55e]">Active</span>
                            )}
                          </td>
                          <td className="px-4 py-2.5">
                            <Button
                              variant="danger"
                              size="sm"
                              type="button"
                              disabled={expired}
                              onClick={() => setRevokeToken(invite.token)}
                            >
                              <Trash2 size={12} />
                              Revoke
                            </Button>
                          </td>
                        </tr>
                      )
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Invite modal */}
      <Modal
        open={inviteModal}
        onClose={() => {
          setInviteModal(false)
          reset(INVITE_USER_DEFAULTS)
          setDebouncedEmail('')
        }}
        title={existingUser ? 'Add Existing Member' : 'Invite User'}
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
              {existingUser ? <><UserPlus size={13} /> Add Member</> : <><Mail size={13} /> Send Invite</>}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          <div className="relative">
            <Input
              label="이메일"
              type="email"
              placeholder="user@example.com"
              {...register('email')}
            />
            {isSearching && (
              <Loader2 size={14} className="absolute right-3 top-[34px] animate-spin text-[var(--color-text-secondary)]" />
            )}
          </div>
          {errors.email && <span className="text-xs text-[#ef4444]">{errors.email.message}</span>}
          {existingUser && (
            <div className="rounded-lg border border-[rgba(34,197,94,0.3)] bg-[rgba(34,197,94,0.08)] px-3 py-2 text-[13px] text-[#86efac]">
              ✓ 기존 사용자: {existingUser.name} ({existingUser.email})
            </div>
          )}
          {!existingUser && debouncedEmail.includes('@') && debouncedEmail.length > 3 && !isSearching && (
            <span className="text-xs text-[var(--color-text-secondary)]">새 사용자로 초대됩니다</span>
          )}
          {!existingUser && (
            <>
              <Input
                label="이름"
                placeholder="User name"
                {...register('name')}
              />
              {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
            </>
          )}
          <NativeSelect label="역할" {...register('role')} className={selectClassName}>
              <option value="developer">Developer</option>
              <option value="devops">DevOps</option>
              <option value="admin">Admin</option>
            </NativeSelect>
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

      <Modal
        open={inviteLinkModal}
        onClose={handleCloseInviteLinkModal}
        title="Generate Invite Link"
        footer={
          generatedLink ? (
            <Button variant="outline" size="sm" onClick={handleCloseInviteLinkModal}>
              Close
            </Button>
          ) : (
            <>
              <Button variant="outline" size="sm" onClick={handleCloseInviteLinkModal}>
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                type="button"
                loading={createInviteLink.isPending}
                onClick={handleGenerateLink}
              >
                <Link2 size={13} />
                Generate
              </Button>
            </>
          )
        }
      >
        {generatedLink ? (
          <div className="flex flex-col gap-3">
            <p className="m-0 text-sm text-[var(--color-text-secondary)]">
              초대 링크가 생성되었습니다. 링크를 복사하여 공유하세요.
            </p>
            <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2">
              <code className="flex-1 truncate text-xs text-[var(--color-text-primary)]">{generatedLink}</code>
              <Button variant="outline" size="sm" type="button" onClick={handleCopyLink}>
                {linkCopied ? <><Check size={13} className="text-[#22c55e]" /> Copied!</> : <><Copy size={13} /> Copy Link</>}
              </Button>
            </div>
            {linkCopied && (
              <span className="text-xs text-[#22c55e]">클립보드에 복사되었습니다</span>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3.5">
            <div className="flex flex-col gap-1">
              <label htmlFor="invite-link-role" className="text-xs font-medium text-[var(--color-text-secondary)]">역할</label>
              <select
                id="invite-link-role"
                value={inviteLinkRole}
                onChange={(e) => setInviteLinkRole(e.target.value as MemberRole)}
                className={selectClassName}
              >
                <option value="developer">Developer</option>
                <option value="devops">DevOps</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="invite-link-expiry" className="text-xs font-medium text-[var(--color-text-secondary)]">만료 기간</label>
              <select
                id="invite-link-expiry"
                value={inviteLinkExpiry}
                onChange={(e) => setInviteLinkExpiry(Number(e.target.value))}
                className={selectClassName}
              >
                {EXPIRY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={revokeToken !== null}
        onClose={() => setRevokeToken(null)}
        onConfirm={handleRevokeInvite}
        title="Revoke Invite Link"
        description="이 초대 링크를 취소하면 더 이상 사용할 수 없습니다. 계속하시겠습니까?"
        confirmLabel="Revoke"
        loading={revokeInviteLink.isPending}
      />
    </div>
  )
}
