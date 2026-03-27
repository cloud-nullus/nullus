import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Mail, Plus, Settings, Trash2 } from 'lucide-react'
import {
  useClusters,
  useCreateOrganization,
  useInviteMember,
  useMembers,
  useOrganization,
  useRemoveMember,
  useUpdateOrganization,
} from '../api/admin-api'
import type { ClusterStatus, CreateOrgRequest, InviteMemberRequest, MemberRole, MemberStatus, Organization } from '../api/admin-api'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Input } from '../../../components/ui/input'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { Modal } from '../../../components/ui/modal'
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

const CLUSTER_STATUS_BADGE: Record<ClusterStatus, { className: string; label: string }> = {
  connected: { className: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]', label: 'Connected' },
  pending: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Pending' },
  error: { className: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Error' },
  inactive: { className: 'bg-[rgba(100,116,139,0.15)] text-[#64748b]', label: 'Inactive' },
  unreachable: { className: 'bg-[rgba(245,158,11,0.15)] text-[#f59e0b]', label: 'Unreachable' },
  auth_failed: { className: 'bg-[rgba(239,68,68,0.15)] text-[#ef4444]', label: 'Auth Failed' },
}

const domainRegex = /^(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$/

const orgSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required').regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Slug must be lowercase alphanumeric with optional hyphens'),
  domain: z.string().optional().refine((value) => !value || domainRegex.test(value), 'Invalid domain'),
  status: z.enum(['active', 'inactive', 'suspended']),
})

const newOrgSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required').regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Slug must be lowercase alphanumeric with optional hyphens'),
  domain: z.string().optional().refine((value) => !value || domainRegex.test(value), 'Invalid domain'),
})

type NewOrgFormData = z.infer<typeof newOrgSchema>

const inviteSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
  role: z.enum(['admin', 'devops', 'developer']),
})

type OrgFormData = z.infer<typeof orgSchema>
type InviteFormData = z.infer<typeof inviteSchema>

const selectClassName = 'rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]'
const tdClassName = 'border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]'

