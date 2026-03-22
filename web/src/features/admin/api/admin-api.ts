import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
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

// --- Query keys ---

const queryKeys = {
  organization: () => ['admin', 'organization'] as const,
  members: (orgId: string) => ['admin', 'members', orgId] as const,
  clusters: () => ['admin', 'clusters'] as const,
  cluster: (id: string) => ['admin', 'clusters', id] as const,
  knownIssues: () => ['admin', 'known-issues'] as const,
  inviteLinks: (orgId: string) => ['invite-links', orgId] as const,
}

// --- API functions ---

const adminApiCalls = {
  getOrganization: () =>
    api.get<Organization>('/admin/organization').then((r) => r.data),

  createOrganization: (data: CreateOrgRequest) =>
    api.post<Organization>('/admin/organizations', data).then((r) => r.data),

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

  deactivateMember: (orgId: string, memberId: string) =>
    api.post<Member>(`/admin/organizations/${orgId}/members/${memberId}/deactivate`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: Cluster[]; total: number }>('/admin/clusters').then((r) => ({
      ...r.data,
      items: (r.data.items ?? []).map((c) => {
        const raw = c as Cluster & { connection_status?: Cluster['status']; org_id?: string }
        return {
          ...c,
          status: raw.status ?? raw.connection_status ?? 'pending',
          organizationIds: c.organizationIds ?? (raw.org_id ? [raw.org_id] : []),
        }
      }),
    })),

  getCluster: (id: string) =>
    api.get<Cluster>(`/admin/clusters/${id}`).then((r) => r.data),

  createCluster: (data: CreateClusterRequest) =>
    api.post<Cluster>('/admin/clusters', data).then((r) => r.data),

  updateCluster: (id: string, data: Partial<CreateClusterRequest>) =>
    api.patch<Cluster>(`/admin/clusters/${id}`, data).then((r) => r.data),

  deleteCluster: (id: string) =>
    api.delete(`/admin/clusters/${id}`).then((r) => r.data),

  verifyCluster: (id: string) =>
    api.post<{ status: string; version?: string }>(`/admin/clusters/${id}/verify`).then((r) => r.data),

  getKnownIssues: () =>
    api.get<{ items: KnownIssue[] }>('/admin/known-issues').then((r) => r.data),

  searchUserByEmail: (email: string) =>
    api.get<{ found: boolean; user?: { id: string; name: string; email: string; is_active: boolean } }>('/admin/users/search', { params: { email } }).then((r) => r.data),

  getClusterNamespaces: (clusterId: string) =>
    api.get<{ items: { name: string }[] }>(`/admin/clusters/${clusterId}/namespaces`).then((r) => r.data?.items ?? []),

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
