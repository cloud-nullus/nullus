import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../__tests__/test-utils'
import { TourOverlay } from './tour-overlay'
import { useTourStore } from '../../stores/tour-store'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

const NEXT = /Next|다음/
const PREV = /Back|이전/
const CLOSE = /End tour|투어 종료/

beforeEach(() => {
  useTourStore.getState().stop()
  mockNavigate.mockClear()
  document.body.innerHTML = ''
})

/**
 * 투어가 가리키는 대상을 실제 DOM 에 세운다.
 *
 * jsdom 은 레이아웃을 계산하지 않아 모든 요소가 0×0 이다. 오버레이는 크기가
 * 0 인 것을 "없는 것" 으로 치므로(조건부로 그려지는 빈 영역을 강조하지 않기
 * 위해서다) 여기서 크기를 심어 준다.
 */
function withSize<T extends HTMLElement>(element: T, width = 120, height = 40): T {
  element.getBoundingClientRect = () =>
    ({ top: 100, left: 100, width, height, right: 100 + width, bottom: 100 + height, x: 100, y: 100, toJSON: () => ({}) }) as DOMRect
  return element
}

function mountTarget(tourId: string) {
  const el = withSize(document.createElement('button'))
  el.setAttribute('data-tour', tourId)
  el.textContent = tourId
  document.body.appendChild(el)
  return el
}

