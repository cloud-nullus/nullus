import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import {
  getSelects,
  optionLabels,
  renderWithProviders,
  selectOption,
  selectedLabel,
} from '../../../__tests__/test-utils'
import { StackHistoryPage } from './stack-history-page'

const mockNavigate = vi.fn()
const mockUseParams = vi.fn()

const mockUseStacks = vi.fn()
const mockUseStackHistory = vi.fn()
const mockUseRollbackStack = vi.fn()
const mockUseStackVersionDiff = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => mockUseParams(),
  }
})

vi.mock('../api/stack-api', () => ({
  useStacks: (...args: unknown[]) => mockUseStacks(...args),
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useRollbackStack: (...args: unknown[]) => mockUseRollbackStack(...args),
  useStackVersionDiff: (...args: unknown[]) => mockUseStackVersionDiff(...args),
}))

vi.mock('../components/version-diff', () => ({
  VersionDiff: () => <div data-testid="version-diff">VersionDiff</div>,
}))

vi.mock('../../../stores/auth-store', () => ({
  useAuthStore: () => ({ role: 'devops', isAuthenticated: true }),
}))

const stacks = [
  {
    id: 'stack-1',
    name: 'Platform Stack',
    templateId: 'tpl-1',
    templateName: 'GitLab + Argo',
    clusterId: 'cluster-1',
    clusterName: 'prod',
    status: 'completed',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
]

describe('StackHistoryPage', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseParams.mockReset()
    mockUseStacks.mockReset()
    mockUseStackHistory.mockReset()
    mockUseRollbackStack.mockReset()
    mockUseStackVersionDiff.mockReset()

    mockUseParams.mockReturnValue({ stackId: 'stack-1' })
    mockUseStacks.mockReturnValue({ data: { items: stacks } })
    mockUseStackHistory.mockReturnValue({ data: [] })
    mockUseRollbackStack.mockReturnValue({ mutate: vi.fn(), isPending: false })
    mockUseStackVersionDiff.mockReturnValue({ data: null })
  })

  it('renders without crash', () => {
    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Stack History').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Compare Versions' })).not.toBeNull()
  })

  it('handles loading-like state safely when history data is undefined', () => {
    mockUseStackHistory.mockReturnValue({ data: undefined })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Stack History').length).toBeGreaterThan(0)
    expect(screen.getByText(/No data available\.|데이터가 없습니다\.|dataTable.empty/)).not.toBeNull()
  })

  it('renders history data', () => {
    mockUseStackHistory.mockReturnValue({
      data: [
        {
          id: 'h-1',
          stackId: 'stack-1',
          version: 3,
          changedBy: 'alice',
          changedAt: '2026-01-03T10:00:00Z',
          reason: 'Scale up',
          snapshot: { replicas: 3 },
        },
      ],
    })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getAllByText('Platform Stack').length).toBeGreaterThan(0)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Scale up').length).toBeGreaterThan(0)
    expect(screen.getAllByText('v3').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Cluster|클러스터/).length).toBeGreaterThan(0)
    expect(screen.getAllByText('prod').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: /Log|로그/ })).toBeInTheDocument()
  })

  // 이 화면은 위에서 고른 스택 하나의 이력만 보여준다. 스택 이름과 클러스터를
  // 행마다 반복하면 같은 값이 계속 찍히면서 폭만 먹는다 — 실제로 그 두 열이
  // 252px 를 차지해 표가 칸을 넘쳤고 "작업" 열이 잘려 보였다.
  //
  // 정보를 없애는 게 아니라 한 번만 적는다. 스택 선택 옆 맥락 줄로 옮긴다.
  it('스택과 클러스터를 행마다 반복하지 않는다', () => {
    mockUseStackHistory.mockReturnValue({
      data: [
        { id: 'h-1', stackId: 'stack-1', version: 1, changedBy: 'system', changedAt: '2026-01-01T10:00:00Z', reason: 'stack created', snapshot: {} },
        { id: 'h-2', stackId: 'stack-1', version: 2, changedBy: 'system', changedAt: '2026-01-02T10:00:00Z', reason: 'deployment started', snapshot: {} },
      ],
    })

    renderWithProviders(<StackHistoryPage />)

    // 값은 여전히 화면에 있다 — 다만 한 번씩만.
    // 스택 이름은 선택기가, 클러스터는 그 옆 맥락 줄이 보여준다.
    expect(screen.getAllByText('Platform Stack')).toHaveLength(1)
    expect(screen.getAllByText('prod')).toHaveLength(1)

    const rows = screen.getAllByRole('row').filter((r) => /v[12]/.test(r.textContent ?? ''))
    expect(rows).toHaveLength(2)
    for (const row of rows) {
      expect(row.textContent).not.toContain('Platform Stack')
      expect(row.textContent).not.toContain('prod')
    }
  })

  // 이력은 최신이 위다. 그리고 "현재" 는 가장 높은 버전이지 목록 첫 행이 아니다.
  //
  // 예전에는 entries[0] 을 현재로 봤는데 API 가 오름차순(v1 → v2)이라 v1 에
  // "현재" 배지가 붙었고, 롤백 버튼은 첫 행에만 숨겨서 정작 현재 버전인 v2 를
  // 롤백 대상으로 내놓고 되돌아갈 곳인 v1 에는 버튼이 없었다.
  describe('현재 버전 판정', () => {
    const twoVersions = [
      { id: 'h-1', stackId: 'stack-1', version: 1, changedBy: 'system', changedAt: '2026-01-01T10:00:00Z', reason: 'stack created', snapshot: {} },
      { id: 'h-2', stackId: 'stack-1', version: 2, changedBy: 'system', changedAt: '2026-01-02T10:00:00Z', reason: 'deployment started', snapshot: {} },
    ]

    it('가장 높은 버전에 현재 배지를 붙인다', () => {
      mockUseStackHistory.mockReturnValue({ data: twoVersions })

      renderWithProviders(<StackHistoryPage />)

      const currentRow = screen.getByText(/CURRENT|현재/).closest('tr')!
      expect(currentRow.textContent).toContain('v2')
      expect(currentRow.textContent).not.toContain('v1')
    })

    it('최신 버전을 맨 위에 둔다', () => {
      mockUseStackHistory.mockReturnValue({ data: twoVersions })

      renderWithProviders(<StackHistoryPage />)

      // 헤더 행을 뺀 본문 행 순서를 본다.
      const bodyRows = screen.getAllByRole('row').filter((r) => /v[12]/.test(r.textContent ?? ''))
      expect(bodyRows[0].textContent).toContain('v2')
      expect(bodyRows[1].textContent).toContain('v1')
    })

    it('현재 버전에는 롤백 버튼을 두지 않는다', () => {
      mockUseStackHistory.mockReturnValue({ data: twoVersions })

      renderWithProviders(<StackHistoryPage />)

      const rows = screen.getAllByRole('row')
      const currentRow = rows.find((r) => /CURRENT|현재/.test(r.textContent ?? ''))!
      const olderRow = rows.find((r) => (r.textContent ?? '').includes('v1'))!

      expect(/Rollback|롤백/.test(currentRow.textContent ?? '')).toBe(false)
      expect(/Rollback|롤백/.test(olderRow.textContent ?? '')).toBe(true)
    })
  })

  // 설정 스냅샷은 중첩 객체다. String() 으로 찍으면 [object Object] 만 나온다.
  it('중첩된 설정도 값까지 펼쳐 보여준다', () => {
    mockUseStackHistory.mockReturnValue({
      data: [
        {
          id: 'h-1',
          stackId: 'stack-1',
          version: 1,
          changedBy: 'system',
          changedAt: '2026-01-01T10:00:00Z',
          reason: 'stack created',
          snapshot: {
            access_domain: 'cicdtest.internal',
            pipeline: { cd_tool: { name: 'argocd', enabled: true } },
          },
        },
      ],
    })

    renderWithProviders(<StackHistoryPage />)
    // 스냅샷 패널은 행을 골라야 열린다.
    fireEvent.click(screen.getByText('stack created'))

    expect(screen.queryByText(/\[object Object\]/)).toBeNull()
    expect(screen.getByText('pipeline.cd_tool.name')).toBeTruthy()
    expect(screen.getByText('argocd')).toBeTruthy()
    expect(screen.getByText('access_domain')).toBeTruthy()
  })

  it('renders empty state when no history entries exist', () => {
    mockUseStackHistory.mockReturnValue({ data: [] })

    renderWithProviders(<StackHistoryPage />)

    expect(screen.getByText(/No data available\.|데이터가 없습니다\.|dataTable.empty/)).not.toBeNull()
  })

  it('keeps the route stack id when list data is stale', () => {
    mockUseParams.mockReturnValue({ stackId: 'stack-new' })
    mockUseStacks.mockReturnValue({ data: { items: stacks } })

    renderWithProviders(<StackHistoryPage />)

    expect(mockUseStackHistory).toHaveBeenCalledWith('stack-new')
    expect(mockNavigate).not.toHaveBeenCalledWith('/stack/history/stack-1', { replace: true })
    // 목록에 없는 라우트 id 라도 셀렉트가 그 값을 그대로 보여 준다.
    expect(selectedLabel(getSelects()[0])).toBe('stack-new')
  })

  it('filters stack selector by cluster filter', () => {
    mockUseStacks.mockReturnValue({
      data: {
        items: [
          ...stacks,
          {
            id: 'stack-2',
            name: 'Dev Stack',
            templateId: 'tpl-2',
            templateName: 'GitLab + Argo',
            clusterId: 'cluster-2',
            clusterName: 'dev',
            status: 'completed',
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-01T00:00:00Z',
          },
        ],
      },
    })

    renderWithProviders(<StackHistoryPage />)

    // [0] 스택 셀렉터, [1] 표 도구줄의 클러스터 필터
    selectOption(getSelects()[1], 'prod')

    expect(optionLabels(getSelects()[0])).toEqual(['Platform Stack'])
  })

})
