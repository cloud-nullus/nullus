import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import type { JobNotification } from '../utils/job-notifications'

const navigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => navigate }
})

const state = vi.fn()
vi.mock('../hooks/use-job-notifications', () => ({
  useJobNotifications: () => state(),
  SETTLED_RETENTION_MS: 300_000,
}))

const { JobNotificationBell } = await import('./job-notification-bell')

const NOW = Date.parse('2026-09-08T00:05:00Z')

const running: JobNotification = {
  id: 'stack:s1',
  kind: 'stack',
  refId: 's1',
  name: 'demo-stack',
  stage: 'installing',
  phase: 'running',
  startedAt: '2026-09-08T00:03:00Z',
  detailPath: '/stack/logs/s1',
}

const failed: JobNotification = {
  id: 'deployment:d1',
  kind: 'deployment',
  refId: 'd1',
  name: 'demo-pipeline',
  stage: 'failed',
  phase: 'failed',
  startedAt: '2026-09-08T00:00:00Z',
  settledAt: NOW - 10_000,
  detailPath: '/cicd/pipelines/p1/logs?deploymentId=d1',
}

const succeeded: JobNotification = {
  ...running,
  stage: 'completed',
  phase: 'succeeded',
  settledAt: NOW - 5_000,
}

function setJobs(jobs: JobNotification[]) {
  state.mockReturnValue({
    jobs,
    runningCount: jobs.filter((job) => job.phase === 'running').length,
    now: NOW,
  })
}

const bell = () => screen.getByTestId('job-notification-bell')

beforeEach(() => {
  vi.clearAllMocks()
  setJobs([])
})

describe('JobNotificationBell — 종 아이콘 (N-2)', () => {
  it('진행 중 작업이 없으면 정적 아이콘이다', () => {
    renderWithProviders(<JobNotificationBell />)

    expect(bell()).toHaveAttribute('data-busy', 'false')
    expect(screen.queryByTestId('job-notification-count')).not.toBeInTheDocument()
  })

  it('진행 중 작업이 있으면 애니메이션 상태가 된다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    expect(bell()).toHaveAttribute('data-busy', 'true')
    // SVG 의 className 은 문자열이 아니라 SVGAnimatedString 이라 속성으로 읽는다.
    expect(screen.getByTestId('job-notification-icon').getAttribute('class')).toContain(
      'nullus-bell--busy',
    )
  })

  it('진행 중 개수를 배지로 보여 준다', () => {
    setJobs([running, { ...running, id: 'stack:s2', refId: 's2', name: 'other-stack' }])
    renderWithProviders(<JobNotificationBell />)

    expect(screen.getByTestId('job-notification-count')).toHaveTextContent('2')
  })

  // 아이콘만 있는 버튼이라 이름은 aria-label 이 전부다. 진행 중일 때는 개수까지
  // 들려야 화면을 보지 않는 사용자도 상태를 안다.
  it('진행 중이면 접근 이름에 개수가 들어간다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    expect(bell()).toHaveAccessibleName('Background jobs — 1 in progress')
  })

  it('진행 중이 없으면 접근 이름이 단순하다', () => {
    renderWithProviders(<JobNotificationBell />)
    expect(bell()).toHaveAccessibleName('Background jobs')
  })
})

describe('JobNotificationBell — 드롭다운 (N-3)', () => {
  it('처음에는 닫혀 있다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    expect(screen.queryByTestId('job-notification-panel')).not.toBeInTheDocument()
    expect(bell()).toHaveAttribute('aria-expanded', 'false')
  })

  it('종을 누르면 진행 중 작업의 이름·단계·경과를 보여 준다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())

    const panel = screen.getByTestId('job-notification-panel')
    expect(within(panel).getByText('demo-stack')).toBeInTheDocument()
    expect(within(panel).getByText(/Installing/)).toBeInTheDocument()
    expect(within(panel).getByText('2m 0s')).toBeInTheDocument()
    expect(bell()).toHaveAttribute('aria-expanded', 'true')
  })

  it('한 번 더 누르면 닫힌다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())
    fireEvent.click(bell())

    expect(screen.queryByTestId('job-notification-panel')).not.toBeInTheDocument()
  })

  it('도는 작업이 없으면 빈 상태 문구를 보여 준다', () => {
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())

    expect(screen.getByText('Nothing is running right now.')).toBeInTheDocument()
  })

  it('완료로 바뀐 작업을 완료 상태로 보여 준다', () => {
    setJobs([succeeded])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())

    const item = screen.getByTestId('job-notification-item-stack:s1')
    expect(item).toHaveAttribute('data-phase', 'succeeded')
    expect(within(item).getByText(/Completed/)).toBeInTheDocument()
  })

  it('실패로 바뀐 작업을 실패 상태로 보여 준다', () => {
    setJobs([failed])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())

    const item = screen.getByTestId('job-notification-item-deployment:d1')
    expect(item).toHaveAttribute('data-phase', 'failed')
    expect(within(item).getByText(/Failed/)).toBeInTheDocument()
  })

  // 끝난 작업의 경과는 흐르면 안 된다 — 끝난 시각에서 멈춘 값을 보여 준다.
  it('끝난 작업의 경과는 끝난 시각까지만 센다', () => {
    setJobs([failed])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())

    // 00:00:00 시작, NOW-10s 에 끝났으므로 4분 50초.
    expect(within(screen.getByTestId('job-notification-panel')).getByText('4m 50s')).toBeInTheDocument()
  })

  it('항목을 누르면 상세 화면으로 이동하고 드롭다운이 닫힌다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())
    fireEvent.click(screen.getByTestId('job-notification-item-stack:s1'))

    expect(navigate).toHaveBeenCalledWith('/stack/logs/s1')
    expect(screen.queryByTestId('job-notification-panel')).not.toBeInTheDocument()
  })

  it('배포 항목은 파이프라인 로그로 이동한다', () => {
    setJobs([failed])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())
    fireEvent.click(screen.getByTestId('job-notification-item-deployment:d1'))

    expect(navigate).toHaveBeenCalledWith('/cicd/pipelines/p1/logs?deploymentId=d1')
  })

  it('Escape 로 닫는다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())
    fireEvent.keyDown(document, { key: 'Escape' })

    expect(screen.queryByTestId('job-notification-panel')).not.toBeInTheDocument()
  })

  it('바깥을 누르면 닫는다', () => {
    setJobs([running])
    renderWithProviders(<JobNotificationBell />)

    fireEvent.click(bell())
    fireEvent.mouseDown(document.body)

    expect(screen.queryByTestId('job-notification-panel')).not.toBeInTheDocument()
  })
})
