import { describe, expect, it, vi, beforeEach } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdDefault } from './monitoring-cicd-view'

const mockUsePipelines = vi.hoisted(() => vi.fn())
const mockUseDeployments = vi.hoisted(() => vi.fn())
const mockUseStackWorkloads = vi.hoisted(() => vi.fn())

vi.mock('../../cicd/api/cicd-api', () => ({
  usePipelines: () => mockUsePipelines(),
  useDeployments: () => mockUseDeployments(),
}))

vi.mock('../../stack/api/stack-api', () => ({
  useStackWorkloads: (...args: unknown[]) => mockUseStackWorkloads(...args),
}))

const pipelines = {
  items: [
    {
      id: 'pl-1',
      name: 'demo-app',
      appType: 'web-backend',
      clusterId: 'c1',
      clusterName: 'kind',
      namespace: 'apps',
    },
  ],
}

describe('CicdDefault — 배포 앱 파드 수', () => {
  beforeEach(() => {
    mockUsePipelines.mockReturnValue({ data: pipelines })
    mockUseDeployments.mockReturnValue({ data: { items: [] } })
    mockUseStackWorkloads.mockReturnValue({ data: undefined })
  })

  // 파드 수는 스택 단위 엔드포인트에서 온다. 스택 id 로 조회해야 한다.
  it('고른 스택으로 워크로드를 조회한다', () => {
    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    expect(mockUseStackWorkloads).toHaveBeenCalledWith('stk_1')
  })

  // 개편 전에는 pods: [null, null] 리터럴이라 무엇을 고르든 항상 비었다.
  it('클러스터에서 읽은 파드 수를 보여준다', () => {
    mockUseStackWorkloads.mockReturnValue({
      data: {
        pipelines: [
          {
            id: 'pl-1',
            name: 'demo-app',
            namespace: 'apps',
            status: 'success',
            lastDeployment: null,
            k8sObjects: [
              { kind: 'Deployment', name: 'demo-app', namespace: 'apps', status: 'running', replicas: 3 },
              { kind: 'Pod', name: 'demo-app-a', namespace: 'apps', status: 'Running' },
              { kind: 'Pod', name: 'demo-app-b', namespace: 'apps', status: 'Running' },
              { kind: 'Pod', name: 'demo-app-c', namespace: 'apps', status: 'Pending' },
            ],
          },
        ],
        summary: { totalPipelines: 1, totalDeployments: 0, runningPods: 2, pendingPods: 1, failedPods: 0 },
      },
    })

    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    const row = screen.getByText('demo-app').closest('tr')!
    expect(within(row).getByText('2/3')).toBeTruthy()
  })

  // 스택을 안 고르면 그 열만 비는 이유를 알려 준다 — 표가 고장난 것처럼 보이면 안 된다.
  it('스택 미선택이면 이유를 알려 준다', () => {
    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="" />)

    expect(
      screen.getByText(/스택을 고르면 배포된 애플리케이션의 실제 파드 수/),
    ).toBeTruthy()
    const row = screen.getByText('demo-app').closest('tr')!
    expect(within(row).getByText('select stack')).toBeTruthy()
  })

  // 클러스터를 못 읽었을 때(백엔드가 빈 목록을 준다) 0/0 이 아니라 '—' 다.
  it('워크로드가 비면 대시로 둔다', () => {
    mockUseStackWorkloads.mockReturnValue({
      data: { pipelines: [], summary: { totalPipelines: 0, totalDeployments: 0, runningPods: 0, pendingPods: 0, failedPods: 0 } },
    })

    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    // Pods 열은 헤더 순서상 5번째다. version 등 다른 열도 대시라 위치로 집는다.
    const row = screen.getByText('demo-app').closest('tr')!
    expect(row.querySelectorAll('td')[4].textContent).toBe('—')
  })
})
