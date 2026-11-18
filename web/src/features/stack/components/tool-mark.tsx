import type { IconSize } from "../../../components/ui/icon";
import { icon } from "../../../theme/tokens.generated";
import { toolMonogram } from "../utils/tool-identity";

/**
 * 타일 한 변은 아이콘 크기 단계를 그대로 따른다(sm 16 · md 20 · lg 28 — DESIGN.md §아이콘).
 * 로고가 있던 자리를 같은 크기로 채우기 위해서다. 글자 크기만 여기서 정한다.
 */
const FONT_SIZE: Record<Exclude<IconSize, "xs">, number> = {
  sm: 8,
  md: 9,
  lg: 11,
};

/**
 * 도구 옆에 붙는 표식 — 브랜드 로고가 아니라 두 글자 약자를 담은 타일이다.
 *
 * 왜 로고를 쓰지 않는지는 tool-identity.ts 의 toolMonogram 에 적었다. 메인 화면,
 * 스택 콘솔 버튼·워크로드·파이프라인 토폴로지, 모니터링이 모두 이 컴포넌트를 거친다 —
 * 같은 도구는 어느 화면에서든 같은 약자, 같은 모양이다. 제품 고유의 색·글꼴을 쓰지
 * 않고 중립색 한 벌만 쓴다.
 *
 * 옆에 이름이 늘 함께 있으므로 장식으로 숨긴다. 약자만 홀로 서는 자리(모니터링 막대
 * 아래)는 감싸는 쪽이 이름을 단다.
 */
export function ToolMark({ name, size = "sm" }: { name: string; size?: Exclude<IconSize, "xs"> }) {
  const monogram = toolMonogram(name);
  const side = icon[size].size;
  return (
    <span
      aria-hidden="true"
      data-tool-mark={monogram}
      className="inline-flex shrink-0 select-none items-center justify-center rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] font-bold leading-none tracking-[-0.02em] text-[var(--color-text-secondary)]"
      style={{ width: side, height: side, fontSize: FONT_SIZE[size] }}
    >
      {monogram}
    </span>
  );
}
