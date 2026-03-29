import { useNavigate } from 'react-router-dom'
import {
  BookOpen,
  Box,
  ChartNoAxesColumn,
  Code2,
  Cog,
  Coins,
  FlaskConical,
  Hammer,
  Rocket,
  ShieldCheck,
  Users,
} from 'lucide-react'
import { useAuthStore } from '../../../stores/auth-store'
import type { Role } from '../../../types'

const features = [
  {
    title: 'DevSecOps Stack 자동 설치',
    description: 'GitLab, ArgoCD, Prometheus 스택을 UI에서 바로 Kubernetes에 배포합니다.',
    icon: Box,
    iconClassName: 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]',
  },
  {
    title: 'Golden Path 템플릿',
    description: '검증된 조합(GitHub + ArgoCD, GitLab All-in-One)을 템플릿으로 제공합니다.',
    icon: BookOpen,
    iconClassName: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]',
  },
  {
    title: 'CI/CD Pipeline 관리',
    description: 'Web/API/Batch 템플릿으로 파이프라인 생성과 배포 이력을 관리합니다.',
    icon: Code2,
    iconClassName: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',
  },
  {
    title: '버전 호환성 보장',
    description: '검증된 도구 버전 조합만 노출해 예측 불가능한 호환성 이슈를 줄입니다.',
    icon: ShieldCheck,
    iconClassName: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  },
  {
    title: '통합 모니터링',
    description: '클러스터, 파이프라인, 애플리케이션 상태를 하나의 대시보드로 확인합니다.',
    icon: ChartNoAxesColumn,
    iconClassName: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  },
  {
    title: 'Role 기반 권한 관리',
    description: 'Admin, DevOps, Developer 역할별 기능 접근을 제어합니다.',
    icon: Users,
    iconClassName: 'bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]',
  },
]

const roadmap = [
  {
    phase: 'Phase 1 - DevOps',
    period: 'v0.1 · 2026 Q2',
    description: 'DevSecOps Stack 자동 설치, CI/CD Pipeline 관리, 모니터링, 버전 호환성',
    active: true,
  },
  {
    phase: 'Phase 2 - DevSecOps',
    period: 'v0.5 · 2026 Q3-Q4',
    description: 'SAST/DAST 보안 스캔, 자동화 테스트, CLI 도구, Multi Cloud 확장',
    active: false,
  },
  {
    phase: 'Phase 3 - InfraOps',
    period: 'v1.0 · 2027+',
    description: 'Kubernetes 클러스터 프로비저닝, IaC 통합, CNCF Sandbox 추진',
    active: false,
  },
]

const stages = [
  { name: 'Develop', icon: Code2, active: true },
  { name: 'Build', icon: Hammer, active: true },
  { name: 'Security', icon: ShieldCheck, active: false },
  { name: 'Test', icon: FlaskConical, active: false },
  { name: 'Deploy', icon: Rocket, active: true },
  { name: 'Monitoring', icon: ChartNoAxesColumn, active: true },
  { name: 'FinOps', icon: Coins, active: false },
]

const quickLinks = [
  { label: 'DevSecOps Stack Install', path: '/stack/templates', icon: Box, iconClassName: 'text-[#818cf8]' },
  { label: 'Stack Templates', path: '/stack/templates', icon: BookOpen, iconClassName: 'text-[#34d399]' },
  { label: 'CI/CD Templates', path: '/cicd/templates', icon: Code2, iconClassName: 'text-[#fbbf24]' },
  { label: 'CI/CD List', path: '/cicd/list', icon: ChartNoAxesColumn, iconClassName: 'text-[#60a5fa]' },
  { label: 'Monitoring Dashboard', path: '/observability/monitoring', icon: ChartNoAxesColumn, iconClassName: 'text-[#f87171]' },
  { label: 'Stack Version', path: '/stack/version', icon: ShieldCheck, iconClassName: 'text-[#c4b5fd]' },
]

