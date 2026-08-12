import { beforeEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { AppLogPanel, AppUsageCharts } from './app-runtime-panels'

const mockUseStackWorkloads = vi.hoisted(() => vi.fn())
const mockUseStackWorkloadLogs = vi.hoisted(() => vi.fn())

vi.mock('../../stack/api/stack-api', () => ({
  useStackWorkloads: (...args: unknown[]) => mockUseStackWorkloads(...args),
  useStackWorkloadLogs: (...args: unknown[]) => mockUseStackWorkloadLogs(...args),
}))

// recharts 는 jsdom 에서 폭이 0이라 범례도 선도 그리지 않는다.
// 어떤 선이 그려지는지 보려면 Line 을 스텁으로 두는 수밖에 없다.
vi.mock('recharts', () => {
  const Pass = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>
  return {
    ResponsiveContainer: Pass,
    LineChart: Pass,
    CartesianGrid: Pass,
    XAxis: Pass,
    YAxis: Pass,
    Tooltip: Pass,
    Legend: Pass,
    Line: ({ dataKey }: { dataKey: string }) => <div data-testid="usage-line">{dataKey}</div>,
  }
})

function pod(name: string, cpu: number | null, memory: number | null) {
  return { kind: 'Pod', name, namespace: 'apps', status: 'Running', cpuMillicores: cpu, memoryMib: memory }
}

const twoApps = {
  dataUpdatedAt: 1000,
  data: {
    pipelines: [
      { id: 'pl-1', name: 'demo-app', namespace: 'apps', status: 'success', lastDeployment: null, k8sObjects: [pod('demo-app-a', 10, 100)] },
      { id: 'pl-2', name: 'demo-api', namespace: 'apps', status: 'success', lastDeployment: null, k8sObjects: [pod('demo-api-a', 20, 200)] },
    ],
    summary: { totalPipelines: 2, totalDeployments: 0, runningPods: 2, pendingPods: 0, failedPods: 0 },
  },
}

describe('AppUsageCharts', () => {
  beforeEach(() => {
    mockUseStackWorkloads.mockReturnValue({ data: undefined, dataUpdatedAt: 0 })
    mockUseStackWorkloadLogs.mockReturnValue({ data: undefined, isLoading: false })
  })

  it('스택의 모든 앱을 그린다', () => {
    mockUseStackWorkloads.mockReturnValue(twoApps)

    renderWithProviders(<AppUsageCharts stackId="stk_1" />)

    // CPU/Memory 두 차트에 앱마다 선 하나씩.
    const drawn = screen.getAllByTestId('usage-line').map((n) => n.textContent)
    expect(drawn).toEqual(['demo-api', 'demo-app', 'demo-api', 'demo-app'])
  })

  // 파이프라인 상세에서는 그 파이프라인만 봐야 한다. 옆 앱의 선이 섞이면
  // 어느 것이 이 앱인지 알 수 없다.
  it('apps 를 주면 그 앱만 그린다', () => {
    mockUseStackWorkloads.mockReturnValue(twoApps)

    renderWithProviders(<AppUsageCharts stackId="stk_1" apps={['demo-app']} />)

    const drawn = screen.getAllByTestId('usage-line').map((n) => n.textContent)
    expect(drawn).toEqual(['demo-app', 'demo-app'])
  })

  it('스택을 모르면 이유를 알려 준다', () => {
    renderWithProviders(<AppUsageCharts stackId="" />)

    expect(screen.getAllByText(/스택을 고르면|스택에 연결되어야/).length).toBe(2)
  })
})

describe('AppLogPanel', () => {
  beforeEach(() => {
    mockUseStackWorkloads.mockReturnValue({ data: undefined, dataUpdatedAt: 0 })
    mockUseStackWorkloadLogs.mockReturnValue({ data: undefined, isLoading: false })
  })

  const logs = {
    isLoading: false,
    data: {
      pods: ['demo-app-aaaaaa', 'demo-api-bbbbbb'],
      truncated: false,
      lines: [
        { pod: 'demo-app-aaaaaa', app: 'demo-app', timestamp: '2026-08-12T10:20:30.000Z', message: 'from app' },
        { pod: 'demo-api-bbbbbb', app: 'demo-api', timestamp: '2026-08-12T10:20:31.000Z', message: 'from api' },
      ],
    },
  }

  it('스택의 모든 앱 로그를 섞어 보여준다', () => {
    mockUseStackWorkloadLogs.mockReturnValue(logs)

    renderWithProviders(<AppLogPanel stackId="stk_1" />)

    expect(screen.getByText('from app')).toBeTruthy()
    expect(screen.getByText('from api')).toBeTruthy()
  })

  it('apps 를 주면 그 앱의 줄만 남긴다', () => {
    mockUseStackWorkloadLogs.mockReturnValue(logs)

    renderWithProviders(<AppLogPanel stackId="stk_1" apps={['demo-app']} />)

    expect(screen.getByText('from app')).toBeTruthy()
    expect(screen.queryByText('from api')).toBeNull()
  })

  // 걸러 낸 결과가 비었을 때 "로그 없음" 은 맞지만, 스택 전체에는 로그가 있는데
  // 이 앱만 없는 것이므로 로딩과는 구분되어야 한다.
  it('그 앱의 로그가 없으면 없다고 말한다', () => {
    mockUseStackWorkloadLogs.mockReturnValue(logs)

    renderWithProviders(<AppLogPanel stackId="stk_1" apps={['nobody']} />)

    expect(screen.getByText('아직 출력한 로그가 없습니다.')).toBeTruthy()
  })
})
