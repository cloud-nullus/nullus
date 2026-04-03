import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import { useAuthStore } from '../../../stores/auth-store'
import type {
  Cluster,
  CreateClusterRequest,
  CreateOrgRequest,
  InviteMemberRequest,
  KnownIssue,
  Member,
  MemberRole,
  Organization,
  UpdateOrgRequest,
} from '../../../types'

export type {
  Cluster,
  ClusterStatus,
  ClusterType,
  CloudProvider,
  CreateClusterRequest,
  CreateOrgRequest,
  InviteMemberRequest,
  KnownIssue,
  KnownIssueSeverity,
  KnownIssueStatus,
  MemberRole,
  MemberStatus,
  Organization,
  OrgStatus,
  UpdateOrgRequest,
} from '../../../types'

export interface InviteLink {
  token: string
  role: MemberRole
  expiresAt: string
  status: 'active' | 'expired'
}

export interface ClusterMonitoringSummary {
  total_pods: number
  ready_pods: number
  cpu_request_millicores: number
  cpu_limit_millicores: number
  memory_request_mib: number
  memory_limit_mib: number
}

// --- Query keys ---

const queryKeys = {
  organization: () => ['admin', 'organization'] as const,
  members: (orgId: string) => ['admin', 'members', orgId] as const,
  clusters: () => ['admin', 'clusters'] as const,
  cluster: (id: string) => ['admin', 'clusters', id] as const,
  clusterMonitoringSummary: (id: string) => ['admin', 'clusters', id, 'monitoring-summary'] as const,
  knownIssues: () => ['admin', 'known-issues'] as const,
  inviteLinks: (orgId: string) => ['invite-links', orgId] as const,
}

type ClusterApiShape = Cluster & {
  connection_status?: Cluster['status']
  org_id?: string
  cloud_provider?: Cluster['cloudProvider']
}

const normalizeClusterTypes = (types: Cluster['types'] | undefined, type: Cluster['type'] | undefined): Cluster['types'] => {
  if (Array.isArray(types) && types.length > 0) {
    return Array.from(new Set(types))
  }
  return type ? [type] : []
}

const normalizeCluster = (cluster: ClusterApiShape): Cluster => ({
  ...cluster,
  type: cluster.type ?? 'target',
  types: normalizeClusterTypes(cluster.types, cluster.type),
  cloudProvider: cluster.cloudProvider ?? cluster.cloud_provider ?? 'on_premise',
  status: cluster.status ?? cluster.connection_status ?? 'pending',
  organizationIds: cluster.organizationIds ?? (cluster.org_id ? [cluster.org_id] : []),
})

const toClusterRequestPayload = (data: Partial<CreateClusterRequest>) => {
  const { cloudProvider, ...rest } = data
  return {
    ...rest,
    cloud_provider: cloudProvider,
  }
}

// --- API functions ---

