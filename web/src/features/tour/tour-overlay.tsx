import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'

import { iconProps } from '../../components/ui/icon'
import { useTourStore } from '../../stores/tour-store'
import { clearTourData } from './tour-mock-adapter'
import type { TourStep } from './tour-steps'

/** 강조 구멍 둘레의 여백. 버튼이 테두리에 딱 붙으면 무엇을 가리키는지 흐려진다. */
const SPOTLIGHT_PADDING = 8
/** 설명 상자 너비. 스타일과 같아야 한다. */
const CARD_WIDTH = 320
/** 아직 재기 전의 높이. 첫 프레임에만 쓰이고 곧 실제 값으로 바뀐다. */
const CARD_FALLBACK_HEIGHT = 150
const CARD_GAP = 14

interface Rect {
  top: number
  left: number
  width: number
  height: number
}

/** 투어가 그린 것들. 대상 탐색에서 반드시 제외한다. */
const OVERLAY_ROOT = '[data-testid="tour-overlay"]'

/**
 * 지금 투어가 스스로 누르는 중인가.
 *
 * 탭을 열려고 투어가 누른 클릭까지 "사용자가 눌렀다" 로 세면 그 걸음이 곧바로
 * 다음으로 넘어가, 보여 주려던 화면을 아무도 보지 못한 채 지나간다(실제로
 * 워크로드·모니터링 걸음이 그렇게 건너뛰어졌다).
 *
 * click() 은 동기로 처리되므로 그 사이에만 켜 두면 정확히 갈린다. isTrusted 를
 * 보지 않는 이유는 그 값을 테스트에서 만들 수 없어, 정작 이 구분을 고정하는
 * 테스트를 쓸 수 없기 때문이다.
 */
let tourIsClicking = false

function clickAsTour(element: HTMLElement) {
  tourIsClicking = true
  try {
    element.click()
  } finally {
    tourIsClicking = false
  }
}

/**
 * 화면에서 대상을 찾되 **투어 자신은 건너뛴다**.
 *
 * 설명 카드도 role="dialog" 다. 걸음이 앱 팝업을 가리킬 때 이 카드가 먼저
 * 걸리면 강조가 카드를 감싸고, 카드는 강조 위치를 따라 움직이며, 그 움직임이
 * 다시 강조를 옮긴다 — 화면이 요동친다. 실제로 그랬다.
 *
 * 크기가 0 인 것도 없는 것으로 친다. 조건부로 그려지는 영역(기능을 켜야 생기는
 * Test·Security 섹션)은 선택자에는 걸리지만 실체가 없어, 그대로 두면 한 줄짜리
 * 빈 조각을 강조하게 된다.
 */
function findTarget(selector: string): Element | null {
  for (const element of document.querySelectorAll(selector)) {
    if (element.closest(OVERLAY_ROOT)) continue
    const box = element.getBoundingClientRect()
    if (box.width > 0 && box.height > 0) return element
  }
  return null
}

/** 화면을 거의 다 덮는 강조는 아무것도 가리키지 않는 것과 같다. */
function coversViewport(rect: Rect): boolean {
  const viewportW = window.innerWidth || 1280
  const viewportH = window.innerHeight || 800
  return rect.width * rect.height > viewportW * viewportH * 0.85
}

/** 1px 미만의 차이는 같은 자리로 본다. 잰 값이 미세하게 흔들려도 다시 그리지 않는다. */
function sameRect(a: Rect | null, b: Rect): boolean {
  if (!a) return false
  return (
    Math.abs(a.top - b.top) < 1 &&
    Math.abs(a.left - b.left) < 1 &&
    Math.abs(a.width - b.width) < 1 &&
    Math.abs(a.height - b.height) < 1
  )
}

/** 두 사각형을 모두 담는 가장 작은 사각형. */
function mergeRects(a: Rect, b: Rect): Rect {
  const top = Math.min(a.top, b.top)
  const left = Math.min(a.left, b.left)
  return {
    top,
    left,
    width: Math.max(a.left + a.width, b.left + b.width) - left,
    height: Math.max(a.top + a.height, b.top + b.height) - top,
  }
}

