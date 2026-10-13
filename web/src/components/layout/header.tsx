import { type ReactNode } from 'react'
import { GraduationCap, HardHat, LaptopMinimal, MessageSquareText, Moon, ShieldCheck, Sun } from 'lucide-react'
import { iconProps } from '../ui/icon'
import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../stores/theme-store'
import { useAuthStore } from '../../stores/auth-store'
import { useTourStore } from '../../stores/tour-store'
import type { Role } from '../../types'
import { LanguageSwitcher } from '../shared/language-switcher'
import { IconButton, iconButtonClassName } from '../ui/icon-button'
import { JobNotificationBell } from '../../features/common/components/job-notification-bell'

// 인앱 피드백 위젯 대신 Nullus 저장소의 이슈 페이지로 보낸다(nullus-plan#67 협의) —
// 받는 곳 · 스크린샷 속 민감정보 · airgap 저장을 따로 만들지 않고 이슈 템플릿에 맡긴다.
export const FEEDBACK_URL = 'https://github.com/cloud-nullus/nullus/issues'

const roleIcons: Record<Role, ReactNode> = {
  admin: <ShieldCheck {...iconProps('sm')} />,
  devops: <HardHat {...iconProps('sm')} />,
  developer: <LaptopMinimal {...iconProps('sm')} />,
}

const roleLabels: Record<Role, string> = {
  admin: 'header.roles.admin',
  devops: 'header.roles.devops',
  developer: 'header.roles.developer',
}

export function Header() {
  const { i18n, t } = useTranslation()
  const { theme, toggleTheme } = useThemeStore()
  const { role, isAuthenticated } = useAuthStore()
  const startTour = useTourStore((state) => state.start)

  const handleLanguageChange = (language: string) => {
    void i18n.changeLanguage(language)
  }

  return (
    <header className="flex h-[var(--header-height)] shrink-0 items-center justify-end gap-3 border-b border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-[var(--page-padding)]">
      {/* Role badge */}
      <div className="flex items-center gap-1.5 rounded-[var(--radius-full)] bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] px-2 py-0.5 text-[11px] font-semibold text-[var(--color-primary)]">
        {roleIcons[role]}
        {t(roleLabels[role])}
      </div>

      {/* 로그인 전에는 걸지 않는다. 종은 목록 API 두 개를 폴링하므로, 인증이
          없는 동안 붙어 있으면 401 만 반복해서 쌓인다. */}
      {isAuthenticated && <JobNotificationBell />}

      <LanguageSwitcher currentLanguage={i18n.language} onLanguageChange={handleLanguageChange} />

      {/* 둘러보기. 역할을 넘기는 이유는 걸음 목록이 역할마다 다르기 때문이다 —
          developer 를 관리자 전용 화면으로 안내하면 ProtectedRoute 가 되돌려
          보내고 그 걸음은 빈 화면을 강조하게 된다. */}
      <IconButton onClick={() => startTour(role)} aria-label={t('tour.start', 'Tutorial')}>
        <GraduationCap {...iconProps('sm')} />
      </IconButton>

      {/* 피드백. 다른 사이트로 보내므로 버튼이 아니라 링크다. noopener 는 새 탭이
          window.opener 로 이 화면을 건드리지 못하게, noreferrer 는 내부 경로가
          Referer 로 GitHub 에 새지 않게 한다. */}
      <a
        href={FEEDBACK_URL}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={t('header.feedback')}
        title={t('header.feedbackHint')}
        className={iconButtonClassName()}
      >
        <MessageSquareText {...iconProps('sm')} />
      </a>

      {/* Theme toggle */}
      <IconButton
        onClick={toggleTheme}
        aria-label={theme === 'dark' ? t('header.theme.switchToLight') : t('header.theme.switchToDark')}
      >
        {theme === 'dark' ? <Sun {...iconProps('sm')} /> : <Moon {...iconProps('sm')} />}
      </IconButton>
    </header>
  )
}