describe('TourOverlay', () => {
  it('투어가 꺼져 있으면 아무것도 그리지 않는다', () => {
    const { container } = renderWithProviders(<TourOverlay />)
    expect(container).toBeEmptyDOMElement()
  })

  it('자기 자신을 강조 대상으로 잡지 않는다', () => {
    // 설명 카드도 role="dialog" 다. 걸음이 팝업을 가리킬 때 이 카드가 먼저
    // 걸리면, 강조 위치가 카드를 움직이고 그 카드가 다시 강조 위치를 바꿔
    // 화면이 요동친다(실제로 4·6·7 걸음에서 그랬다).
    useTourStore.getState().start('admin')
    useTourStore.getState().goTo(3) // clusterForm — 대상이 앱 팝업이다
    renderWithProviders(<TourOverlay />)

    // 앱 팝업이 없으므로 강조 구멍도 없어야 한다(가운데 설명만).
    expect(screen.queryByTestId('tour-spotlight')).not.toBeInTheDocument()
  })

  it('탭 걸음은 탭과 그 안의 영역을 함께 감싼다', () => {
    // 탭 버튼만 강조하면 "이 탭에서 무엇을 고르는지" 가 화면에서 잘려 나간다.
    const tab = withSize(document.createElement('button'))
    tab.setAttribute('data-tab', 'artifacts')
    const panel = withSize(document.createElement('div'), 400, 200)
    panel.setAttribute('data-tour', 'install-panel')
    document.body.append(tab, panel)

    useTourStore.getState().start('admin')
    const index = useTourStore.getState().steps.findIndex((step) => step.id === 'installArtifacts')
    useTourStore.getState().goTo(index)
    renderWithProviders(<TourOverlay />)

    expect(screen.getByTestId('tour-spotlight')).toBeInTheDocument()
  })

  it('탭 걸음은 대상이 이미 보여도 눌러서 탭을 옮긴다', () => {
    // 탭 버튼은 어느 탭이 열려 있든 늘 화면에 있다. "대상이 보이면 건너뛴다" 로
    // 두었더니 탭이 영영 바뀌지 않고 Artifacts 에 멈춰 있었다.
    const tab = withSize(document.createElement('button'))
    tab.setAttribute('data-tab', 'storage')
    const clicks = vi.fn()
    tab.addEventListener('click', clicks)
    document.body.appendChild(tab)

    useTourStore.getState().start('admin')
    const index = useTourStore.getState().steps.findIndex((step) => step.id === 'installStorage')
    useTourStore.getState().goTo(index)
    renderWithProviders(<TourOverlay />)

    expect(clicks).toHaveBeenCalled()
  })

  it('팝업 걸음은 이미 열려 있으면 다시 누르지 않는다', () => {
    // 한 번 더 누르면 팝업이 닫힌다.
    const trigger = withSize(document.createElement('button'))
    trigger.setAttribute('data-tour', 'register-cluster')
    const clicks = vi.fn()
    trigger.addEventListener('click', clicks)
    const dialog = withSize(document.createElement('div'), 500, 400)
    dialog.setAttribute('data-modal', '')
    document.body.append(trigger, dialog)

    useTourStore.getState().start('admin')
    useTourStore.getState().goTo(3) // clusterForm
    renderWithProviders(<TourOverlay />)

    expect(clicks).not.toHaveBeenCalled()
  })

  it('크기가 0 인 영역은 없는 것으로 친다', () => {
    // Test·Security 섹션은 그 기능을 켜야 생긴다. 선택자에는 걸리지만 실체가
    // 없어, 그대로 두면 한 줄짜리 빈 조각을 강조하게 된다(실제로 그랬다).
    const empty = document.createElement('div')
    empty.setAttribute('data-tab', 'artifacts') // 크기를 심지 않는다 → 0×0
    document.body.appendChild(empty)

    useTourStore.getState().start('admin')
    const index = useTourStore.getState().steps.findIndex((step) => step.id === 'installArtifacts')
    useTourStore.getState().goTo(index)
    renderWithProviders(<TourOverlay />)

    expect(screen.queryByTestId('tour-spotlight')).not.toBeInTheDocument()
  })

  it('화면을 거의 다 덮는 합집합은 쓰지 않는다', () => {
    // 강조가 화면 전체면 아무것도 가리키지 않는 것과 같다.
    const tab = withSize(document.createElement('button'))
    tab.setAttribute('data-tab', 'artifacts')
    const huge = withSize(document.createElement('div'), 5000, 5000)
    huge.setAttribute('data-tour', 'install-panel')
    document.body.append(tab, huge)

    useTourStore.getState().start('admin')
    const index = useTourStore.getState().steps.findIndex((step) => step.id === 'installArtifacts')
    useTourStore.getState().goTo(index)
    renderWithProviders(<TourOverlay />)

    const spotlight = screen.getByTestId('tour-spotlight')
    // 탭 크기(120×40 + 여백)만 남아야 한다.
    expect(Number.parseInt(spotlight.style.width, 10)).toBeLessThan(200)
  })

  it('현재 걸음의 제목과 설명을 보여 준다', () => {
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    expect(screen.getByRole('dialog', { name: /tour|둘러보기/i })).toBeInTheDocument()
    expect(screen.getByText(/1 \/ \d+/)).toBeInTheDocument()
  })

  it('우측 하단의 다음·이전 버튼으로 걸음을 넘긴다', () => {
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    fireEvent.click(screen.getByRole('button', { name: NEXT }))
    expect(useTourStore.getState().stepIndex).toBe(1)

    fireEvent.click(screen.getByRole('button', { name: PREV }))
    expect(useTourStore.getState().stepIndex).toBe(0)
  })

  it('강조된 요소를 누르면 다음 걸음으로 넘어간다', () => {
    // 설명만 읽고 넘기는 것이 아니라 실제로 눌러 보게 하는 것이 이 투어의 요점이다.
    const target = mountTarget('hero-cta')
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    fireEvent.click(target)

    expect(useTourStore.getState().stepIndex).toBe(1)
  })

  it('투어가 스스로 누른 클릭으로는 넘어가지 않는다', () => {
    // 탭을 열려고 투어가 누른 것까지 세면, 보여 주려던 화면을 아무도 보지 못한
    // 채 걸음이 지나간다(실제로 워크로드·모니터링 걸음이 그렇게 건너뛰어졌다).
    const tab = withSize(document.createElement('button'))
    tab.setAttribute('data-tab', 'storage')
    document.body.appendChild(tab)

    useTourStore.getState().start('admin')
    const index = useTourStore.getState().steps.findIndex((step) => step.id === 'installStorage')
    useTourStore.getState().goTo(index)
    renderWithProviders(<TourOverlay />)

    // 이 걸음은 대상(탭)을 스스로 누른다. 그 클릭으로 걸음이 넘어가면 안 된다.
    expect(useTourStore.getState().stepIndex).toBe(index)
  })

  it('걸음이 다른 화면이면 그리로 옮긴다', () => {
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    // 처음 두 걸음은 홈이라 옮기지 않는다. 세 번째(클러스터 등록)에서 옮긴다.
    mockNavigate.mockClear()
    fireEvent.click(screen.getByRole('button', { name: NEXT }))
    fireEvent.click(screen.getByRole('button', { name: NEXT }))

    expect(useTourStore.getState().stepIndex).toBe(2)
    expect(mockNavigate).toHaveBeenCalledWith('/admin/clusters')
  })

  it('닫기로 투어를 끝낸다', () => {
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    fireEvent.click(screen.getByRole('button', { name: CLOSE }))

    expect(useTourStore.getState().isActive).toBe(false)
  })

  it('Escape 로도 끝난다', () => {
    useTourStore.getState().start('admin')
    renderWithProviders(<TourOverlay />)

    fireEvent.keyDown(window, { key: 'Escape' })

    expect(useTourStore.getState().isActive).toBe(false)
  })
})