/**
 * 대상의 화면 위 사각형을 추적한다.
 *
 * 스크롤·리사이즈로 움직이므로 한 번 재고 마는 것으로는 부족하다. 대상이 아직
 * 없을 수도 있다(화면 전환 직후, 데이터가 늦게 오는 목록) — 그때는 null 을
 * 돌려주고 화면 가운데에 설명만 띄운다.
 */
function useTargetRect(selector: string | undefined, union: string | undefined, stepId: string): Rect | null {
  const [rect, setRect] = useState<Rect | null>(null)

  useLayoutEffect(() => {
    if (!selector) {
      setRect(null)
      return
    }

    let frame = 0
    const measure = () => {
      const element = findTarget(selector)
      if (!element) {
        setRect(null)
        return
      }
      const box = element.getBoundingClientRect()
      let next: Rect = { top: box.top, left: box.left, width: box.width, height: box.height }

      // 탭처럼 "누르는 곳" 과 "보이는 곳" 이 갈린 걸음은 둘을 함께 감싼다.
      // 탭 버튼만 강조하면 그 탭에서 무엇을 고르는지가 화면에서 잘려 나간다.
      const extra = union ? findTarget(union) : null
      if (extra) {
        const extraBox = extra.getBoundingClientRect()
        const merged = mergeRects(next, {
          top: extraBox.top,
          left: extraBox.left,
          width: extraBox.width,
          height: extraBox.height,
        })
        // 합쳐서 화면을 거의 다 덮으면 강조가 아무 말도 하지 않는 것과 같다.
        // 그럴 때는 누르는 곳만 남긴다.
        if (!coversViewport(merged)) next = merged
      }
      setRect((current) => (sameRect(current, next) ? current : next))
    }

    measure()
    // 화면을 막 옮겨 온 직후에는 대상이 아직 안 그려져 있다. 몇 프레임 더 본다.
    const retry = window.setInterval(measure, 250)
    const onChange = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(measure)
    }
    window.addEventListener('scroll', onChange, true)
    window.addEventListener('resize', onChange)

    return () => {
      window.clearInterval(retry)
      cancelAnimationFrame(frame)
      window.removeEventListener('scroll', onChange, true)
      window.removeEventListener('resize', onChange)
    }
  }, [selector, union, stepId])

  return rect
}

/**
 * 걸음에 들어올 때 필요한 것을 눌러 둔다(팝업 열기, 탭 옮기기).
 *
 * 두 종류를 구분한다.
 *
 *   대상 = 누를 곳(탭)  — 늘 누른다. 탭 버튼은 어느 탭이 열려 있든 항상 화면에
 *                        있으므로 "대상이 보이면 건너뛴다" 로 두면 탭이 영영
 *                        바뀌지 않는다(실제로 Artifacts 에 멈춰 있었다).
 *                        이미 열린 탭을 다시 눌러도 무해하다.
 *   대상 ≠ 누를 곳(팝업) — 대상이 없을 때만 누른다. 열려 있는데 또 누르면 닫힌다.
 */
function useActivateStep(step: TourStep | undefined) {
  useEffect(() => {
    if (!step?.activate) return

    const chain = Array.isArray(step.activate) ? step.activate : [step.activate]
    // 마지막이 대상을 여는 손잡이다. 그것과 대상이 같으면(탭) 늘 누르고,
    // 다르면(팝업) 대상이 없을 때만 누른다 — 열려 있는데 또 누르면 닫힌다.
    const isToggle = step.target !== chain[chain.length - 1]
    let cancelled = false

    // 화면을 막 옮겨 온 직후에는 누를 것이 아직 없다. 하나씩, 나타나는 대로 누른다.
    const run = (position: number, attempt: number) => {
      if (cancelled || position >= chain.length) return
      if (isToggle && step.target && findTarget(step.target)) return

      const trigger = findTarget(chain[position]) as HTMLElement | null
      if (trigger) {
        clickAsTour(trigger)
        // 앞의 클릭이 다음 것을 화면에 올린다(행 선택 → 상세 탭). 한 박자 뒤에 본다.
        window.setTimeout(() => run(position + 1, 0), 180)
        return
      }
      if (attempt < 12) window.setTimeout(() => run(position, attempt + 1), 200)
    }

    run(0, 0)
    return () => {
      cancelled = true
    }
  }, [step])
}

