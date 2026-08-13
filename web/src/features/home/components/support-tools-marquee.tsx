import { useTranslation } from 'react-i18next'
import { SUPPORT_TOOLS, supportToolIconURL, type SupportTool } from '../utils/support-tools'

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
        {t('home.supportTools.description', 'Open source that Nullus installs and wires into your cluster.')}
      </p>

      <div className="nullus-marquee">
        <div className="nullus-marquee-track">
          <ToolRow label={t('home.supportTools.title', 'Support Tools')} />
          {/* 이음매를 메우기 위한 두 번째 벌. 읽는 사람에게는 같은 목록을 두 번
              읽어 주는 것일 뿐이라 보조기술에서는 숨긴다. */}
          <ToolRow aria-hidden />
        </div>
      </div>
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
      <img
        src={supportToolIconURL(tool.icon)}
        alt=""
        aria-hidden="true"
        className={`h-7 w-7 shrink-0 object-contain${tool.monochromeWhite ? ' nullus-mono-mark' : ''}`}
      />
      <span>
        <span className="block text-sm font-bold text-[var(--color-text-primary)]">{tool.name}</span>
        <span className="block text-[11px] text-[var(--color-text-muted)]">{t(tool.categoryKey)}</span>
      </span>
    </li>
  )
}
