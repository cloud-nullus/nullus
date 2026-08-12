import { describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import { renderWithProviders } from '../../../__tests__/test-utils'
import { CicdHistoryPage } from './cicd-history-page'

// 롤백 UI 는 282cc8f 에서 제거됐다. useRollbackDeployment 훅과 그 테스트는
// cicd-api 쪽에 남아 있으므로 여기서는 이력 표시만 검증한다.
vi.mock('../api/cicd-api', () => ({
  useDeployments: () => ({
    data: {
      items: [
        {
          id: 'd-success',
          pipelineId: 'frontend-web',
          pipelineName: 'frontend-web',
          version: 'v1.2.3',
          status: 'success',
          triggeredBy: 'kim.dev',
          startedAt: '2026-03-03T14:22:00Z',
          completedAt: '2026-03-03T14:28:00Z',
        },
        {
          id: 'd-failed',
          pipelineId: 'backend-api',
          pipelineName: 'backend-api',
          version: 'v2.1.0',
          status: 'failed',
          triggeredBy: 'park.dev',
          startedAt: '2026-03-01T16:25:00Z',
          completedAt: '2026-03-01T16:30:00Z',
        },
        {
          id: 'd-running',
          pipelineId: 'batch-runner',
          pipelineName: 'batch-runner',
          version: 'v1.3.1',
          status: 'running',
          triggeredBy: 'choi.devops',
          startedAt: '2026-03-03T10:00:00Z',
          completedAt: null,
        },
      ],
      total: 3,
    },
  }),
}))

describe('CicdHistoryPage', () => {
  it('renders the page heading', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'CI/CD History' }),
    ).not.toBeNull()
  })

  it('renders a row per deployment with pipeline, version and deployer', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(screen.getAllByText('frontend-web').length).toBeGreaterThan(0)
    expect(screen.getAllByText('backend-api').length).toBeGreaterThan(0)
    expect(screen.getAllByText('batch-runner').length).toBeGreaterThan(0)

    expect(screen.getByText('v1.2.3')).not.toBeNull()
    expect(screen.getByText('v2.1.0')).not.toBeNull()
    expect(screen.getByText('kim.dev')).not.toBeNull()
  })

  it('shows the deployment status of each row', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(screen.getAllByText(/success/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/failed/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/running/i).length).toBeGreaterThan(0)
  })
  it('selecting a row opens the detail sub table with every field', async () => {
    renderWithProviders(<CicdHistoryPage />)

    // 상세는 선택 전에는 없다.
    expect(screen.queryByText('Deployment Detail')).toBeNull()

    // 행의 상세 보기 버튼을 누른다.
    fireEvent.click(screen.getAllByRole('button', { name: 'View detail' })[0])

    const detail = await screen.findByText('Deployment Detail')
    expect(detail).not.toBeNull()

    // 서브 테이블이 메인 테이블 컬럼과 같은 6개 항목을 모두 보여준다 —
    // 행 확장을 없애도 정보가 줄지 않는다는 계약이다(기획안 D5).
    // 상세 라벨은 개편 전 문자열을 그대로 유지한다 — 'Triggered By' 는 메인 컬럼의
    // 'Deployer' 와 어긋나지만, 문자열을 없애는 건 정보 유실이므로 보존한다.
    const section = detail.closest('section') as HTMLElement
    for (const label of ['Pipeline', 'Version', 'Triggered By', 'Status', 'Started At', 'Completed At']) {
      expect(within(section).getByText(label)).not.toBeNull()
    }
    expect(within(section).getByText('frontend-web')).not.toBeNull()
    expect(within(section).getByText('v1.2.3')).not.toBeNull()
    expect(within(section).getByText('kim.dev')).not.toBeNull()
  })

  // 상세는 표 오른쪽에 붙는다. 아래로 펼치면 행을 고를 때마다 표가 밀려 내려가
  // 방금 고른 행을 잃고, 표 아래로 화면 절반이 빈 채 남는다.
  // 스택 이력·스택 목록·CI/CD 목록이 모두 이 좌우 배치를 쓴다.
  it('상세를 표 오른쪽에 둔다', async () => {
    renderWithProviders(<CicdHistoryPage />)

    fireEvent.click(screen.getAllByRole('button', { name: 'View detail' })[0])
    const detail = await screen.findByText('Deployment Detail')

    const table = screen.getAllByRole('table')[0]
    // 표와 상세의 최소 공통 조상이 flex 행이면 좌우로 놓인 것이다.
    // 아래로 쌓으면 그 조상은 페이지 바깥 div 라 flex 가 아니다.
    let common: HTMLElement | null = detail.parentElement
    while (common && !common.contains(table)) common = common.parentElement
    expect(common).not.toBeNull()
    expect(common!.className).toContain('flex')
  })

  // 고른 행이 없을 때도 오른쪽 칸은 자리를 지키고 무엇을 하라고 알려 준다.
  it('선택 전에는 고르라고 알려 준다', () => {
    renderWithProviders(<CicdHistoryPage />)

    expect(screen.getByText(/Select a deployment|배포를 선택/)).toBeTruthy()
  })

  it('closing the detail sub table hides it', async () => {
    renderWithProviders(<CicdHistoryPage />)

    fireEvent.click(screen.getAllByRole('button', { name: 'View detail' })[0])
    await screen.findByText('Deployment Detail')

    fireEvent.click(screen.getByRole('button', { name: 'Close' }))

    expect(screen.queryByText('Deployment Detail')).toBeNull()
  })
})