/** 이 요소를 실제로 스크롤하는 조상. 없으면 문서 자체다. */
function scrollParent(element: Element): Element | null {
  let node = element.parentElement
  while (node) {
    const overflowY = window.getComputedStyle(node).overflowY
    if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight) {
      return node
    }
    node = node.parentElement
  }
  return null
}

/**
 * 대상을 스크롤 상자의 위쪽에 세운다.
 *
 * scrollIntoView 를 쓰지 않는다. 그 함수는 "어느 상자를 얼마나" 를 브라우저가
 * 정하는데, 탭처럼 누른 직후 본문 높이가 바뀌는 자리에서는 대상이 화면 위로
 * 밀려 올라가 강조가 잘린 채 남았다. 여기서는 얼마나 움직일지를 직접 계산한다.
 */
/**
 * 스크롤 상자 위에 붙어 있는 머리의 높이.
 *
 * PageHeader 는 sticky 라 스크롤과 무관하게 상자 맨 위를 덮는다. 그것을 빼지
 * 않고 대상을 "상자 맨 위" 로 올리면 대상이 머리 **밑에 깔린다** — 좌표상으로는
 * 보이는데 화면에는 없는, 찾기 어려운 상태가 된다.
 */
function stickyHeaderHeight(container: Element): number {
  const header = container.querySelector('[data-sticky-header]')
  return header ? Math.round(header.getBoundingClientRect().height) : 0
}

function ensureVisible(element: Element, topMargin: number, pinToTop: boolean) {
  const box = element.getBoundingClientRect()
  const container = scrollParent(element)

  // 즉시 옮긴다. 부드러운 스크롤은 애니메이션이 끝나기 전에 다음 보정이 위치를
  // 재면서 값이 겹쳐 과하게 밀렸다 — 탭이 화면 위로 사라지고 강조만 맨 위에
  // 잘린 채 남는 증상이 그것이었다.
  if (!container) {
    const delta = box.top - topMargin
    if (Math.abs(delta) > 8) window.scrollBy(0, delta)
    return
  }

  const containerBox = container.getBoundingClientRect()
  const safeTop = containerBox.top + stickyHeaderHeight(container) + topMargin
  const delta = box.top - safeTop
  const outOfView = box.top < safeTop || box.bottom > containerBox.bottom
  // 아래에 본문을 함께 보여 줄 걸음은 "보이기만 하면 된다" 로 부족하다. 탭이
  // 화면 맨 아래에 걸려 있어도 조건상 보이는 것이라, 정작 보여 주려던 본문이
  // 화면 밖으로 잘렸다.
  if ((pinToTop && Math.abs(delta) > 8) || outOfView) {
    container.scrollTop += delta
  }
}

/**
 * 강조할 것을 화면 안으로 끌어온다.
 *
 * 걸음이 가리키는 것이 스크롤 아래에 있으면 강조가 화면 밖에 그려져 아무것도
 * 보이지 않는다. 두 번 맞춘다 — 탭을 누른 직후에는 본문이 아직 안 그려져 높이가
 * 바뀌고, 그 뒤에 한 번 더 맞춰야 자리가 선다.
 */
function useScrollTargetIntoView(step: TourStep | undefined) {
  useEffect(() => {
    if (!step?.target) return

    let cancelled = false
    const tryScroll = (attempt: number) => {
      if (cancelled) return
      const element = findTarget(step.target as string)
      if (!element) {
        if (attempt < 12) window.setTimeout(() => tryScroll(attempt + 1), 200)
        return
      }
      // 함께 감쌀 본문이 있으면 대상을 위쪽에 붙인다 — 가운데로 맞추면 화면
      // 절반이 위에 낭비되고 정작 보여 주려던 본문이 아래로 잘린다.
      ensureVisible(element, step.union ? 16 : 120, Boolean(step.union))
    }

    const first = window.setTimeout(() => tryScroll(0), 150)
    const second = window.setTimeout(() => tryScroll(0), 900)
    return () => {
      cancelled = true
      window.clearTimeout(first)
      window.clearTimeout(second)
    }
  }, [step])
}

