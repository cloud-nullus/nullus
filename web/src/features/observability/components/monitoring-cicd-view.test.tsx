import { describe, expect, it, vi, beforeEach } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdDefault } from './monitoring-cicd-view'

const mockUsePipelines = vi.hoisted(() => vi.fn())
const mockUseDeployments = vi.hoisted(() => vi.fn())
const mockUseStackWorkloads = vi.hoisted(() => vi.fn())
const mockUseStackWorkloadLogs = vi.hoisted(() => vi.fn())

vi.mock('../../cicd/api/cicd-api', () => ({
  usePipelines: () => mockUsePipelines(),
  useDeployments: () => mockUseDeployments(),
}))

vi.mock('../../stack/api/stack-api', () => ({
  useStackWorkloads: (...args: unknown[]) => mockUseStackWorkloads(...args),
  useStackWorkloadLogs: (...args: unknown[]) => mockUseStackWorkloadLogs(...args),
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
    mockUseStackWorkloadLogs.mockReturnValue({ data: undefined, isLoading: false })
  })

  describe('애플리케이션 로그', () => {
    // 지표만으로는 앱이 왜 그 상태인지 알 수 없다. 같은 화면에서 읽혀야 한다.
    it('고른 스택의 로그를 시간순으로 보여준다', () => {
      mockUseStackWorkloadLogs.mockReturnValue({
        isLoading: false,
        data: {
          pods: ['demo-app-a', 'demo-app-b'],
          truncated: false,
          lines: [
            { pod: 'demo-app-aaaaaa', app: 'demo-app', timestamp: '2026-08-12T10:20:30.000Z', message: 'started' },
            { pod: 'demo-app-bbbbbb', app: 'demo-app', timestamp: '2026-08-12T10:20:31.000Z', message: 'GET / 200' },
          ],
        },
      })

      renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

      expect(screen.getByText('started')).toBeTruthy()
      expect(screen.getByText('GET / 200')).toBeTruthy()
      // 줄마다 출처를 알 수 있어야 한다 — 섞어 놓았으므로.
      expect(screen.getByText('aaaaaa')).toBeTruthy()
      expect(screen.getByText('bbbbbb')).toBeTruthy()
    })

    it('스택 id 와 꼬리 줄 수로 조회한다', () => {
      renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

      expect(mockUseStackWorkloadLogs.mock.calls[0][0]).toBe('stk_1')
      expect(mockUseStackWorkloadLogs.mock.calls[0][1]).toBe(200)
    })

    // 로그가 없는 것과 아직 못 읽은 것은 다르다.
    it('읽는 중과 로그 없음을 구분한다', () => {
      mockUseStackWorkloadLogs.mockReturnValue({ data: undefined, isLoading: true })
      const { unmount } = renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)
      expect(screen.getByText('로그를 읽는 중입니다.')).toBeTruthy()
      unmount()

      mockUseStackWorkloadLogs.mockReturnValue({ data: { lines: [], pods: [], truncated: false }, isLoading: false })
      renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)
      expect(screen.getByText('아직 출력한 로그가 없습니다.')).toBeTruthy()
    })

    it('스택 미선택이면 이유를 알려 준다', () => {
      renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="" />)

      expect(screen.getByText('위에서 스택을 고르면 배포된 앱의 로그를 보여줍니다.')).toBeTruthy()
    })
  })

  // 파드 수는 스택 단위 엔드포인트에서 온다. 스택 id 로 조회해야 한다.
  it('고른 스택으로 워크로드를 조회한다', () => {
    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    expect(mockUseStackWorkloads.mock.calls[0][0]).toBe('stk_1')
  })

  // 실시간 그래프를 그리므로 목록용 30초 주기로는 선이 못 움직인다.
  it('실시간 주기로 조회한다', () => {
    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    expect(mockUseStackWorkloads.mock.calls[0][1]).toBe(5_000)
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

  // "Running" 만으로는 파드가 메모리 한계에 붙어 있는지 놀고 있는지 알 수 없다.
  // 스택 도구 파드는 이미 사용량을 보여주는데 CI/CD 로 배포한 앱만 없었다.
  it('앱의 파드 사용량 합계를 보여준다', () => {
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
              { kind: 'Pod', name: 'demo-app-a', namespace: 'apps', status: 'Running', cpuMillicores: 37, memoryMib: 128 },
              { kind: 'Pod', name: 'demo-app-b', namespace: 'apps', status: 'Running', cpuMillicores: 13, memoryMib: 96 },
            ],
          },
        ],
        summary: { totalPipelines: 1, totalDeployments: 0, runningPods: 2, pendingPods: 0, failedPods: 0 },
      },
    })

    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    const row = screen.getByText('demo-app').closest('tr')!
    expect(within(row).getByText('50m')).toBeTruthy()
    expect(within(row).getByText('224Mi')).toBeTruthy()
  })

  // metrics-server 가 없으면 백엔드가 null 을 준다. 0 으로 그리면 "안 쓰는
  // 파드" 로 읽힌다 — 못 읽은 것과 0 은 다르다.
  it('사용량을 못 읽으면 0 이 아니라 대시다', () => {
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
              { kind: 'Pod', name: 'demo-app-a', namespace: 'apps', status: 'Running', cpuMillicores: null, memoryMib: null },
            ],
          },
        ],
        summary: { totalPipelines: 1, totalDeployments: 0, runningPods: 1, pendingPods: 0, failedPods: 0 },
      },
    })

    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    const row = screen.getByText('demo-app').closest('tr')!
    const cells = [...row.querySelectorAll('td')].map((td) => td.textContent)
    expect(cells).not.toContain('0m')
    expect(cells).not.toContain('0Mi')
  })

  // 실시간 그래프는 "선이 없다" 가 세 가지 뜻이다. 사용자가 할 일이 다르므로
  // 구분해서 말해야 한다 — 스택을 고르거나, 기다리거나, metrics-server 를 깔거나.
  it('스택을 안 고르면 그래프가 이유를 알려 준다', () => {
    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="" />)

    expect(screen.getByText('App CPU (Live)')).toBeTruthy()
    expect(screen.getByText('App Memory (Live)')).toBeTruthy()
    expect(
      screen.getAllByText(/스택을 고르면 배포된 앱의 자원 사용을 실시간으로/).length,
    ).toBe(2)
  })

  it('사용량을 못 읽으면 metrics-server 를 짚어 준다', () => {
    mockUseStackWorkloads.mockReturnValue({
      dataUpdatedAt: 1000,
      data: {
        pipelines: [
          {
            id: 'pl-1',
            name: 'demo-app',
            namespace: 'apps',
            status: 'success',
            lastDeployment: null,
            k8sObjects: [
              { kind: 'Pod', name: 'demo-app-a', namespace: 'apps', status: 'Running', cpuMillicores: null, memoryMib: null },
            ],
          },
        ],
        summary: { totalPipelines: 1, totalDeployments: 0, runningPods: 1, pendingPods: 0, failedPods: 0 },
      },
    })

    renderWithProviders(<CicdDefault selectedClusterId="c1" selectedStackId="stk_1" />)

    expect(screen.getAllByText(/metrics-server 가 있는지 확인/).length).toBe(2)
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