export function HomePage() {
  const navigate = useNavigate()
  const { role } = useAuthStore()

  const getRoleLandingPath = (currentRole: Role): string => {
    if (currentRole === 'developer') {
      return '/cicd/templates'
    }

    return '/stack/templates'
  }

  return (
    <div>
      <div className="mb-8 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-home-hero-bg)] p-8 text-center">
        <div className="mx-auto mb-5 flex h-20 w-20 items-center justify-center rounded-[20px] bg-[linear-gradient(135deg,#ffd700,#f59e0b)] text-[#1a1d29] shadow-[0_8px_32px_rgba(255,215,0,0.3)]">
          <Box size={36} />
        </div>
        <h1 className="m-0 mb-2.5 text-4xl font-extrabold text-[var(--color-text-primary)]">Nullus Platform</h1>
        <p className="m-0 mb-2 text-base text-[var(--color-text-secondary)]">DevSecOps 인프라 자동화 플랫폼</p>
        <p className="mx-auto mb-8 max-w-[720px] text-sm leading-7 text-[var(--color-text-muted)]">
          검증된 CI/CD Golden Path 조합을 선택하고, 노코드 UI 기반으로 Kubernetes DevSecOps 파이프라인을 빠르게 구축합니다.
        </p>

        <div className="flex flex-wrap justify-center gap-3">
          <button
            type="button"
            onClick={() => navigate(getRoleLandingPath(role))}
            className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] px-6 py-3 text-sm font-bold text-[#1a1d29]"
          >
            <Rocket size={16} />
            Stack 시작하기
          </button>
          <button
            type="button"
            onClick={() => navigate('/cicd/templates')}
            className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.15)] px-6 py-3 text-sm font-semibold text-[#a5b4fc]"
          >
            <Code2 size={16} />
            CI/CD 파이프라인
          </button>
        </div>
      </div>

      <div className="mb-8">
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">핵심 기능</h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3.5">
          {features.map((feature) => {
            const Icon = feature.icon
            return (
              <div key={feature.title} className="rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[18px]">
                <div className={`mb-3 flex h-9 w-9 items-center justify-center rounded-[10px] ${feature.iconClassName}`}>
                  <Icon size={16} />
                </div>
                <div className="mb-1.5 text-sm font-bold text-[var(--color-text-primary)]">{feature.title}</div>
                <div className="text-xs leading-5 text-[var(--color-text-secondary)]">{feature.description}</div>
              </div>
            )
          })}
        </div>
      </div>

      <div className="mb-8">
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">개발 로드맵</h2>
        <div className="flex flex-wrap gap-3.5">
          {roadmap.map((item) => (
            <div
              key={item.phase}
              className={`min-w-[220px] flex-1 rounded-[12px] border p-[18px] ${item.active ? 'border-[rgba(255,215,0,0.35)] bg-[rgba(255,215,0,0.06)]' : 'border-[var(--color-border-default)] bg-[var(--color-surface-card)]'}`}
            >
              <div className={`mb-2 inline-flex rounded-[999px] px-2.5 py-1 text-[11px] font-bold ${item.active ? 'bg-[rgba(255,215,0,0.14)] text-[#ffd700]' : 'bg-[rgba(148,163,184,0.12)] text-[#94a3b8]'}`}>
                {item.period}
              </div>
              <div className={`mb-1.5 text-sm font-bold ${item.active ? 'text-[#ffd700]' : 'text-[#94a3b8]'}`}>{item.phase}</div>
              <div className="text-xs leading-6 text-[var(--color-text-secondary)]">{item.description}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="mb-8 flex flex-wrap gap-2 rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
        {stages.map((stage) => {
          const Icon = stage.icon
          return (
            <div
              key={stage.name}
              className={`inline-flex items-center gap-2 rounded-[10px] border px-3 py-2 text-xs font-semibold ${stage.active ? 'border-[rgba(99,102,241,0.35)] bg-[rgba(99,102,241,0.12)] text-[#a5b4fc]' : 'border-[var(--color-border-default)] bg-[rgba(148,163,184,0.08)] text-[var(--color-text-secondary)]'}`}
            >
              <Icon size={14} />
              {stage.name}
            </div>
          )
        })}
      </div>

      <div>
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">빠른 이동</h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2.5">
          {quickLinks.map((item) => {
            const Icon = item.icon
            return (
              <button
                key={item.label}
                type="button"
                onClick={() => navigate(item.path)}
                className="flex cursor-pointer items-center gap-2.5 rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3.5 py-3 text-left text-sm text-[var(--color-text-primary)] transition-colors hover:border-[#ffd70066]"
              >
                <Icon size={16} className={item.iconClassName} />
                <span>{item.label}</span>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