const adminApiCalls = {
  getOrganization: () =>
    api.get<Organization>('/admin/organization').then((r) => r.data),

  createOrganization: (data: CreateOrgRequest) =>
    api.post<Organization>('/admin/orgs', data).then((r) => r.data),

  updateOrganization: (data: UpdateOrgRequest) =>
    api.patch<Organization>('/admin/organization', data).then((r) => r.data),

  getMembers: (orgId: string) =>
    api.get<{ items: Member[]; total: number }>(`/admin/organizations/${orgId}/members`).then((r) => ({
      ...r.data,
      items: (r.data.items ?? []).map((m) => {
        const raw = m as Member & { is_active?: boolean; created_at?: string }
        return {
          ...m,
          status: m.status ?? (raw.is_active ? 'active' : 'pending'),
          joinedAt: m.joinedAt ?? raw.created_at ?? '',
        }
      }),
    })),

  inviteMember: (orgId: string, data: InviteMemberRequest) =>
    api.post<Member>(`/admin/organizations/${orgId}/members`, data).then((r) => r.data),

  removeMember: (orgId: string, memberId: string) =>
    api.delete(`/admin/organizations/${orgId}/members/${memberId}`).then((r) => r.data),

  updateMemberRole: (orgId: string, memberId: string, role: MemberRole) =>
    api.patch<Member>(`/admin/organizations/${orgId}/members/${memberId}`, { role }).then((r) => r.data),

  updateMember: (
    orgId: string,
    memberId: string,
    data: { name: string; email: string; role: MemberRole }
  ) => api.patch<Member>(`/admin/organizations/${orgId}/members/${memberId}`, data).then((r) => r.data),

  deactivateMember: (orgId: string, memberId: string) =>
    api.post<Member>(`/admin/organizations/${orgId}/members/${memberId}/deactivate`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: Cluster[]; total: number }>('/admin/clusters', {
      params: {
        page: 1,
        page_size: 500,
        per_page: 500,
        limit: 500,
      },
    }).then((r) => ({
      ...r.data,
      items: (r.data.items ?? []).map((c) => normalizeCluster(c as ClusterApiShape)),
    })),

  getCluster: (id: string) =>
    api.get<ClusterApiShape>(`/admin/clusters/${id}`).then((r) => normalizeCluster(r.data)),

  createCluster: (data: CreateClusterRequest) =>
    api.post<ClusterApiShape>('/admin/clusters', toClusterRequestPayload(data)).then((r) => normalizeCluster(r.data)),

  updateCluster: (id: string, data: Partial<CreateClusterRequest>) =>
    api.patch<ClusterApiShape>(`/admin/clusters/${id}`, toClusterRequestPayload(data)).then((r) => normalizeCluster(r.data)),

  deleteCluster: (id: string) =>
    api.delete(`/admin/clusters/${id}`).then((r) => r.data),

  verifyCluster: (id: string) =>
    api.post<{ status: string; version?: string }>(`/admin/clusters/${id}/verify`).then((r) => r.data),

  verifyClusterDraft: (data: { endpoint?: string; kubeconfig: string }) =>
    api.post<{ status: string; version?: string }>('/admin/clusters/verify', data).then((r) => r.data),

  getKnownIssues: () =>
    api.get<{ items: KnownIssue[] }>('/admin/known-issues').then((r) => r.data),

  searchUserByEmail: (email: string) =>
    api.get<{ found: boolean; user?: { id: string; name: string; email: string; is_active: boolean } }>('/admin/users/search', { params: { email } }).then((r) => r.data),

  getClusterNamespaces: (clusterId: string) =>
    api.get<{ items: { name: string }[] }>(`/admin/clusters/${clusterId}/namespaces`).then((r) => r.data?.items ?? []),

  getClusterMonitoringSummary: (clusterId: string) =>
    api.get<ClusterMonitoringSummary>(`/admin/clusters/${clusterId}/monitoring-summary`).then((r) => r.data),

  createInviteLink: (orgId: string, data: { role: MemberRole; expiresInDays: number }) =>
    api.post<{ token: string; url: string; role: MemberRole; expiresAt: string }>(`/admin/organizations/${orgId}/invites`, data).then((r) => r.data),

  getInviteLinks: (orgId: string) =>
    api.get<{ items: InviteLink[] }>(`/admin/organizations/${orgId}/invites`).then((r) => r.data),

  revokeInviteLink: (orgId: string, token: string) =>
    api.delete(`/admin/organizations/${orgId}/invites/${token}`).then((r) => r.data),
}

// --- Hooks ---

export function useOrganization() {
  return useQuery({
    queryKey: queryKeys.organization(),
    queryFn: adminApiCalls.getOrganization,
  })
}

export function useCreateOrganization() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApiCalls.createOrganization,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.organization() })
    },
  })
}

export function useUpdateOrganization() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApiCalls.updateOrganization,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.organization() })
    },
  })
}

export function useMembers(orgId: string) {
  return useQuery({
    queryKey: queryKeys.members(orgId),
    queryFn: () => adminApiCalls.getMembers(orgId),
    enabled: !!orgId,
  })
}

export function useInviteMember(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: InviteMemberRequest) => adminApiCalls.inviteMember(orgId, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.members(orgId) })
    },
  })
}

export function useRemoveMember(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (memberId: string) => adminApiCalls.removeMember(orgId, memberId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.members(orgId) })
    },
  })
}

export function useUpdateUserRole(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ memberId, role }: { memberId: string; role: MemberRole }) =>
      adminApiCalls.updateMemberRole(orgId, memberId, role),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.members(orgId) })
    },
  })
}

export function useUpdateMember(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ memberId, data }: { memberId: string; data: { name: string; email: string; role: MemberRole } }) =>
      adminApiCalls.updateMember(orgId, memberId, data),
    onSuccess: (updatedMember, variables) => {
      qc.setQueryData<{ items: Member[]; total: number } | undefined>(queryKeys.members(orgId), (prev) => {
        if (!prev) {
          return prev
        }

        const nextItems = prev.items.map((member) => {
          if (member.id !== variables.memberId) {
            return member
          }

          return {
            ...member,
            name: updatedMember?.name ?? variables.data.name,
            email: updatedMember?.email ?? variables.data.email,
            role: updatedMember?.role ?? variables.data.role,
            status: updatedMember?.status ?? member.status,
            joinedAt: updatedMember?.joinedAt ?? member.joinedAt,
          }
        })

        return {
          ...prev,
          items: nextItems,
          total: nextItems.length,
        }
      })
      void qc.invalidateQueries({ queryKey: queryKeys.members(orgId) })
    },
  })
}

