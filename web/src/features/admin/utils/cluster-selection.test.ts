import { describe, it, expect } from 'vitest'
import { findPlatformCluster } from './cluster-selection'
import type { Cluster } from '../../../types'

const cluster = (overrides: Partial<Cluster>): Cluster => ({
  id: 'id',
  name: 'name',
  type: 'target',
  types: ['target'],
  cloudProvider: 'on_premise',
  endpoint: 'https://example.invalid',
  status: 'connected',
  organizationIds: [],
  createdAt: '2026-01-01T00:00:00Z',
  nodeArchitectures: [],
  ...overrides,
})

describe('findPlatformCluster', () => {
  it('returns undefined when the list is empty', () => {
    expect(findPlatformCluster([])).toBeUndefined()
  })

  it('picks the cluster whose types include pipeline, regardless of list order', () => {
    const target = cluster({ id: 'develop', name: 'kind-nullus-develop', type: 'target', types: ['target'] })
    const platform = cluster({ id: 'platform', name: 'kind-nullus-platform', type: 'pipeline', types: ['pipeline'] })

    // develop sorts first (e.g. registered later / created_at DESC) — selection must not depend on order
    expect(findPlatformCluster([target, platform])?.id).toBe('platform')
    expect(findPlatformCluster([platform, target])?.id).toBe('platform')
  })

  it('falls back to the type field when types array is missing pipeline', () => {
    const platform = cluster({ id: 'platform', type: 'pipeline', types: [] })
    expect(findPlatformCluster([platform])?.id).toBe('platform')
  })

  it('returns undefined when no cluster is typed as pipeline', () => {
    const target = cluster({ id: 'develop', type: 'target', types: ['target'] })
    expect(findPlatformCluster([target])).toBeUndefined()
  })
})
