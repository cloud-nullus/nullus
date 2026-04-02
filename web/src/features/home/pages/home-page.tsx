import { useNavigate } from 'react-router-dom'
import { useState } from 'react'
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
    title: 'Automated DevSecOps Stack Installation',
    description: 'Deploy GitLab, ArgoCD, and Prometheus stacks directly to Kubernetes from the UI.',
    icon: Box,
    iconClassName: 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]',
  },
  {
    title: 'Golden Path Templates',
    description: 'Provides validated combinations (GitHub + ArgoCD, GitLab All-in-One) as templates.',
    icon: BookOpen,
    iconClassName: 'bg-[rgba(16,185,129,0.15)] text-[#34d399]',
  },
  {
    title: 'CI/CD Pipeline Management',
    description: 'Create pipelines and manage deployment history with Web/API/Batch templates.',
    icon: Code2,
    iconClassName: 'bg-[rgba(245,158,11,0.15)] text-[#fbbf24]',
  },
  {
    title: 'Version Compatibility Assurance',
    description: 'Expose only validated tool version combinations to reduce unpredictable compatibility issues.',
    icon: ShieldCheck,
    iconClassName: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
  },
  {
    title: 'Unified Monitoring',
    description: 'Check cluster, pipeline, and application status from a single dashboard.',
    icon: ChartNoAxesColumn,
    iconClassName: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  },
  {
    title: 'Role-based Access Control',
    description: 'Control feature access by role: Admin, DevOps, and Developer.',
    icon: Users,
    iconClassName: 'bg-[rgba(139,92,246,0.15)] text-[#c4b5fd]',
  },
]

const roadmap = [
  {
    phase: 'Phase 1 - DevOps',
    period: 'v0.1 · 2026 Q2',
    description: 'Automated DevSecOps stack install, CI/CD pipeline management, monitoring, and version compatibility',
    active: true,
  },
  {
    phase: 'Phase 2 - DevSecOps',
    period: 'v0.5 · 2026 Q3-Q4',
    description: 'Nullus CLI, Security scanning, Automated Tests, Customization, Air-Gap (Offline Mode)',
    active: false,
  },
  {
    phase: 'Phase 3 - InfraOps',
    period: 'v1.0 · 2027+',
    description: 'Kubernetes Cluster Provisioning, IaC integration, FinOps',
    active: false,
  },
]

const stages = [
  { name: 'Cluster Provisioning', icon: Cog, active: false },
  { name: 'Develop', icon: Code2, active: true },
  { name: 'Build', icon: Hammer, active: true },
  { name: 'Security', icon: ShieldCheck, active: false },
  { name: 'Test', icon: FlaskConical, active: false },
  { name: 'Deploy', icon: Rocket, active: true },
  { name: 'Monitoring', icon: ChartNoAxesColumn, active: true },
  { name: 'FinOps', icon: Coins, active: false },
]

