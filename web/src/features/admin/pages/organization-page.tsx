import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Mail, Plus, Settings, Trash2 } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
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
import { Button } from '../../../components/ui/button'
import { Select } from '../../../components/ui/select'
import { ConfirmDialog } from '../../../components/shared/confirm-dialog'
import { Input } from '../../../components/ui/input'
import { ListDetailPanel } from '../../../components/shared/list-detail-panel'
import { Modal } from '../../../components/ui/modal'
import { cn } from '../../../lib/utils'
import { PageHeader } from '../../../components/layout/page-header'
import { Checkbox } from '../../../components/ui/checkbox'
import { tableHeadRowClass, thClass } from '../../../components/shared/table-chrome'
import { Badge } from '../../../components/ui/badge'

const STATUS_BADGE: Record<MemberStatus, { className: string }> = {
  active: { className: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]' },
  pending: { className: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]' },
  inactive: { className: 'bg-[color-mix(in_srgb,_var(--color-text-muted)_15%,_transparent)] text-[var(--color-text-muted)]' },
}

const ROLE_BADGE: Record<MemberRole, { className: string }> = {
  admin: { className: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]' },
  devops: { className: 'bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] text-[var(--color-primary)]' },
  developer: { className: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]' },
}

const CLUSTER_STATUS_BADGE: Record<ClusterStatus, { className: string }> = {
  connected: { className: 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]' },
  pending: { className: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]' },
  error: { className: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]' },
  inactive: { className: 'bg-[color-mix(in_srgb,_var(--color-text-muted)_15%,_transparent)] text-[var(--color-text-muted)]' },
  unreachable: { className: 'bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]' },
  auth_failed: { className: 'bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]' },
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

const tdClassName = 'border-t border-[var(--color-border-default)] px-3.5 py-3 text-sm text-[var(--color-text-primary)]'

function getMemberStatusLabel(t: TFunction, status: MemberStatus) {
  if (status === 'active') return t('organizationPage.memberStatus.active', 'Active')
  if (status === 'pending') return t('organizationPage.memberStatus.pending', 'Pending')
  return t('organizationPage.memberStatus.inactive', 'Inactive')
}

function getMemberRoleLabel(t: TFunction, role: MemberRole) {
  if (role === 'admin') return t('organizationPage.role.admin', 'Admin')
  if (role === 'devops') return t('organizationPage.role.devops', 'DevOps')
  return t('organizationPage.role.developer', 'Developer')
}

function getClusterStatusLabel(t: TFunction, status: ClusterStatus) {
  if (status === 'connected') return t('organizationPage.clusterStatus.connected', 'Connected')
  if (status === 'pending') return t('organizationPage.clusterStatus.pending', 'Pending')
  if (status === 'error') return t('organizationPage.clusterStatus.error', 'Error')
  if (status === 'inactive') return t('organizationPage.clusterStatus.inactive', 'Inactive')
  if (status === 'unreachable') return t('organizationPage.clusterStatus.unreachable', 'Unreachable')
  return t('organizationPage.clusterStatus.authFailed', 'Auth Failed')
}

function getOrganizationStatusLabel(
  t: TFunction,
  status: Organization['status']
) {
  if (status === 'active') return t('organizationPage.orgStatus.active', 'Active')
  if (status === 'inactive') return t('organizationPage.orgStatus.inactive', 'Inactive')
  return t('organizationPage.orgStatus.suspended', 'Suspended')
}

export function OrganizationPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
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
    control,
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
    control: controlInvite,
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
      <PageHeader
        breadcrumb={[{ label: t('sidebar.organization', 'Organization') }]}
        icon={<Settings {...iconProps('sm')} />}
        tone="accent"
        title={t('sidebar.organization', 'Organization')}
        subtitle={t('organizationPage.description', 'Manage organization settings, access scope, and members in one place.')}
        actions={
          <Button
            variant="primary"
            size="md"
            type="button"
            onClick={() => {
              resetNewOrg()
              setNewOrgModal(true)
            }}
          >
            <Plus {...iconProps('sm')} />
            {t('organizationPage.actions.newOrganization', 'New Organization')}
          </Button>
        }
      />

      <div className="h-[860px]">
        <ListDetailPanel
          listWidth={280}
          listContent={
            <>
              <div className="border-b border-[var(--color-border-default)] px-4 py-3 text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
                {t('organizationPage.list.organizations', 'Organizations')} ({organizations.length})
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
                        ? 'border-l-[var(--color-primary)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]'
                        : 'border-l-transparent bg-transparent'
                    )}
                  >
                    <div className="mb-1 flex items-center justify-between">
                      <span className={cn('text-sm font-semibold', selected ? 'text-[var(--color-primary)]' : 'text-[var(--color-text-primary)]')}>
                        {org.name}
                      </span>
                      <span className={cn('rounded-[5px] px-2 py-0.5 text-[11px] font-semibold', org.status === 'active' ? 'bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]' : 'bg-[color-mix(in_srgb,_var(--color-text-muted)_15%,_transparent)] text-[var(--color-text-muted)]')}>
                        {getOrganizationStatusLabel(t, org.status)}
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
                    <h2 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">{t('organizationPage.detail.organizationDetail', 'Organization Detail')}</h2>
                    <Button
                      variant="primary"
                      size="sm"
                      loading={updateOrg.isPending || isSubmitting}
                      onClick={handleSubmit(handleSave)}
                      disabled={!isValid || isSubmitting}
                      type="button"
                    >
                      {t('organizationPage.actions.saveChanges', 'Save Changes')}
                    </Button>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <Input label={t('organizationPage.form.organizationName', 'Organization Name')} {...register('name')} />
                    <Input label={t('organizationPage.form.slug', 'Slug')} {...register('slug')} />
                    <Input label={t('organizationPage.form.domain', 'Domain')} {...register('domain')} />
                    {/* register() 로는 못 쓴다 — Select 의 값은 React 상태라 reset() 이 안 닿는다. */}
                    <Controller
                      name="status"
                      control={control}
                      render={({ field }) => (
                        <Select label={t('organizationPage.form.status', 'Status')} {...field}>
                          <option value="active">{t('organizationPage.orgStatus.active', 'Active')}</option>
                          <option value="inactive">{t('organizationPage.orgStatus.inactive', 'Inactive')}</option>
                          <option value="suspended">{t('organizationPage.orgStatus.suspended', 'Suspended')}</option>
                        </Select>
                      )}
                    />
                  </div>
                  {(errors.name || errors.slug || errors.domain) && (
                    <div className="mt-2 text-xs text-[var(--color-error)]">
                      {errors.name?.message ?? errors.slug?.message ?? errors.domain?.message}
                    </div>
                  )}
                </div>

                <div className="mb-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
                  <h3 className="mb-3 mt-0 text-sm font-bold text-[var(--color-text-primary)]">{t('organizationPage.detail.clusterAccessScope', 'Cluster Access Scope')}</h3>
                  <div className="grid grid-cols-1 gap-2.5 lg:grid-cols-2">
                    {allClusters.map((cluster) => {
                      const checked = clusterAccessScope.includes(cluster.name)
                      const statusBadge = CLUSTER_STATUS_BADGE[cluster.status]

                      return (
                        <label
                          key={cluster.id}
                          className={cn(
                            'flex cursor-pointer items-start gap-2.5 rounded-md border px-2.5 py-2 transition-all duration-150 sm:items-center',
                            checked
                              ? 'border-[color-mix(in_srgb,_var(--color-primary)_30%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_8%,_transparent)]'
                              : 'border-[var(--color-border-default)] bg-transparent'
                          )}
                        >
                          <Checkbox
                            checked={checked}
                            onChange={() => handleScopeToggle(cluster.name)}
                          />
                          <div className="flex min-w-0 flex-1 flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                            <div className="min-w-0">
                              <div
                                className={cn('truncate text-sm font-semibold', checked ? 'text-[var(--color-primary)]' : 'text-[var(--color-text-primary)]')}
                                title={cluster.name}
                              >
                                {cluster.name}
                              </div>
                              <div className="truncate text-xs text-[var(--color-text-secondary)]">{cluster.type.toUpperCase()}</div>
                            </div>
                            <span className={cn('shrink-0 self-start whitespace-nowrap rounded-[5px] px-2 py-0.5 text-[11px] font-semibold sm:self-auto', statusBadge.className)}>
                              {getClusterStatusLabel(t, cluster.status)}
                            </span>
                          </div>
                        </label>
                      )
                    })}
                  </div>
                </div>

                <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--color-border-default)] px-[18px] py-4">
                    <h3 className="m-0 text-sm font-bold text-[var(--color-text-primary)]">{t('organizationPage.detail.memberManagement', 'Member Management')}</h3>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => navigate('/admin/users')}
                        type="button"
                      >
                        <Plus {...iconProps('xs')} />
                        {t('organizationPage.actions.addUser', 'Add User')}
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          resetInvite({ name: '', email: '', role: 'developer' })
                          setInviteModal(true)
                        }}
                        type="button"
                      >
                        <Plus {...iconProps('xs')} />
                        {t('organizationPage.actions.inviteMember', 'Invite Member')}
                      </Button>
                    </div>
                  </div>

                  <div className="overflow-x-auto">
                    <table className="w-full min-w-[700px] border-collapse">
                      <thead>
                        <tr className={tableHeadRowClass}>
                          {[
                            t('organizationPage.table.name', 'Name'),
                            t('organizationPage.table.email', 'Email'),
                            t('organizationPage.table.role', 'Role'),
                            t('organizationPage.table.status', 'Status'),
                            t('organizationPage.table.actions', 'Actions'),
                          ].map((header) => (
                            <th key={header} className={cn(thClass)}>
                              {header}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {members.map((member) => (
                          <tr key={member.id}>
                            <td className={tdClassName}>
                              <div className="flex min-w-0 items-center gap-2.5">
                                <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[linear-gradient(135deg,var(--color-primary),var(--color-accent-alt))] text-xs font-bold text-white">
                                  {member.name.slice(0, 1).toUpperCase()}
                                </span>
                                <span className="truncate font-semibold" title={member.name}>{member.name}</span>
                              </div>
                            </td>
                            <td className={cn(tdClassName, 'text-[var(--color-text-secondary)]')}>{member.email}</td>
                            <td className={tdClassName}>
                              <Badge className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', ROLE_BADGE[member.role].className)}>
                                {getMemberRoleLabel(t, member.role)}
                              </Badge>
                            </td>
                            <td className={tdClassName}>
                              <Badge className={cn('rounded-[5px] px-2 py-0.5 text-xs font-semibold', STATUS_BADGE[member.status].className)}>
                                {getMemberStatusLabel(t, member.status)}
                              </Badge>
                            </td>
                            <td className={tdClassName}>
                              {member.role === 'admin' ? (
                                <span className="text-xs text-[var(--color-text-muted)]">{t('organizationPage.table.owner', 'Owner')}</span>
                              ) : (
                                <Button
                                  variant="danger"
                                  size="sm"
                                  loading={removeMember.isPending}
                                  onClick={() => setRemoveMemberId(member.id)}
                                  type="button"
                                >
                                  <Trash2 {...iconProps('xs')} />
                                  {t('organizationPage.actions.remove', 'Remove')}
                                </Button>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            ) : null
          }
          emptyDetailMessage={t('organizationPage.detail.selectOrganization', 'Select an organization to view details')}
        />
      </div>

      <Modal
        open={inviteModal}
        onClose={() => {
          setInviteModal(false)
          resetInvite({ name: '', email: '', role: 'developer' })
        }}
        title={t('organizationPage.modal.inviteMember', 'Invite Member')}
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
              {t('organizationPage.actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              loading={inviteMember.isPending || isInviteSubmitting}
              onClick={handleInviteSubmit(handleInvite)}
              disabled={!isInviteValid || isInviteSubmitting}
              type="button"
            >
              <Mail {...iconProps('xs')} />
              {t('organizationPage.actions.sendInvite', 'Send Invite')}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3.5">
          <Input label={t('organizationPage.form.name', 'Name')} placeholder={t('organizationPage.form.namePlaceholder', 'e.g. Hong Gil-dong')} {...registerInvite('name')} />
          {inviteErrors.name && <span className="text-xs text-[var(--color-error)]">{inviteErrors.name.message}</span>}

          <Input label={t('organizationPage.form.email', 'Email')} type="email" placeholder="member@example.com" {...registerInvite('email')} />
          {inviteErrors.email && <span className="text-xs text-[var(--color-error)]">{inviteErrors.email.message}</span>}

          <Controller
            name="role"
            control={controlInvite}
            render={({ field }) => (
              <Select label={t('organizationPage.form.role', 'Role')} {...field}>
                <option value="developer">{t('organizationPage.role.developer', 'Developer')}</option>
                <option value="devops">{t('organizationPage.role.devops', 'DevOps')}</option>
                <option value="admin">{t('organizationPage.role.admin', 'Admin')}</option>
              </Select>
            )}
          />
          {inviteErrors.role && <span className="text-xs text-[var(--color-error)]">{inviteErrors.role.message}</span>}
        </div>
      </Modal>

      <Modal
        open={newOrgModal}
        onClose={() => {
          setNewOrgModal(false)
          resetNewOrg()
        }}
        title={t('organizationPage.modal.newOrganization', 'New Organization')}
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
              {t('organizationPage.actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              loading={createOrg.isPending || isNewOrgSubmitting}
              onClick={handleNewOrgSubmit(handleCreateOrg)}
              disabled={!isNewOrgValid || isNewOrgSubmitting}
            >
              <Plus {...iconProps('xs')} />
              {t('organizationPage.actions.createOrganization', 'Create Organization')}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <Input label={t('organizationPage.form.organizationName', 'Organization Name')} placeholder={t('organizationPage.form.organizationNamePlaceholder', 'e.g. Acme Corp')} {...registerNewOrg('name')} />
          {newOrgErrors.name && <span className="text-xs text-[var(--color-error)]">{newOrgErrors.name.message}</span>}
          <Input
            label={t('organizationPage.form.slug', 'Slug')}
            placeholder={t('organizationPage.form.slugPlaceholder', 'e.g. acme-corp')}
            {...registerNewOrg('slug')}
          />
          {newOrgErrors.slug && <span className="text-xs text-[var(--color-error)]">{newOrgErrors.slug.message}</span>}
          <Input label={t('organizationPage.form.domainOptional', 'Domain (optional)')} placeholder={t('organizationPage.form.domainPlaceholder', 'e.g. acme.com')} {...registerNewOrg('domain')} />
          {newOrgErrors.domain && <span className="text-xs text-[var(--color-error)]">{newOrgErrors.domain.message}</span>}
        </div>
      </Modal>

      <ConfirmDialog
        open={removeMemberId !== null}
        onClose={() => setRemoveMemberId(null)}
        onConfirm={handleConfirmRemove}
        title={t('organizationPage.confirm.removeMemberTitle', 'Remove Member')}
        description={t('organizationPage.confirm.removeMemberDescription', 'Remove the selected member from this organization. Continue?')}
        confirmLabel={t('organizationPage.actions.remove', 'Remove')}
        loading={removeMember.isPending}
      />
    </div>
  )
}
