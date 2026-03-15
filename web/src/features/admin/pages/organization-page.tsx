import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Settings, Plus, Trash2, Mail } from 'lucide-react'
import { useOrganization, useUpdateOrganization, useMembers, useInviteMember, useRemoveMember, useClusters } from '../api/admin-api'
import type { MemberRole, MemberStatus, InviteMemberRequest } from '../api/admin-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { cn } from '../../../lib/utils'

const STATUS_BADGE: Record<MemberStatus, { className: string }> = {
  active: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]' },
  pending: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]' },
  inactive: { className: 'bg-[rgba(100,116,139,0.15)] text-[#64748b]' },
}

const ROLE_BADGE: Record<MemberRole, { className: string }> = {
  admin: { className: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]' },
  devops: { className: 'bg-[rgba(99,102,241,0.15)] text-[#a5b4fc]' },
  developer: { className: 'bg-[rgba(34,197,94,0.15)] text-[#34d399]' },
}

const domainRegex = /^(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$/

const orgSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required').regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Slug must be lowercase alphanumeric with optional hyphens'),
  domain: z
    .string()
    .optional()
    .refine((value) => !value || domainRegex.test(value), 'Invalid domain'),
  status: z.enum(['active', 'inactive', 'suspended']),
})

const inviteSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
  role: z.enum(['admin', 'devops', 'developer']),
})

type OrgFormData = z.infer<typeof orgSchema>
type InviteFormData = z.infer<typeof inviteSchema>

