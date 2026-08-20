import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { StackConnectPanel } from './monitoring-connect-panel'
import type { OSSMonitoringStatus } from '../../stack/api/stack-api-types'

const mockUseStackMonitoring = vi.hoisted(() => vi.fn())

vi.mock('../../stack/api/stack-api', () => ({
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
}))

function oss(partial: Partial<OSSMonitoringStatus> & { key: string; name: string }): OSSMonitoringStatus {
  return {
    version: '1.0.0',
    enabled: true,
    status: 'running',
    pod_count: 1,
    ready_pods: 1,
    pods: [],
    ...partial,
  } as OSSMonitoringStatus
}

function snapshot(...tools: OSSMonitoringStatus[]) {
  return { data: { oss_statuses: tools }, isLoading: false }
}

const grafana = oss({
  key: 'visualization',
  name: 'grafana',
  version: '11.5.1',
  url: 'https://grafana.nullus.local',
})
const argocd = oss({
  key: 'cd_tool',
  name: 'argocd',
  version: 'v2.13.3',
  url: 'https://argocd.nullus.local',
})

function renderPanel(onConnect = vi.fn(), onSkip = vi.fn()) {
  renderWithProviders(
    <StackConnectPanel stackId="stk_1" stackName="demo" onConnect={onConnect} onSkip={onSkip} />,
  )
  return { onConnect, onSkip }
}

describe('StackConnectPanel', () => {
  beforeEach(() => {
    mockUseStackMonitoring.mockReturnValue(snapshot(grafana, argocd))
  })

  // 예전에는 도구 목록이 화면에 하드코딩돼 있어 실제로 깔리지 않은 Kibana 까지
  // "detected" 로 떴다. 목록은 스택이 실제로 설치한 것에서만 온다.
  it('스택이 실제로 설치한 도구만 나열한다', () => {
    renderPanel()

    expect(screen.getByTestId('connect-row-visualization')).toBeInTheDocument()
    expect(screen.getByTestId('connect-row-cd_tool')).toBeInTheDocument()
    expect(screen.queryByText(/kibana/i)).not.toBeInTheDocument()
  })

  it('설치할 때 받은 접속 도메인으로 주소를 미리 채운다', () => {
    renderPanel()

    expect(screen.getByTestId('connect-url-visualization')).toHaveValue('https://grafana.nullus.local')
    expect(screen.getByTestId('connect-url-cd_tool')).toHaveValue('https://argocd.nullus.local')
  })

  // 게이트웨이 앞에 다른 주소를 두는 설치가 있다. 미리 채운 값은 출발점이지 강제가 아니다.
  it('미리 채운 주소를 고쳐서 쓸 수 있다', () => {
    const { onConnect } = renderPanel()

    fireEvent.change(screen.getByTestId('connect-url-visualization'), {
      target: { value: 'https://metrics.example.com' },
    })
    fireEvent.click(screen.getByTestId('connect-embed-visualization'))
    fireEvent.click(screen.getByTestId('connect-submit'))

    expect(onConnect).toHaveBeenCalledWith([
      { label: 'grafana', url: 'https://metrics.example.com' },
    ])
  })

  // 대부분의 OSS 는 X-Frame-Options 로 iframe 을 막는다. 기본 동작은 새 창이고,
  // 임베드는 되는 것만 골라서 켜는 선택지다.
  it('도구마다 새 창으로 여는 링크를 준다', () => {
    renderPanel()

    const open = screen.getByTestId('connect-open-visualization')
    expect(open).toHaveAttribute('href', 'https://grafana.nullus.local')
    expect(open).toHaveAttribute('target', '_blank')
  })

  it('임베드로 고른 도구만 탭이 된다', () => {
    const { onConnect } = renderPanel()

    fireEvent.click(screen.getByTestId('connect-embed-cd_tool'))
    fireEvent.click(screen.getByTestId('connect-submit'))

    expect(onConnect).toHaveBeenCalledWith([
      { label: 'argocd', url: 'https://argocd.nullus.local' },
    ])
  })

  it('아무것도 고르지 않으면 탭을 추가할 수 없다', () => {
    renderPanel()

    expect(screen.getByTestId('connect-submit')).toBeDisabled()
  })

  // 주소 규칙을 모르는 도구까지 그럴듯한 호스트를 지어내지 않는다.
  it('서버가 주소를 주지 않은 도구는 빈 입력으로 둔다', () => {
    mockUseStackMonitoring.mockReturnValue(
      snapshot(oss({ key: 'trace_layer', name: 'unknown-tool', url: undefined })),
    )

    renderPanel()

    expect(screen.getByTestId('connect-url-trace_layer')).toHaveValue('')
    expect(screen.queryByTestId('connect-open-trace_layer')).not.toBeInTheDocument()
  })

  it('설치된 도구를 아직 읽지 못하면 안내만 남긴다', () => {
    mockUseStackMonitoring.mockReturnValue({ data: undefined, isLoading: false })

    renderPanel()

    expect(screen.queryByTestId('connect-submit')).not.toBeInTheDocument()
    expect(screen.getByTestId('connect-empty')).toBeInTheDocument()
  })
})
