// 표의 표면 스타일. DataTable 과 손으로 짠 표가 같은 값을 쓰게 하는 곳이다.
//
// 이 앱의 표는 두 종류다. 정렬·페이지네이션·전역 검색이 필요한 목록은
// `DataTable`(TanStack)이고, 그럴 게 없는 정적 표(알려진 이슈, 토큰 목록,
// 호환성 매트릭스 등)는 그냥 <table> 이다. 후자를 DataTable 로 억지로 옮기면
// 컬럼 정의와 셀 렌더러만 늘고 얻는 게 없다 — 그래서 구조는 그대로 두고
// 표면만 하나로 모은다.
//
// 개편 전에는 후자 14곳이 각자 헤더 마크업을 복사하고 있었고, 그 과정에서
// DataTable 과 어긋났다:
//   - 헤더 배경: 손으로 짠 표는 bg-[color-mix(... text-primary 2% ...)],
//     DataTable 은 --color-surface-sunken. 한 화면 안에서 두 표가 위아래로
//     놓이면 헤더 색이 다르게 보였다
//   - 셀 여백: px-3.5 / px-[14px] / px-4 세 가지
//   - 행 높이: 지정 없음(내용에 따라 들쭉날쭉)
//
// 값은 DESIGN.md §Layout 의 --table-* 토큰에서 온다.

/** `<thead>` 안의 `<tr>` 에 붙인다. */
export const tableHeadRowClass = 'bg-[var(--color-surface-sunken)]'

/** `<th>` 에 붙인다. */
export const thClass =
  'h-[var(--table-header-height)] whitespace-nowrap px-[var(--table-cell-px)] text-left ' +
  'text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]'

/** `<td>` 에 붙인다. 행 구분선은 위쪽 테두리로 그린다 (DataTable 과 같은 방식). */
export const tdClass =
  'h-[var(--table-row-height)] border-t border-[var(--color-border-default)] ' +
  'px-[var(--table-cell-px)] py-1 text-[13px] text-[var(--color-text-primary)]'