/**
 * 대상을 누르면 다음 걸음으로 넘긴다 — 설명만 읽히지 않고 실제로 눌러 보게 한다.
 *
 * 투어가 스스로 누른 것은 세지 않는다(tourIsClicking).
 */
function useAdvanceOnTargetClick(selector: string | undefined, stepId: string, onAdvance: () => void) {
  useEffect(() => {
    if (!selector) return
    const element = findTarget(selector)
    if (!element) return

    const handler = () => {
      if (!tourIsClicking) onAdvance()
    }
    element.addEventListener('click', handler)
    return () => element.removeEventListener('click', handler)
  }, [selector, stepId, onAdvance])
}

/** 강조 사각형을 화면 안으로 자른다. 여백은 테두리가 붙지 않을 만큼만 둔다. */
function clampToViewport(rect: Rect): Rect {
  const viewportW = window.innerWidth || 1280
  const viewportH = window.innerHeight || 800
  const top = Math.max(4, rect.top)
  const left = Math.max(4, rect.left)
  return {
    top,
    left,
    width: Math.max(0, Math.min(rect.left + rect.width, viewportW - 4) - left),
    height: Math.max(0, Math.min(rect.top + rect.height, viewportH - 4) - top),
  }
}

/** 설명 상자를 대상 옆에 놓는다. 화면 밖으로 나가면 반대쪽으로 뒤집는다. */
function cardPosition(
  rect: Rect | null,
  placement: TourStep['placement'],
  cardHeight: number,
): { top: number; left: number } {
  // 높이를 상수로 가정했더니 문구가 길어진 걸음에서 카드가 대상을 덮었다.
  // 실제로 그려진 높이를 받아서 계산한다.
  const CARD_ESTIMATED_HEIGHT = cardHeight
  const viewportW = window.innerWidth || 1280
  const viewportH = window.innerHeight || 800

  if (!rect) {
    return { top: Math.max(16, viewportH / 2 - CARD_ESTIMATED_HEIGHT / 2), left: Math.max(16, viewportW / 2 - CARD_WIDTH / 2) }
  }

  const clampLeft = (value: number) => Math.min(Math.max(16, value), Math.max(16, viewportW - CARD_WIDTH - 16))
  const clampTop = (value: number) => Math.min(Math.max(16, value), Math.max(16, viewportH - CARD_ESTIMATED_HEIGHT - 16))

  // 대상의 중심축에 맞춘다. 모서리에 맞추면 큰 대상(팝업·긴 패널)에서 설명이
  // 저 멀리 구석에 떨어져 무엇을 가리키는지 읽히지 않는다.
  const centerY = clampTop(rect.top + rect.height / 2 - CARD_ESTIMATED_HEIGHT / 2)
  const centerX = clampLeft(rect.left + rect.width / 2 - CARD_WIDTH / 2)

  switch (placement) {
    case 'top': {
      const above = rect.top - CARD_ESTIMATED_HEIGHT - CARD_GAP
      return above > 16
        ? { top: above, left: centerX }
        : { top: clampTop(rect.top + rect.height + CARD_GAP), left: centerX }
    }
    case 'left': {
      const leftOf = rect.left - CARD_WIDTH - CARD_GAP
      return leftOf > 16
        ? { top: centerY, left: leftOf }
        : { top: centerY, left: clampLeft(rect.left + rect.width + CARD_GAP) }
    }
    case 'right': {
      const rightOf = rect.left + rect.width + CARD_GAP
      return rightOf + CARD_WIDTH < viewportW - 16
        ? { top: centerY, left: rightOf }
        : { top: centerY, left: clampLeft(rect.left - CARD_WIDTH - CARD_GAP) }
    }
    default: {
      const below = rect.top + rect.height + CARD_GAP
      return below + CARD_ESTIMATED_HEIGHT < viewportH - 16
        ? { top: below, left: centerX }
        : { top: clampTop(rect.top - CARD_ESTIMATED_HEIGHT - CARD_GAP), left: centerX }
    }
  }
}

