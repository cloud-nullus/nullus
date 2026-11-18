import { useTranslation } from 'react-i18next'
import { ToolMark } from '../../stack/components/tool-mark'
import { SUPPORT_TOOLS, supportToolCategoryKey, type SupportTool } from '../utils/support-tools'

/**
 * 지원 OSS 를 왼쪽으로 흘려 보여 주는 띠.
 *
 * 같은 목록을 두 벌 이어 붙이고 트랙을 절반(-50%)만 민다. 한 바퀴가 끝나는
 * 순간의 화면이 시작과 같아지므로 이음매가 보이지 않는다 — 카드 폭이 제품명
 * 길이마다 달라서, 개수나 픽셀로 계산하는 방식은 여기서 쓸 수 없다.
 *
 * 애니메이션·마스크·정지 규칙은 index.css 의 `.nullus-marquee*` 에 있다.
 * 움직임을 줄이는 설정에서는 애니메이션을 끄고 가로 스크롤로 바꾼다.
 */
export function SupportToolsMarquee() {
  const { t } = useTranslation()

  return (
    <section className="mb-8" aria-labelledby="support-tools-heading">
      <h2 id="support-tools-heading" className="mb-1 mt-0 text-lg font-bold text-[var(--color-text-primary)]">
        {t('home.supportTools.title', 'Support Tools')}
      </h2>
      <p className="mb-4 mt-0 text-xs text-[var(--color-text-secondary)]">
        {t('home.supportTools.description', 'Tools Nullus installs into your cluster or connects to, and wires together for you.')}
      </p>

      <div className="nullus-marquee">
        <div className="nullus-marquee-track">
          <ToolRow label={t('home.supportTools.title', 'Support Tools')} />
          {/* 이음매를 메우기 위한 두 번째 벌. 읽는 사람에게는 같은 목록을 두 번
              읽어 주는 것일 뿐이라 보조기술에서는 숨긴다. */}
          <ToolRow aria-hidden />
        </div>
      </div>
      {/* 이름을 글자로 적는 것은 사실 표시다. 제휴·보증 관계가 아님을 함께 밝힌다. */}
      <p className="mb-0 mt-2 text-[11px] text-[var(--color-text-muted)]">
        {t('home.supportTools.trademarkNotice')}
      </p>
    </section>
  )
}

function ToolRow({ label, 'aria-hidden': ariaHidden }: { label?: string; 'aria-hidden'?: boolean }) {
  return (
    <ul
      className="m-0 flex shrink-0 list-none gap-2.5 pl-0 pr-2.5"
      aria-label={label}
      aria-hidden={ariaHidden}
    >
      {SUPPORT_TOOLS.map((tool) => (
        <ToolCard key={`${tool.name}-${ariaHidden ? 'echo' : 'lead'}`} tool={tool} />
      ))}
    </ul>
  )
}

function ToolCard({ tool }: { tool: SupportTool }) {
  const { t } = useTranslation()

  return (
    <li className="flex shrink-0 items-center gap-3 whitespace-nowrap rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-4 py-3">
      {/* 제품 로고가 아니라 글자 약자다 — 모든 카드가 같은 타일·같은 색을 쓴다. */}
      <ToolMark name={tool.name} size="lg" />
      <span>
        <span className="block text-sm font-bold text-[var(--color-text-primary)]">{tool.name}</span>
        <span className="block text-[11px] text-[var(--color-text-muted)]">{t(supportToolCategoryKey(tool.category))}</span>
      </span>
    </li>
  )
}
