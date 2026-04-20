import { describe, expect, it } from 'vitest'
import type { CompatibilityMatrix } from '../../../types'
import {
  isMatrixCompatibleWithCluster,
  matrixArchMismatches,
  toolSupportsArch,
} from './compatibility-arch'

const verifiedMatrix = (tools: CompatibilityMatrix['tools']): CompatibilityMatrix => ({
  id: 'fixture-matrix',
  name: 'Fixture Matrix',
  status: 'verified',
  k8sRange: '1.27-1.35',
  tools,
})

const gitlabOnly = verifiedMatrix([
  {
    name: 'GitLab CE',
    helmVersion: '9.5.1',
    appVersion: '18.5.1',
    archSupport: ['amd64'],
    minK8sVersion: '1.27',
    tier: 'stable',
  },
  {
    name: 'Argo CD',
    helmVersion: '6.8.0',
    appVersion: 'v2.8.3',
    archSupport: ['amd64', 'arm64'],
    minK8sVersion: '1.26',
    tier: 'stable',
  },
])

const multiArchOnly = verifiedMatrix([
  {
    name: 'MinIO',
    helmVersion: '5.2.0',
    appVersion: 'RELEASE.2024-08-03T04-33-23Z',
    archSupport: ['amd64', 'arm64'],
    minK8sVersion: '1.26',
    tier: 'stable',
  },
  {
    name: 'Grafana',
    helmVersion: '8.5.0',
    appVersion: '11.1.0',
    archSupport: ['amd64', 'arm64'],
    minK8sVersion: '1.26',
    tier: 'stable',
  },
])

describe('toolSupportsArch', () => {
  it('treats an empty archSupport list as amd64-only for backward compatibility', () => {
    const tool = gitlabOnly.tools[0]
    const empty = { ...tool, archSupport: [] }
    expect(toolSupportsArch(empty, 'amd64')).toBe(true)
    expect(toolSupportsArch(empty, 'arm64')).toBe(false)
  })

  it('returns false for an empty arch string', () => {
    expect(toolSupportsArch(gitlabOnly.tools[0], '')).toBe(false)
  })

  it('matches when the arch is in the list', () => {
    expect(toolSupportsArch(gitlabOnly.tools[1], 'arm64')).toBe(true)
  })
})

describe('isMatrixCompatibleWithCluster', () => {
  it('returns unknown when the cluster has no recorded architectures', () => {
    expect(isMatrixCompatibleWithCluster(gitlabOnly, [])).toBe('unknown')
    expect(isMatrixCompatibleWithCluster(gitlabOnly, undefined)).toBe('unknown')
  })

  it('returns compatible for a pure amd64 cluster with amd64-only tools', () => {
    expect(isMatrixCompatibleWithCluster(gitlabOnly, ['amd64'])).toBe('compatible')
  })

  it('returns incompatible when any tool misses any cluster arch', () => {
    // GitLab is amd64-only — adding arm64 nodes makes the matrix incompatible.
    expect(isMatrixCompatibleWithCluster(gitlabOnly, ['amd64', 'arm64'])).toBe('incompatible')
  })

  it('returns compatible when every tool covers every cluster arch', () => {
    expect(isMatrixCompatibleWithCluster(multiArchOnly, ['amd64', 'arm64'])).toBe('compatible')
  })
})

describe('matrixArchMismatches', () => {
  it('returns an empty list when the cluster archs are unknown', () => {
    expect(matrixArchMismatches(gitlabOnly, [])).toEqual([])
    expect(matrixArchMismatches(gitlabOnly, undefined)).toEqual([])
  })

  it('returns an empty list when everything is compatible', () => {
    expect(matrixArchMismatches(multiArchOnly, ['amd64', 'arm64'])).toEqual([])
  })

  it('enumerates per-tool missing archs on mismatch', () => {
    const result = matrixArchMismatches(gitlabOnly, ['amd64', 'arm64'])
    // GitLab CE should report arm64 missing; Argo CD supports both and is omitted.
    expect(result).toHaveLength(1)
    expect(result[0]).toEqual({ toolName: 'GitLab CE', missingArchs: ['arm64'] })
  })
})
