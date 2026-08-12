import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fireEvent, render, screen, renderHook } from '@testing-library/react'
import { optionLabels, selectOption } from '../../../__tests__/test-utils'
import type { Cluster, Stack } from '../../../types'
import { ClusterStackFilter, useClusterStackFilterState } from './cluster-stack-filter'

const mockUseClusters = vi.hoisted(() => vi.fn())
const mockUseStacks = vi.hoisted(() => vi.fn())

// 컴포넌트는 조직 권한으로 걸러진 useScopedClusters 를 쓴다.
vi.mock('../../admin/api/admin-api', () => ({
  useScopedClusters: mockUseClusters,
}))

vi.mock('../../stack/api/stack-api', () => ({
  useStacks: mockUseStacks,
}))

describe('useClusterStackFilterState', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseClusters.mockReturnValue({
      data: {
        items: [
          { id: 'c1', name: 'prod-cluster', status: 'connected' },
          { id: 'c2', name: 'dev-cluster', status: 'pending' },
        ],
      },
    })
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          { id: 's1', name: 'stack-a', clusterId: 'c1', status: 'running' },
          { id: 's2', name: 'stack-b', clusterId: 'c2', status: 'warning' },
        ],
      },
    })
  })

  it('returns all stacks when cluster is not selected', () => {
    const { result } = renderHook(() => useClusterStackFilterState('', ''))

    expect(result.current.filteredStacks).toHaveLength(2)
    expect(result.current.hasContext).toBe(false)
  })

  it('filters stacks by selected cluster and resolves selected stack', () => {
    const { result } = renderHook(() => useClusterStackFilterState('c1', 's1'))

    expect(result.current.filteredStacks).toHaveLength(1)
    expect(result.current.filteredStacks[0]?.id).toBe('s1')
    expect(result.current.selectedCluster?.name).toBe('prod-cluster')
    expect(result.current.selectedStack?.name).toBe('stack-a')
    expect(result.current.hasContext).toBe(true)
  })
})

describe('ClusterStackFilter', () => {
  const clusters: Cluster[] = [{
    id: 'c1',
    name: 'prod-cluster',
    type: 'target',
    types: ['target'],
    cloudProvider: 'aws',
    endpoint: 'https://prod.example.com',
    status: 'connected',
    organizationIds: ['org-1'],
    createdAt: '2026-01-01T00:00:00Z',
  }]
  const stacks: Stack[] = [{
    id: 's1',
    name: 'stack-a',
    templateId: 'tmpl-1',
    templateName: 'Template A',
    clusterId: 'c1',
    clusterName: 'prod-cluster',
    status: 'running',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }]

  it('renders cluster/stack selects and options', () => {
    render(
      <ClusterStackFilter
        selectedClusterId=""
        selectedStackId=""
        onClusterChange={vi.fn()}
        onStackChange={vi.fn()}
        onClear={vi.fn()}
        clusters={clusters}
        filteredStacks={stacks}
      />
    )

    expect(screen.queryByLabelText('Cluster')).not.toBeNull()
    expect(screen.queryByLabelText('Stack')).not.toBeNull()
    // 목록은 포털 Menu 라 열어야 DOM 에 나온다.
    expect(optionLabels(screen.getByLabelText('Cluster'))).toContain('prod-cluster')
    expect(optionLabels(screen.getByLabelText('Stack'))).toContain('stack-a (running)')
  })

  it('calls handlers when selecting cluster/stack and clear action', () => {
    const onClusterChange = vi.fn()
    const onStackChange = vi.fn()
    const onClear = vi.fn()

    render(
      <ClusterStackFilter
        // 고르기 전 상태에서 시작한다. 이미 선택된 항목을 다시 누르면
        // 값이 그대로라 onChange 가 안 난다 — native <select> 와 같다.
        selectedClusterId=""
        selectedStackId="s1"
        onClusterChange={onClusterChange}
        onStackChange={onStackChange}
        onClear={onClear}
        clusters={clusters}
        filteredStacks={stacks}
        selectedCluster={clusters[0]}
        selectedStack={stacks[0]}
      />
    )

    selectOption(screen.getByLabelText('Cluster'), 'prod-cluster')
    selectOption(screen.getByLabelText('Stack'), '— Select Stack —')
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))

    expect(onClusterChange).toHaveBeenCalledWith('c1')
    expect(onStackChange).toHaveBeenCalledWith('')
    expect(onClear).toHaveBeenCalled()
  })
})
