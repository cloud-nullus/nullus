import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Users, Plus, Mail } from 'lucide-react'
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

  const [inviteModal, setInviteModal] = useState(false)
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
    if (!query) return true
    return user.name.toLowerCase().includes(query) || user.email.toLowerCase().includes(query)
  })

  const handleInvite = (data: InviteUserFormData) => {
    inviteMember.mutate({ email: data.email, role: data.role }, {
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
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]">
            <Users size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              User Management
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              사용자 목록 및 역할 관리
            </p>
          </div>
        </div>
        <Button
          variant="primary"
          size="md"
          onClick={() => {
            reset(INVITE_USER_DEFAULTS)
            setInviteModal(true)
          }}
          type="button"
        >
          <Plus size={15} />
          Invite User
        </Button>
      </div>

      <div className="mb-4 max-w-[320px]">
        <Input
          placeholder="이름/이메일 검색..."
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {isLoading ? (
        <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
          Loading users...
        </div>
      ) : (
        <DataTable columns={columns} data={filteredUsers} getRowKey={(row) => row.id} emptyMessage="사용자가 없습니다." />
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
