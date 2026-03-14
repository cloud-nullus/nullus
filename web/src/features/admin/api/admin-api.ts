import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'

// --- Types ---

export type OrgStatus = 'active' | 'inactive' | 'suspended'
export type MemberRole = 'admin' | 'devops' | 'developer'
export type MemberStatus = 'active' | 'pending' | 'inactive'
export type ClusterType = 'kubernetes' | 'eks' | 'gke' | 'aks' | 'k3s'
export type ClusterStatus = 'connected' | 'pending' | 'error' | 'inactive'

export interface Organization {
  id: string
  name: string
  slug: string
  domain: string
  status: OrgStatus
  clusterAccessScope: string[]
  createdAt: string
}

export interface OrgMember {
  id: string
  name: string
  email: string
  role: MemberRole
  status: MemberStatus
  joinedAt: string
}

export interface Cluster {
  id: string
  name: string
  type: ClusterType
  endpoint: string
  status: ClusterStatus
  organizationIds: string[]
  createdAt: string
}

export interface UpdateOrgRequest {
  name?: string
  slug?: string
  domain?: string
  status?: OrgStatus
  clusterAccessScope?: string[]
}

export interface InviteMemberRequest {
  email: string
  role: MemberRole
}

export interface CreateClusterRequest {
  name: string
  type: ClusterType
  kubeconfig: string
}

// --- Query keys ---

const queryKeys = {
  organization: () => ['admin', 'organization'] as const,
  members: (orgId: string) => ['admin', 'members', orgId] as const,
  clusters: () => ['admin', 'clusters'] as const,
  cluster: (id: string) => ['admin', 'clusters', id] as const,
}

// --- API functions ---

const adminApiCalls = {
  getOrganization: () =>
    api.get<Organization>('/admin/organization').then((r) => r.data),

  updateOrganization: (data: UpdateOrgRequest) =>
    api.patch<Organization>('/admin/organization', data).then((r) => r.data),

  getMembers: (orgId: string) =>
    api.get<{ items: OrgMember[]; total: number }>(`/admin/organizations/${orgId}/members`).then((r) => r.data),

  inviteMember: (orgId: string, data: InviteMemberRequest) =>
    api.post<OrgMember>(`/admin/organizations/${orgId}/members`, data).then((r) => r.data),

  removeMember: (orgId: string, memberId: string) =>
    api.delete(`/admin/organizations/${orgId}/members/${memberId}`).then((r) => r.data),

  getClusters: () =>
    api.get<{ items: Cluster[]; total: number }>('/admin/clusters').then((r) => r.data),

  getCluster: (id: string) =>
    api.get<Cluster>(`/admin/clusters/${id}`).then((r) => r.data),

  createCluster: (data: CreateClusterRequest) =>
    api.post<Cluster>('/admin/clusters', data).then((r) => r.data),

  deleteCluster: (id: string) =>
    api.delete(`/admin/clusters/${id}`).then((r) => r.data),
}

// --- Hooks ---

export function useOrganization() {
  return useQuery({
    queryKey: queryKeys.organization(),
    queryFn: adminApiCalls.getOrganization,
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

export function useClusters() {
  return useQuery({
    queryKey: queryKeys.clusters(),
    queryFn: adminApiCalls.getClusters,
  })
}

export function useCluster(id: string) {
  return useQuery({
    queryKey: queryKeys.cluster(id),
    queryFn: () => adminApiCalls.getCluster(id),
    enabled: !!id,
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

export function useDeleteCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApiCalls.deleteCluster,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.clusters() })
    },
  })
}