export function OrganizationPage() {
  const { data: orgData, isLoading: orgLoading } = useOrganization()
  const org = orgData ?? { id: '', name: '', slug: '', domain: '', status: 'active' as const, clusterAccessScope: [] }
  const orgId = org.id
  const { data: membersData } = useMembers(orgId)
  const members = membersData?.items ?? []
  const { data: clustersData } = useClusters()
  const allClusters = (clustersData?.items ?? []).map((c) => c.name)

  const updateOrg = useUpdateOrganization()
  const inviteMember = useInviteMember(orgId)
  const removeMember = useRemoveMember(orgId)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isValid, isSubmitting },
  } = useForm<OrgFormData>({
    resolver: zodResolver(orgSchema),
    defaultValues: {
      name: org.name,
      slug: org.slug,
      domain: org.domain,
      status: org.status,
    },
    mode: 'onChange',
  })
  const {
    register: registerInvite,
    handleSubmit: handleInviteSubmit,
    reset: resetInvite,
    formState: { errors: inviteErrors, isValid: isInviteValid, isSubmitting: isInviteSubmitting },
  } = useForm<InviteFormData>({
    resolver: zodResolver(inviteSchema),
    defaultValues: { name: '', email: '', role: 'developer' },
    mode: 'onChange',
  })
  const [clusterAccessScope, setClusterAccessScope] = useState<string[]>([...(org.clusterAccessScope ?? [])])

  const [inviteModal, setInviteModal] = useState(false)
  const [removeMemberId, setRemoveMemberId] = useState<string | null>(null)

  useEffect(() => {
    reset({
      name: org.name,
      slug: org.slug,
      domain: org.domain,
      status: org.status,
    })
    setClusterAccessScope([...(org.clusterAccessScope ?? [])])
  }, [org.domain, org.name, org.slug, org.status, org.clusterAccessScope, reset])

  const handleSave = (data: OrgFormData) => {
    updateOrg.mutate({
      ...data,
      clusterAccessScope,
    })
  }

  const handleInvite = (data: InviteFormData) => {
    inviteMember.mutate(data as InviteMemberRequest, {
      onSuccess: () => {
        setInviteModal(false)
        resetInvite({ name: '', email: '', role: 'developer' })
      },
    })
  }

  const handleScopeToggle = (cluster: string) => {
    setClusterAccessScope((current) =>
      current.includes(cluster)
        ? current.filter((c) => c !== cluster)
        : [...current, cluster]
    )
  }

  const handleConfirmRemove = () => {
    if (!removeMemberId) return
    removeMember.mutate(removeMemberId, {
      onSuccess: () => {
        setRemoveMemberId(null)
      },
    })
  }

  const selectClassName = 'rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'
  const tdClassName = 'border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]'

  if (orgLoading || !orgData) {
    return <div className="flex h-[200px] items-center justify-center text-[var(--color-text-secondary)]">Loading...</div>
  }

  return (
    <div>
      <Breadcrumb items={[{ label: 'Organization' }]} />

      {/* Page header */}
      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]">
          <Settings size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            Organization
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            조직 정보 및 멤버를 관리합니다.
          </p>
        </div>
      </div>

      <div className="mb-6 grid grid-cols-2 gap-5">
        {/* Org info form */}
        <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
          <h2 className="mb-4 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
            조직 정보
          </h2>
          <div className="flex flex-col gap-3">
            <Input
              label="조직 이름"
              {...register('name')}
            />
            {errors.name && <span className="text-xs text-[#ef4444]">{errors.name.message}</span>}
            <Input
              label="슬러그 (Slug)"
              {...register('slug')}
            />
            {errors.slug && <span className="text-xs text-[#ef4444]">{errors.slug.message}</span>}
            <Input
              label="도메인"
              {...register('domain')}
            />
            {errors.domain && <span className="text-xs text-[#ef4444]">{errors.domain.message}</span>}
            <div className="flex flex-col gap-1">
              <label htmlFor="organization-status" className="text-xs font-medium text-[var(--color-text-secondary)]">상태</label>
              <select
                id="organization-status"
                {...register('status')}
                className={selectClassName}
              >
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
                <option value="suspended">Suspended</option>
              </select>
            </div>
            <Button
              variant="primary"
              size="sm"
              loading={updateOrg.isPending || isSubmitting}
              onClick={handleSubmit(handleSave)}
              disabled={!isValid || isSubmitting}
              type="button"
            >
              저장
            </Button>
          </div>
        </div>

        {/* Cluster access scope */}
        <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
          <h2 className="mb-4 mt-0 text-sm font-bold text-[var(--color-text-primary)]">
            클러스터 접근 범위
          </h2>
          <p className="mb-3.5 mt-0 text-[13px] text-[var(--color-text-secondary)]">
            이 조직에서 접근 가능한 클러스터를 선택하세요.
          </p>
          <div className="flex flex-col gap-2">
            {allClusters.map((cluster) => {
                const checked = clusterAccessScope.includes(cluster)
              return (
                <label
                  key={cluster}
                  className={cn(
                    'flex cursor-pointer items-center gap-2.5 rounded-md border px-2.5 py-2 transition-all duration-150',
                    checked
                      ? 'border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.08)]'
                      : 'border-transparent bg-transparent'
                  )}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => handleScopeToggle(cluster)}
                    className="h-[15px] w-[15px] accent-[#6366f1]"
                  />
                  <span className={cn('text-sm', checked ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                    {cluster}
                  </span>
                </label>
              )
            })}
          </div>
          <div className="mt-4 flex justify-end">
            <Button
              variant="primary"
              size="sm"
              loading={updateOrg.isPending}
              onClick={handleSubmit(handleSave)}
              type="button"
            >
              클러스터 접근 범위 저장
            </Button>
          </div>
        </div>
      </div>

      {/* Members table */}
      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-[18px] py-4">
          <h2 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">
            멤버 관리
          </h2>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              resetInvite({ name: '', email: '', role: 'developer' })
              setInviteModal(true)
            }}
            type="button"
          >
            <Plus size={13} />
            Invite Member
          </Button>
        </div>
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              {['이름', '이메일', '역할', '상태', 'Actions'].map((h) => (
                <th
                  key={h}
                  className="px-3.5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {members.map((m) => (
              <tr key={m.id}>
                <td className={tdClassName}>
                  <span className="font-semibold">{m.name}</span>
                </td>
                <td className={cn(tdClassName, 'text-[var(--color-text-secondary)]')}>{m.email}</td>
                <td className={tdClassName}>
                  <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', ROLE_BADGE[m.role].className)}>
                    {m.role}
                  </span>
                </td>
                <td className={tdClassName}>
                  <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', STATUS_BADGE[m.status].className)}>
                    {m.status}
                  </span>
                </td>
                <td className={tdClassName}>
                  <Button
                    variant="danger"
                    size="sm"
                    loading={removeMember.isPending}
                    onClick={() => setRemoveMemberId(m.id)}
                    type="button"
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
        onClose={() => {
          setInviteModal(false)
          resetInvite({ name: '', email: '', role: 'developer' })
        }}
        title="Invite Member"
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setInviteModal(false)
                resetInvite({ name: '', email: '', role: 'developer' })
              }}
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={inviteMember.isPending || isInviteSubmitting}
              onClick={handleInviteSubmit(handleInvite)}
              disabled={!isInviteValid || isInviteSubmitting}
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
              placeholder="홍길동"
              {...registerInvite('name')}
            />
            {inviteErrors.name && <span className="text-xs text-[#ef4444]">{inviteErrors.name.message}</span>}
            <Input
              label="이메일"
              type="email"
              placeholder="member@example.com"
              {...registerInvite('email')}
            />
            {inviteErrors.email && <span className="text-xs text-[#ef4444]">{inviteErrors.email.message}</span>}
            <div className="flex flex-col gap-1">
              <label htmlFor="organization-invite-role" className="text-xs font-medium text-[var(--color-text-secondary)]">역할</label>
              <select
                id="organization-invite-role"
                {...registerInvite('role')}
                className={selectClassName}
            >
              <option value="developer">Developer</option>
              <option value="devops">DevOps</option>
              <option value="admin">Admin</option>
              </select>
            </div>
            {inviteErrors.role && <span className="text-xs text-[#ef4444]">{inviteErrors.role.message}</span>}
          </div>
      </Modal>

      <ConfirmDialog
        open={removeMemberId !== null}
        onClose={() => setRemoveMemberId(null)}
        onConfirm={handleConfirmRemove}
        title="Remove Member"
        description="선택한 멤버를 조직에서 제거합니다. 계속하시겠습니까?"
        confirmLabel="Remove"
        loading={removeMember.isPending}
      />
    </div>
  )
}