export function OrganizationPage() {
  const { data: orgData, isLoading: orgLoading } = useOrganization()
  const [localOrgs, setLocalOrgs] = useState<Organization[]>([])
  const organizations = useMemo(() => {
    const fromApi = orgData ? [orgData] : []
    return [...fromApi, ...localOrgs]
  }, [orgData, localOrgs])
  const [selectedOrgId, setSelectedOrgId] = useState<string | null>(null)

  useEffect(() => {
    if (!selectedOrgId && organizations.length > 0) {
      setSelectedOrgId(organizations[0].id)
    }
  }, [organizations, selectedOrgId])

  const selectedOrg = organizations.find((org) => org.id === selectedOrgId) ?? organizations[0] ?? null
  const orgId = selectedOrg?.id ?? ''

  const { data: membersData } = useMembers(orgId)
  const members = membersData?.items ?? []
  const { data: clustersData } = useClusters()
  const allClusters = clustersData?.items ?? []

  const createOrg = useCreateOrganization()
  const updateOrg = useUpdateOrganization()
  const inviteMember = useInviteMember(orgId)
  const removeMember = useRemoveMember(orgId)

  const [clusterAccessScope, setClusterAccessScope] = useState<string[]>([])
  const [inviteModal, setInviteModal] = useState(false)
  const [newOrgModal, setNewOrgModal] = useState(false)
  const [removeMemberId, setRemoveMemberId] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isValid, isSubmitting },
  } = useForm<OrgFormData>({
    resolver: zodResolver(orgSchema),
    defaultValues: {
      name: selectedOrg?.name ?? '',
      slug: selectedOrg?.slug ?? '',
      domain: selectedOrg?.domain ?? '',
      status: selectedOrg?.status ?? 'active',
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

  const {
    register: registerNewOrg,
    handleSubmit: handleNewOrgSubmit,
    reset: resetNewOrg,
    formState: { errors: newOrgErrors, isValid: isNewOrgValid, isSubmitting: isNewOrgSubmitting },
  } = useForm<NewOrgFormData>({
    resolver: zodResolver(newOrgSchema),
    defaultValues: { name: '', slug: '', domain: '' },
    mode: 'onChange',
  })

  useEffect(() => {
    if (!selectedOrg) {
      return
    }

    reset({
      name: selectedOrg.name,
      slug: selectedOrg.slug,
      domain: selectedOrg.domain,
      status: selectedOrg.status,
    })
    setClusterAccessScope([...(selectedOrg.clusterAccessScope ?? [])])
  }, [selectedOrg, reset])

  const handleSave = (data: OrgFormData) => {
    if (!selectedOrg) {
      return
    }

    updateOrg.mutate({
      ...data,
      clusterAccessScope,
    }, {
      onError: () => {
        // Mock mode fallback: update local org
        setLocalOrgs(prev => prev.map(org => 
          org.id === selectedOrg.id 
            ? { ...org, ...data, clusterAccessScope }
            : org
        ))
      },
    })
  }

  const handleScopeToggle = (clusterName: string) => {
    setClusterAccessScope((current) =>
      current.includes(clusterName)
        ? current.filter((name) => name !== clusterName)
        : [...current, clusterName]
    )
  }

  const handleCreateOrg = (data: NewOrgFormData) => {
    const payload: CreateOrgRequest = {
      name: data.name,
      slug: data.slug,
      domain: data.domain || undefined,
      status: 'active',
    }
    createOrg.mutate(payload, {
      onSuccess: () => {
        setNewOrgModal(false)
        resetNewOrg()
      },
      onError: () => {
        // Mock mode fallback: add to local state
        const mockOrg: Organization = {
          id: `org-${Date.now()}`,
          name: payload.name,
          slug: payload.slug,
          domain: payload.domain || '',
          status: 'active',
          clusterAccessScope: [],
          createdAt: new Date().toISOString(),
        }
        setLocalOrgs(prev => [...prev, mockOrg])
        setNewOrgModal(false)
        resetNewOrg()
        setSelectedOrgId(mockOrg.id)
      },
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

  const handleConfirmRemove = () => {
    if (!removeMemberId) {
      return
    }

    removeMember.mutate(removeMemberId, {
      onSuccess: () => {
        setRemoveMemberId(null)
      },
    })
  }

  if (orgLoading) {
    return <div className="flex h-[200px] items-center justify-center text-[var(--color-text-secondary)]">Loading...</div>
  }

  return (
    <div>
      <Breadcrumb items={[{ label: 'Organization' }]} />

      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]">
            <Settings size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">Organization</h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">조직 설정, 접근 범위, 멤버를 통합 관리합니다.</p>
          </div>
        </div>
        <Button
          variant="primary"
          size="md"
          type="button"
          onClick={() => {
            resetNewOrg()
            setNewOrgModal(true)
          }}
        >
          <Plus size={15} />
          New Organization
        </Button>
      </div>

      <div className="h-[700px]">
        <ListDetailPanel
          listWidth={280}
          listContent={
            <>
              <div className="border-b border-[var(--color-border-default)] px-4 py-3 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                Organizations ({organizations.length})
              </div>
              {organizations.map((org) => {
                const selected = selectedOrg?.id === org.id
                return (
                  <button
                    key={org.id}
                    type="button"
                    onClick={() => setSelectedOrgId(org.id)}
                    className={cn(
                      'w-full cursor-pointer border-0 border-b border-l-[3px] border-b-[var(--color-border-default)] px-4 py-3.5 text-left transition-all duration-150',
                      selected
                        ? 'border-l-[#6366f1] bg-[rgba(99,102,241,0.1)]'
                        : 'border-l-transparent bg-transparent'
                    )}
                  >
                    <div className="mb-1 flex items-center justify-between">
                      <span className={cn('text-sm font-semibold', selected ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                        {org.name}
                      </span>
                      <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', org.status === 'active' ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]' : 'bg-[rgba(100,116,139,0.15)] text-[#64748b]')}>
                        {org.status}
                      </span>
                    </div>
                    <div className="text-xs text-[var(--color-text-secondary)]">{org.slug}</div>
                  </button>
                )
              })}
            </>
          }
          detailContent={
            selectedOrg ? (
              <div className="min-w-0 p-4">
                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <div className="mb-4 flex items-center justify-between">
                    <h2 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Organization Detail</h2>
                    <Button
                      variant="primary"
                      size="sm"
                      loading={updateOrg.isPending || isSubmitting}
                      onClick={handleSubmit(handleSave)}
                      disabled={!isValid || isSubmitting}
                      type="button"
                    >
                      Save Changes
                    </Button>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <Input label="Organization Name" {...register('name')} />
                    <Input label="Slug" {...register('slug')} />
                    <Input label="Domain" {...register('domain')} />
                    <NativeSelect label="Status" {...register('status')} className={selectClassName}>
                        <option value="active">Active</option>
                        <option value="inactive">Inactive</option>
                        <option value="suspended">Suspended</option>
                      </NativeSelect>
                  </div>
                  {(errors.name || errors.slug || errors.domain) && (
                    <div className="mt-2 text-xs text-[#ef4444]">
                      {errors.name?.message ?? errors.slug?.message ?? errors.domain?.message}
                    </div>
                  )}
                </div>

                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">Cluster Access Scope</h3>
                  <div className="grid grid-cols-2 gap-2.5">
                    {allClusters.map((cluster) => {
                      const checked = clusterAccessScope.includes(cluster.name)
                      const statusBadge = CLUSTER_STATUS_BADGE[cluster.status]

                      return (
                        <label
                          key={cluster.id}
                          className={cn(
                            'flex cursor-pointer items-center gap-2.5 rounded-md border px-2.5 py-2 transition-all duration-150',
                            checked
                              ? 'border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.08)]'
                              : 'border-[var(--color-border-default)] bg-transparent'
                          )}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => handleScopeToggle(cluster.name)}
                            className="h-[15px] w-[15px] accent-[#6366f1]"
                          />
                          <div className="flex flex-1 items-center justify-between gap-2">
                            <div>
                              <div className={cn('text-sm font-semibold', checked ? 'text-[#a5b4fc]' : 'text-[var(--color-text-primary)]')}>
                                {cluster.name}
                              </div>
                              <div className="text-xs text-[var(--color-text-secondary)]">{cluster.type.toUpperCase()}</div>
                            </div>
                            <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', statusBadge.className)}>
                              {statusBadge.label}
                            </span>
                          </div>
                        </label>
                      )
                    })}
                  </div>
                </div>

                <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
                  <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-[18px] py-4">
                    <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">Member Management</h3>
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
                        {['Name', 'Email', 'Role', 'Status', 'Actions'].map((header) => (
                          <th key={header} className="px-3.5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                            {header}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {members.map((member) => (
                        <tr key={member.id}>
                          <td className={tdClassName}>
                            <div className="flex items-center gap-2.5">
                              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[linear-gradient(135deg,#6366f1,#8b5cf6)] text-xs font-bold text-white">
                                {member.name.slice(0, 1).toUpperCase()}
                              </span>
                              <span className="font-semibold">{member.name}</span>
                            </div>
                          </td>
                          <td className={cn(tdClassName, 'text-[var(--color-text-secondary)]')}>{member.email}</td>
                          <td className={tdClassName}>
                            <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', ROLE_BADGE[member.role].className)}>
                              {member.role}
                            </span>
                          </td>
                          <td className={tdClassName}>
                            <span className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', STATUS_BADGE[member.status].className)}>
                              {member.status}
                            </span>
                          </td>
                          <td className={tdClassName}>
                            {member.role === 'admin' ? (
                              <span className="text-xs text-[var(--color-text-muted)]">Owner</span>
                            ) : (
                              <Button
                                variant="danger"
                                size="sm"
                                loading={removeMember.isPending}
                                onClick={() => setRemoveMemberId(member.id)}
                                type="button"
                              >
                                <Trash2 size={13} />
                                Remove
                              </Button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : null
          }
          emptyDetailMessage="Select an organization to view details"
        />
      </div>

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
          <Input label="Name" placeholder="홍길동" {...registerInvite('name')} />
          {inviteErrors.name && <span className="text-xs text-[#ef4444]">{inviteErrors.name.message}</span>}

          <Input label="Email" type="email" placeholder="member@example.com" {...registerInvite('email')} />
          {inviteErrors.email && <span className="text-xs text-[#ef4444]">{inviteErrors.email.message}</span>}

          <NativeSelect label="Role" {...registerInvite('role')} className={selectClassName}>
              <option value="developer">Developer</option>
              <option value="devops">DevOps</option>
              <option value="admin">Admin</option>
            </NativeSelect>
          {inviteErrors.role && <span className="text-xs text-[#ef4444]">{inviteErrors.role.message}</span>}
        </div>
      </Modal>

      <Modal
        open={newOrgModal}
        onClose={() => {
          setNewOrgModal(false)
          resetNewOrg()
        }}
        title="New Organization"
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => {
                setNewOrgModal(false)
                resetNewOrg()
              }}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              loading={createOrg.isPending || isNewOrgSubmitting}
              onClick={handleNewOrgSubmit(handleCreateOrg)}
              disabled={!isNewOrgValid || isNewOrgSubmitting}
            >
              <Plus size={13} />
              Create Organization
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <Input label="Organization Name" placeholder="예: Acme Corp" {...registerNewOrg('name')} />
          {newOrgErrors.name && <span className="text-xs text-[#ef4444]">{newOrgErrors.name.message}</span>}
          <Input
            label="Slug"
            placeholder="예: acme-corp"
            {...registerNewOrg('slug')}
          />
          {newOrgErrors.slug && <span className="text-xs text-[#ef4444]">{newOrgErrors.slug.message}</span>}
          <Input label="Domain (optional)" placeholder="예: acme.com" {...registerNewOrg('domain')} />
          {newOrgErrors.domain && <span className="text-xs text-[#ef4444]">{newOrgErrors.domain.message}</span>}
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
