import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@tanstack/react-query', () => ({
  useQuery: vi.fn(),
  useMutation: vi.fn(),
  useQueryClient: vi.fn(),
}))

vi.mock('../../../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import {
  useClusterNamespaces,
  useClusters,
  useCreateCluster,
  useCreateInviteLink,
  useCreateOrganization,
  useDeactivateUser,
  useDeleteCluster,
  useInviteLinks,
  useInviteMember,
  useKnownIssues,
  useMembers,
  useOrganization,
  useRemoveMember,
  useRevokeInviteLink,
  useSearchUser,
  useUpdateCluster,
  useUpdateOrganization,
  useUpdateUserRole,
  useVerifyCluster,
} from './admin-api'

describe('admin-api hooks', () => {
  const latestMutationConfig = () => {
    const calls = vi.mocked(useMutation).mock.calls
    return calls[calls.length - 1]?.[0] as { onSuccess?: (...args: unknown[]) => void }
  }

  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useQuery).mockReturnValue({} as never)
    vi.mocked(useMutation).mockReturnValue({} as never)
    vi.mocked(useQueryClient).mockReturnValue({ invalidateQueries: vi.fn() } as never)

    vi.mocked(api.get).mockReset()
    vi.mocked(api.post).mockReset()
    vi.mocked(api.patch).mockReset()
    vi.mocked(api.delete).mockReset()
  })

  it('exports expected hooks as functions', () => {
    expect(typeof useOrganization).toBe('function')
    expect(typeof useCreateOrganization).toBe('function')
    expect(typeof useUpdateOrganization).toBe('function')
    expect(typeof useMembers).toBe('function')
    expect(typeof useInviteMember).toBe('function')
    expect(typeof useRemoveMember).toBe('function')
    expect(typeof useUpdateUserRole).toBe('function')
    expect(typeof useDeactivateUser).toBe('function')
    expect(typeof useClusters).toBe('function')
    expect(typeof useCreateCluster).toBe('function')
    expect(typeof useUpdateCluster).toBe('function')
    expect(typeof useDeleteCluster).toBe('function')
    expect(typeof useVerifyCluster).toBe('function')
    expect(typeof useSearchUser).toBe('function')
    expect(typeof useClusterNamespaces).toBe('function')
    expect(typeof useKnownIssues).toBe('function')
    expect(typeof useCreateInviteLink).toBe('function')
    expect(typeof useInviteLinks).toBe('function')
    expect(typeof useRevokeInviteLink).toBe('function')
  })

  it('configures query hooks with expected query keys', () => {
    useOrganization()
    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(expect.objectContaining({ queryKey: ['admin', 'organization'] }))

    useMembers('org-1')
    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ['admin', 'members', 'org-1'], enabled: true })
    )

    useKnownIssues()
    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(expect.objectContaining({ queryKey: ['admin', 'known-issues'] }))

    useInviteLinks('org-1')
    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ['invite-links', 'org-1'], enabled: true })
    )

    useSearchUser('dev@nullus.io')
    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ['users', 'search', 'dev@nullus.io'], enabled: true })
    )
  })

  it('configures organization and member mutation invalidation', () => {
    const invalidateQueries = vi.fn()
    vi.mocked(useQueryClient).mockReturnValue({ invalidateQueries } as never)

    useCreateOrganization()
    latestMutationConfig().onSuccess?.()

    useUpdateOrganization()
    latestMutationConfig().onSuccess?.()

    useInviteMember('org-1')
    latestMutationConfig().onSuccess?.()

    useUpdateUserRole('org-1')
    latestMutationConfig().onSuccess?.()

    useDeactivateUser('org-1')
    latestMutationConfig().onSuccess?.()

    useRemoveMember('org-1')
    latestMutationConfig().onSuccess?.()

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['admin', 'organization'] })
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['admin', 'members', 'org-1'] })
  })

  it('configures cluster mutation invalidation with cluster-specific keys', () => {
    const invalidateQueries = vi.fn()
    vi.mocked(useQueryClient).mockReturnValue({ invalidateQueries } as never)

    useCreateCluster()
    latestMutationConfig().onSuccess?.()

    useUpdateCluster()
    latestMutationConfig().onSuccess?.(undefined, { id: 'cluster-1', data: { name: 'new-name' } })

    useDeleteCluster()
    latestMutationConfig().onSuccess?.(undefined, 'cluster-1')

    useVerifyCluster()
    latestMutationConfig().onSuccess?.(undefined, 'cluster-1')

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['admin', 'clusters'] })
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['admin', 'clusters', 'cluster-1'] })
  })

  it('configures invite-link mutation invalidation', () => {
    const invalidateQueries = vi.fn()
    vi.mocked(useQueryClient).mockReturnValue({ invalidateQueries } as never)

    useCreateInviteLink('org-1')
    latestMutationConfig().onSuccess?.()

    useRevokeInviteLink('org-1')
    latestMutationConfig().onSuccess?.()

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['invite-links', 'org-1'] })
  })
})