/**
 * 제품 둘러보기 오버레이.
 *
 * 어둡게 덮되 강조 대상만 뚫어 둔다. SVG 마스크 대신 사각형 넷(위·아래·왼·오른)을
 * 두르는 이유는 그 구멍이 **클릭을 그대로 통과시켜야** 하기 때문이다 — 마스크
 * 하나로 덮으면 대상이 가려져 "눌러 보세요" 라고 해 놓고 누를 수 없게 된다.
 */
export function TourOverlay() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const { isActive, stepIndex, steps, next, prev, stop } = useTourStore()

  const step = isActive ? steps[stepIndex] : undefined
  const rect = useTargetRect(step?.target, step?.union, step?.id ?? '')
  const advance = useCallback(() => next(), [next])
  useActivateStep(step)
  useScrollTargetIntoView(step)
  useAdvanceOnTargetClick(step?.target, step?.id ?? '', advance)

  // 걸음이 다른 화면을 가리키면 먼저 그리로 옮긴다.
  useEffect(() => {
    if (!step) return
    if (location.pathname === step.route) return
    navigate(step.route)
  }, [location.pathname, navigate, step])

  useEffect(() => {
    if (!isActive) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') stop()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [isActive, stop])

  // 투어가 끝나면 목업을 캐시에서 걷어 낸다. 남겨 두면 새로고침 전까지 가짜
  // 스택·파이프라인이 목록에 그대로 남아, 사용자는 자기 계정에 없는 것을
  // 있다고 믿게 된다.
  // MUI 는 모달을 열 때 body 의 다른 자식들에 aria-hidden 을 건다. 투어는 그
  // 모달을 설명하는 중이므로 보조기기에서 사라지면 안 된다 — 붙는 즉시 떼어낸다.
  const overlayRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const node = overlayRef.current
    if (!node) return
    const observer = new MutationObserver(() => {
      if (node.getAttribute('aria-hidden') === 'true') node.removeAttribute('aria-hidden')
    })
    observer.observe(node, { attributes: true, attributeFilter: ['aria-hidden'] })
    return () => observer.disconnect()
  }, [isActive])

  // 카드의 실제 높이. 걸음마다 문구 길이가 달라 자리 계산이 달라진다.
  const cardRef = useRef<HTMLDivElement>(null)
  const [cardHeight, setCardHeight] = useState(CARD_FALLBACK_HEIGHT)
  useLayoutEffect(() => {
    const node = cardRef.current
    if (!node) return
    const measure = () => setCardHeight(Math.round(node.getBoundingClientRect().height) || CARD_FALLBACK_HEIGHT)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [stepIndex, isActive])

  const wasActive = useRef(false)
  useEffect(() => {
    if (wasActive.current && !isActive) clearTourData(queryClient)
    wasActive.current = isActive
  }, [isActive, queryClient])

  if (!step) return null

  // 화면보다 큰 대상(탭 + 긴 본문)을 그대로 두면 강조가 화면 밖으로 흘러 테두리가
  // 잘린 채 보인다. 보이는 만큼으로 자른다.
  const hole = rect
    ? clampToViewport({
        top: rect.top - SPOTLIGHT_PADDING,
        left: rect.left - SPOTLIGHT_PADDING,
        width: rect.width + SPOTLIGHT_PADDING * 2,
        height: rect.height + SPOTLIGHT_PADDING * 2,
      })
    : null
  const card = cardPosition(rect, step.placement, cardHeight)
  // 막은 --color-surface-overlay 로 만든다. 글자색으로 만들면 테마가 뒤집힐 때
  // 막도 같이 뒤집혀 다크에서 흰 막이 된다(surface-tokens.test.ts 가 그 규칙을
  // 지킨다). 이 토큰은 두 테마 모두에서 어두운 값으로 정의돼 있다.
  // 걸음이 바뀔 때 순간이동하면 어디로 옮겨 갔는지 눈이 따라가지 못한다. 눈이
  // 좇는 것 — 테두리와 설명 상자 — 만 이어서 움직인다. 움직임을 줄여 달라고 한
  // 사람에게는 그대로 순간이동한다(motion-safe).
  //
  // **배경막은 애니메이션하지 않는다.** 자리를 250ms 마다 다시 재는데 전환이
  // 300ms 면 배경막이 영원히 뒤따라오며 구멍을 덮는다 — 강조된 것을 누를 수
  // 없게 된다(Playwright 가 "backdrop intercepts pointer events" 로 잡았다).
  const glide = 'motion-safe:transition-all motion-safe:duration-300 motion-safe:ease-out'
  const backdrop = 'fixed bg-[var(--color-surface-overlay)]'
  const isLast = stepIndex === steps.length - 1

  // 앱 껍데기 안이 아니라 body 로 포털한다. 그리고 MUI 모달(z-index 1300)보다
  // 위에 둔다 — 그러지 않으면 팝업을 설명하는 걸음에서 투어가 팝업 뒤로 숨는다.
  return createPortal(
    <div className="fixed inset-0 z-[1400]" data-testid="tour-overlay" ref={overlayRef}>
      {hole ? (
        <>
          {/* 구멍 둘레 넷. 가운데는 비워 두어 클릭이 대상까지 간다. */}
          <div className={backdrop} style={{ top: 0, left: 0, right: 0, height: Math.max(0, hole.top) }} />
          <div className={backdrop} style={{ top: hole.top + hole.height, left: 0, right: 0, bottom: 0 }} />
          <div className={backdrop} style={{ top: hole.top, left: 0, width: Math.max(0, hole.left), height: hole.height }} />
          <div className={backdrop} style={{ top: hole.top, left: hole.left + hole.width, right: 0, height: hole.height }} />
          <div
            aria-hidden
            data-testid="tour-spotlight"
            className={`pointer-events-none fixed rounded-[10px] ring-2 ring-[var(--color-brand-gold)] ${glide} shadow-[0_0_0_4px_color-mix(in_srgb,_var(--color-brand-gold)_25%,_transparent)]`}
            style={{ top: hole.top, left: hole.left, width: hole.width, height: hole.height }}
          />
        </>
      ) : (
        <div className={`${backdrop} inset-0`} />
      )}

      <div
        ref={cardRef}
        role="dialog"
        aria-modal="false"
        aria-label={t('tour.title', 'Product tour')}
        className={`fixed w-[320px] rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4 shadow-[var(--shadow-overlay)] ${glide}`}
        style={{ top: card.top, left: card.left }}
      >
        <div className="mb-1 text-[11px] font-semibold text-[var(--color-brand-gold)]">
          {stepIndex + 1} / {steps.length}
        </div>
        <h3 className="m-0 mb-1.5 text-sm font-bold text-[var(--color-text-primary)]">
          {t(`tour.steps.${step.id}.title`, step.id)}
        </h3>
        <p className="m-0 text-xs leading-5 text-[var(--color-text-secondary)]">
          {t(`tour.steps.${step.id}.body`, '')}
        </p>
      </div>

      {/* 조작은 우측 하단 한 곳에 모은다. 설명 상자가 화면을 돌아다녀도 버튼
          자리는 고정이라 눈이 따라다니지 않는다. 단축키 배지(bottom-4) 위에 둔다. */}
      <div className="fixed bottom-16 right-4 flex items-center gap-2 rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3 py-2 shadow-[var(--shadow-overlay)]">
        <button
          type="button"
          onClick={stop}
          data-testid="tour-end"
          className="cursor-pointer rounded-lg border-none bg-transparent px-2 py-1 text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        >
          <X {...iconProps('xs')} className="mr-1 inline" />
          {t('tour.controls.end', 'End tour')}
        </button>
        <button
          type="button"
          onClick={prev}
          data-testid="tour-prev"
          disabled={stepIndex === 0}
          className="inline-flex cursor-pointer items-center gap-1 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-2.5 py-1.5 text-xs font-semibold text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
        >
          <ChevronLeft {...iconProps('xs')} />
          {t('tour.controls.prev', 'Back')}
        </button>
        <button
          type="button"
          onClick={next}
          data-testid="tour-next"
          className="inline-flex cursor-pointer items-center gap-1 rounded-lg border-none bg-[var(--color-primary)] px-2.5 py-1.5 text-xs font-bold text-[var(--color-surface-base)]"
        >
          {isLast ? t('tour.controls.finish', 'Finish') : t('tour.controls.next', 'Next')}
          <ChevronRight {...iconProps('xs')} />
        </button>
      </div>
    </div>,
    document.body,
  )
}