export function useDeactivateUser(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (memberId: string) => adminApiCalls.deactivateMember(orgId, memberId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.members(orgId) })
    },
  })
}

export function useClusters() {
  return useQuery({
    queryKey: queryKeys.clusters(),
    queryFn: adminApiCalls.getClusters,
  })
}

export function useScopedClusters() {
  const { data: clustersData, ...rest } = useClusters()
  const { data: org } = useOrganization()
  const role = useAuthStore((s) => s.role)
  const userOrgId = useAuthStore((s) => s.user?.orgId ?? '')
  const scope = org?.clusterAccessScope ?? []

  const normalizeKey = (value: string) => value.trim().toLowerCase()
  const scopeSet = new Set(scope.map((value) => normalizeKey(value)))

  const items = clustersData?.items ?? []
  if (role === 'admin') {
    return { ...rest, data: clustersData ? { ...clustersData, items, total: items.length } : clustersData }
  }

  const filteredByScope = scope.length > 0
    ? items.filter((c) => scopeSet.has(normalizeKey(c.name)) || scopeSet.has(normalizeKey(c.id)))
    : items

  const filtered = userOrgId
    ? filteredByScope.filter((c) => c.organizationIds.length === 0 || c.organizationIds.includes(userOrgId))
    : filteredByScope

  return { ...rest, data: clustersData ? { ...clustersData, items: filtered, total: filtered.length } : clustersData }
}

export function useCluster(id: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.cluster(id),
    queryFn: () => adminApiCalls.getCluster(id),
    enabled: enabled && !!id,
  })
}


export function useCreateCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApiCalls.createCluster,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.clusters() })
    },
  })
}

export function useUpdateCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreateClusterRequest> }) =>
      adminApiCalls.updateCluster(id, data),
    onSuccess: (_, variables) => {
      void qc.invalidateQueries({ queryKey: queryKeys.clusters() })
      void qc.invalidateQueries({ queryKey: queryKeys.cluster(variables.id) })
    },
  })
}

export function useDeleteCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => adminApiCalls.deleteCluster(id),
    onSuccess: (_, id) => {
      void qc.invalidateQueries({ queryKey: queryKeys.clusters() })
      void qc.invalidateQueries({ queryKey: queryKeys.cluster(id) })
    },
  })
}

export function useVerifyCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => adminApiCalls.verifyCluster(id),
    onSuccess: (_, id) => {
      void qc.invalidateQueries({ queryKey: queryKeys.clusters() })
      void qc.invalidateQueries({ queryKey: queryKeys.cluster(id) })
    },
  })
}

export function useVerifyClusterDraft() {
  return useMutation({
    mutationFn: (data: { endpoint?: string; kubeconfig: string }) => adminApiCalls.verifyClusterDraft(data),
  })
}

export function useSearchUser(email: string) {
  return useQuery({
    queryKey: ['users', 'search', email],
    queryFn: () => adminApiCalls.searchUserByEmail(email),
    enabled: email.length > 3 && email.includes('@'),
  })
}

export function useClusterNamespaces(clusterId: string) {
  return useQuery({
    queryKey: ['admin', 'clusters', clusterId, 'namespaces'],
    queryFn: () => adminApiCalls.getClusterNamespaces(clusterId),
    enabled: !!clusterId,
  })
}

export function useClusterMonitoringSummary(clusterId: string) {
  return useQuery({
    queryKey: queryKeys.clusterMonitoringSummary(clusterId),
    queryFn: () => adminApiCalls.getClusterMonitoringSummary(clusterId),
    enabled: !!clusterId,
    refetchInterval: 5000,
  })
}

export function useKnownIssues() {
  return useQuery({
    queryKey: queryKeys.knownIssues(),
    queryFn: adminApiCalls.getKnownIssues,
  })
}

export function useCreateInviteLink(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { role: MemberRole; expiresInDays: number }) =>
      adminApiCalls.createInviteLink(orgId, data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.inviteLinks(orgId) })
    },
  })
}

export function useInviteLinks(orgId: string) {
  return useQuery({
    queryKey: queryKeys.inviteLinks(orgId),
    queryFn: () => adminApiCalls.getInviteLinks(orgId),
    enabled: !!orgId,
  })
}

export function useRevokeInviteLink(orgId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (token: string) => adminApiCalls.revokeInviteLink(orgId, token),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.inviteLinks(orgId) })
    },
  })
}