const ROADMAP_STAGE_ACTIVATIONS: Record<string, string[]> = {
  'Phase 1 - DevOps': ['Develop', 'Build', 'Deploy', 'Monitoring'],
  'Phase 2 - DevSecOps': ['Develop', 'Build', 'Security', 'Test', 'Deploy', 'Monitoring'],
  'Phase 3 - InfraOps': ['Cluster Provisioning', 'Develop', 'Build', 'Security', 'Test', 'Deploy', 'Monitoring', 'FinOps'],
}

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
  const [selectedRoadmapPhase, setSelectedRoadmapPhase] = useState(roadmap[0].phase)
  const isAdmin = role === 'admin'
  const isDevops = role === 'devops'
  const isDeveloper = role === 'developer'

  const getRoleLandingPath = (currentRole: Role): string => {
    if (currentRole === 'developer') {
      return '/cicd/templates'
    }

    return '/stack/templates'
  }

  const enabledButtonClassName =
    'inline-flex cursor-pointer items-center gap-2 rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] px-6 py-3 text-sm font-bold text-[#1a1d29]'
  const disabledButtonClassName =
    'inline-flex cursor-not-allowed items-center gap-2 rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(148,163,184,0.12)] px-6 py-3 text-sm font-semibold text-[var(--color-text-muted)] opacity-60'

  const canRegisterCluster = isAdmin
  const canStartStack = isAdmin || isDevops
  const canUseCicdPipeline = isAdmin || isDevops || isDeveloper

  return (
    <div>
      <div className="mb-8 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-home-hero-bg)] p-8 text-center">
        <div className="mx-auto mb-5 flex h-20 w-20 items-center justify-center rounded-[20px] bg-[linear-gradient(135deg,#ffd700,#f59e0b)] text-[#1a1d29] shadow-[0_8px_32px_rgba(255,215,0,0.3)]">
          <Box size={36} />
        </div>
        <h1 className="m-0 mb-2.5 text-4xl font-extrabold text-[var(--color-text-primary)]">Nullus Platform</h1>
        <p className="m-0 mb-2 text-base text-[var(--color-text-secondary)]">DevSecOps Infrastructure Automation Platform</p>
        <p className="mx-auto mb-8 max-w-[900px] text-sm leading-7 text-[var(--color-text-muted)]">
          Select validated CI/CD golden path combinations and quickly build Kubernetes DevSecOps pipelines with a no-code UI.
        </p>

        <div className="flex flex-wrap justify-center gap-3">
          <button
            type="button"
            disabled={!canRegisterCluster}
            onClick={() => navigate('/admin/clusters')}
            className={canRegisterCluster ? enabledButtonClassName : disabledButtonClassName}
          >
            <Cog size={16} />
            Register Cluster
          </button>
          <button
            type="button"
            disabled={!canStartStack}
            onClick={() => navigate(getRoleLandingPath(role))}
            className={canStartStack ? enabledButtonClassName : disabledButtonClassName}
          >
            <Rocket size={16} />
            Start Stack
          </button>
          <button
            type="button"
            disabled={!canUseCicdPipeline}
            onClick={() => navigate('/cicd/templates')}
            className={canUseCicdPipeline ? enabledButtonClassName : disabledButtonClassName}
          >
            <Code2 size={16} />
            CI/CD Pipeline
          </button>
        </div>
      </div>

      <div className="mb-8">
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">Core Features</h2>
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
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">Roadmap</h2>
        <div className="flex flex-wrap gap-3.5">
          {roadmap.map((item) => {
            const isSelected = selectedRoadmapPhase === item.phase
            return (
            <button
              key={item.phase}
              type="button"
              onClick={() => setSelectedRoadmapPhase(item.phase)}
              className={`min-w-[220px] flex-1 cursor-pointer rounded-[12px] border p-[18px] text-left transition-all duration-150 ${isSelected ? 'border-[rgba(255,215,0,0.55)] bg-[rgba(255,215,0,0.1)] shadow-[0_0_0_1px_rgba(255,215,0,0.35),0_8px_26px_rgba(255,215,0,0.2)]' : 'border-[var(--color-border-default)] bg-[var(--color-surface-card)] hover:border-[rgba(255,215,0,0.35)] hover:bg-[rgba(255,215,0,0.04)]'}`}
            >
              <div className={`mb-2 inline-flex rounded-[999px] px-2.5 py-1 text-[11px] font-bold ${isSelected ? 'bg-[rgba(255,215,0,0.14)] text-[#ffd700]' : 'bg-[rgba(148,163,184,0.12)] text-[#94a3b8]'}`}>
                {item.period}
              </div>
              <div className={`mb-1.5 text-sm font-bold ${isSelected ? 'text-[#ffd700]' : 'text-[#94a3b8]'}`}>{item.phase}</div>
              <div className="text-xs leading-6 text-[var(--color-text-secondary)]">{item.description}</div>
            </button>
          )})}
        </div>
      </div>

      <div className="mb-8 flex flex-wrap gap-2 rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
        {stages.map((stage) => {
          const Icon = stage.icon
          const activeStages = ROADMAP_STAGE_ACTIVATIONS[selectedRoadmapPhase] ?? []
          const isActiveForSelectedRoadmap = activeStages.includes(stage.name)
          return (
            <div
              key={stage.name}
              className={`inline-flex items-center gap-2 rounded-[10px] border px-3 py-2 text-xs font-semibold transition-all duration-150 ${isActiveForSelectedRoadmap ? 'border-[rgba(255,215,0,0.8)] bg-[linear-gradient(135deg,rgba(255,215,0,0.22),rgba(245,158,11,0.2))] text-[#ffe38a] shadow-[0_0_0_1px_rgba(255,215,0,0.4),0_0_18px_rgba(255,215,0,0.3)]' : 'border-[var(--color-border-default)] bg-[rgba(148,163,184,0.08)] text-[var(--color-text-secondary)] opacity-75'}`}
            >
              <Icon size={14} />
              {stage.name}
            </div>
          )
        })}
      </div>

      <div>
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">Quick Links</h2>
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
