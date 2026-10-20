// 커밋된 비밀은 이미 유출됐다는 경고.
//
// 시크릿 탐지는 다른 스캔과 조치가 다르다 — 취약점은 고치면 끝나지만 커밋에 들어간 비밀은
// 커밋을 지워도 이미 유출이다(Secret Detection 설계 §5). 그래서 탐지가 있으면 판정이 경고
// (warn)여도 이 줄을 먼저 보인다. CI/CD 실행 기록의 상세와 보안 대시보드의 상세 패널이 같은
// 문구를 써야 하므로 shared 에 둔다(관측 기능이 cicd 기능 폴더를 import 하지 않게).

import { useTranslation } from 'react-i18next'
import { StatusIcon } from '../ui/status-icon'
import { cn } from '../../lib/utils'

export function SecretRotateWarning({ className }: { className?: string }) {
  const { t } = useTranslation()
  return (
    <p
      role="alert"
      className={cn(
        'm-0 flex items-start gap-1.5 rounded-md border border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_10%,_transparent)] px-2 py-1.5 text-[12px] font-medium text-[var(--color-error)]',
        className,
      )}
    >
      <StatusIcon tone="warning" size="xs" inheritColor className="mt-0.5 shrink-0" />
      {t('cicdListPage.secretScan.rotateWarning')}
    </p>
  )
}
