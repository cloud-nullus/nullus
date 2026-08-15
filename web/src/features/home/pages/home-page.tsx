import { useNavigate } from 'react-router-dom'
import { IconTile } from '../../../components/ui/icon-tile'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { BookOpen, Box, ChartColumn, Code2, Coins, FlaskConical, Hammer, Rocket, Settings, ShieldCheck, Users, Zap } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { useClusters } from '../../admin/api/admin-api'
import { useAuthStore } from '../../../stores/auth-store'
import { roleLandingPath } from '../../auth/role-landing'
import { NullusMark } from '../../../components/brand/nullus-mark'
import { SupportToolsMarquee } from '../components/support-tools-marquee'

// 화면에 나오는 글자는 전부 i18n 키로 받는다. 로드맵·단계·기능은 id 로 식별하고
// 이름은 번역에서 온다 — 예전에는 영어 표시 문자열이 곧 식별자여서, 번역을
// 붙이는 순간 로드맵 선택과 단계 강조가 서로를 못 찾게 된다.
const features = [
  { id: 'stackInstall', icon: Box, token: '--color-primary' },
  { id: 'goldenPath', icon: BookOpen, token: '--color-success' },
  { id: 'cicd', icon: Code2, token: '--color-warning' },
  { id: 'compatibility', icon: ShieldCheck, token: '--color-error' },
  { id: 'monitoring', icon: ChartColumn, token: '--color-info' },
  { id: 'rbac', icon: Users, token: '--color-accent-alt' },
]

/**
 * 퀵스타트가 여는 템플릿.
 *
 * 8Gi 노드 하나에 들어가는 최소 구성이라 "일단 돌려 보는" 첫 설치에 맞는다.
 * 템플릿이 규모 프로파일(local)까지 들고 있으므로 마법사가 자원 계획도 그
 * 크기에서 시작한다 — 여기서 크기를 따로 넘길 필요가 없다.
 */
const QUICK_START_TEMPLATE_ID = 'gitea-jenkins-argocd-lite-v1'

const roadmap = [{ id: 'phase1' }, { id: 'phase2' }, { id: 'phase3' }]

const stages = [
  { id: 'clusterProvisioning', icon: Settings },
  { id: 'develop', icon: Code2 },
  { id: 'build', icon: Hammer },
  { id: 'security', icon: ShieldCheck },
  { id: 'test', icon: FlaskConical },
  { id: 'deploy', icon: Rocket },
  { id: 'monitoring', icon: ChartColumn },
  { id: 'finops', icon: Coins },
]

const ROADMAP_STAGE_ACTIVATIONS: Record<string, string[]> = {
  phase1: ['develop', 'build', 'deploy', 'monitoring'],
  phase2: ['develop', 'build', 'security', 'test', 'deploy', 'monitoring'],
  phase3: ['clusterProvisioning', 'develop', 'build', 'security', 'test', 'deploy', 'monitoring', 'finops'],
}

const quickLinks = [
  { id: 'stackInstall', path: '/stack/templates', icon: Box, iconClassName: 'text-[var(--color-primary)]' },
  { id: 'stackTemplates', path: '/stack/templates', icon: BookOpen, iconClassName: 'text-[var(--color-success)]' },
  { id: 'cicdTemplates', path: '/cicd/templates', icon: Code2, iconClassName: 'text-[var(--color-warning)]' },
  { id: 'cicdList', path: '/cicd/list', icon: ChartColumn, iconClassName: 'text-[var(--color-info)]' },
  { id: 'monitoring', path: '/observability/monitoring', icon: ChartColumn, iconClassName: 'text-[var(--color-error)]' },
  { id: 'stackVersion', path: '/stack/version', icon: ShieldCheck, iconClassName: 'text-[var(--color-accent-alt)]' },
]

export function HomePage() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { role } = useAuthStore()
  const { data: clusters } = useClusters()
  const [selectedRoadmapPhase, setSelectedRoadmapPhase] = useState(roadmap[0].id)
  const isAdmin = role === 'admin'
  const isDevops = role === 'devops'
  const isDeveloper = role === 'developer'

  const enabledButtonClassName =
    'inline-flex cursor-pointer items-center gap-2 rounded-[10px] border-none bg-[var(--color-primary)] px-6 py-3 text-sm font-bold text-[var(--color-surface-base)]'
  const disabledButtonClassName =
    'inline-flex cursor-not-allowed items-center gap-2 rounded-[10px] border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-secondary)_12%,_transparent)] px-6 py-3 text-sm font-semibold text-[var(--color-text-muted)] opacity-60'

  const canRegisterCluster = isAdmin
  const canStartStack = isAdmin || isDevops
  const canUseCicdPipeline = isAdmin || isDevops || isDeveloper

  // 클러스터가 하나도 없으면 마법사는 설치할 곳을 찾지 못한다. 누를 수 있게
  // 두면 사용자는 그 사실을 마법사 마지막 단계에 가서야 알게 된다.
  const hasRegisteredCluster = (clusters?.items ?? []).length > 0
  const canQuickStart = canStartStack && hasRegisteredCluster

  return (
    <div>
      <div className="mb-8 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-home-hero-bg)] p-8 text-center">
        {/* 마크가 스스로 3색을 갖고 있어 금색 타일 위에 얹으면 색이 싸운다.
            바탕 없이 그대로 둔다 — 바로 아래 제목이 이름을 말해 준다. */}
        <NullusMark size={80} decorative className="mx-auto mb-5 block" />
        {/* 제품명은 번역하지 않는다. */}
        <h1 className="m-0 mb-2.5 text-4xl font-extrabold text-[var(--color-text-primary)]">Nullus Platform</h1>
        <p className="m-0 mb-2 text-base text-[var(--color-text-secondary)]">
          {t('home.tagline', 'DevSecOps Infrastructure Automation Platform')}
        </p>
        <p className="mx-auto mb-8 max-w-[900px] text-sm leading-7 text-[var(--color-text-muted)]">
          {t(
            'home.description',
            'Select validated CI/CD golden path combinations and quickly build Kubernetes DevSecOps pipelines with a no-code UI.',
          )}
        </p>

        <div className="flex flex-wrap justify-center gap-3" data-tour="hero-cta">
          <button
            type="button"
            disabled={!canRegisterCluster}
            onClick={() => navigate('/admin/clusters')}
            className={canRegisterCluster ? enabledButtonClassName : disabledButtonClassName}
          >
            <Settings {...iconProps('sm')} />
            {t('home.cta.registerCluster', 'Register Cluster')}
          </button>
          <button
            type="button"
            disabled={!canStartStack}
            onClick={() => navigate(roleLandingPath(role))}
            className={canStartStack ? enabledButtonClassName : disabledButtonClassName}
          >
            <Rocket {...iconProps('sm')} />
            {t('home.cta.startStack', 'Start Stack')}
          </button>
          <button
            type="button"
            disabled={!canUseCicdPipeline}
            onClick={() => navigate('/cicd/templates')}
            className={canUseCicdPipeline ? enabledButtonClassName : disabledButtonClassName}
          >
            <Code2 {...iconProps('sm')} />
            {t('home.cta.cicdPipeline', 'CI/CD Pipeline')}
          </button>
          <button
            type="button"
            data-tour="quick-start"
            disabled={!canQuickStart}
            onClick={() => navigate(`/stack/install?template=${QUICK_START_TEMPLATE_ID}`)}
            className={canQuickStart ? enabledButtonClassName : disabledButtonClassName}
          >
            <Zap {...iconProps('sm')} />
            {t('home.cta.quickStart', 'Quick Start (8Gi)')}
          </button>
        </div>

        {canStartStack && !hasRegisteredCluster && (
          // 비활성 버튼만 두면 왜 안 눌리는지 알 수 없다. 막힌 이유와 다음 걸음을
          // 같은 자리에서 말한다.
          <p className="m-0 mt-3 text-xs text-[var(--color-text-muted)]">
            {t('home.cta.quickStartNeedsCluster', 'Register a cluster first to use Quick Start.')}
          </p>
        )}
      </div>

      <SupportToolsMarquee />

      <div className="mb-8">
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">
          {t('home.coreFeatures.title', 'Core Features')}
        </h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3.5">
          {features.map((feature) => {
            const Icon = feature.icon
            return (
              <div key={feature.id} className="rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[18px]">
                <IconTile token={feature.token} className="mb-3">
                  <Icon {...iconProps('md')} />
                </IconTile>
                <div className="mb-1.5 text-sm font-bold text-[var(--color-text-primary)]">
                  {t(`home.coreFeatures.${feature.id}.title`)}
                </div>
                <div className="text-xs leading-5 text-[var(--color-text-secondary)]">
                  {t(`home.coreFeatures.${feature.id}.description`)}
                </div>
              </div>
            )
          })}
        </div>
      </div>

      <div className="mb-8">
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">{t('home.roadmap.title', 'Roadmap')}</h2>
        <div className="flex flex-wrap gap-3.5">
          {roadmap.map((item) => {
            const isSelected = selectedRoadmapPhase === item.id
            return (
            <button
              key={item.id}
              type="button"
              onClick={() => setSelectedRoadmapPhase(item.id)}
              className={`min-w-[220px] flex-1 cursor-pointer rounded-[12px] border p-[18px] text-left transition-all duration-150 ${isSelected ? 'border-[color-mix(in_srgb,_var(--color-brand-gold)_55%,_transparent)] bg-[color-mix(in_srgb,_var(--color-brand-gold)_10%,_transparent)] shadow-[0_0_0_1px_color-mix(in_srgb,_var(--color-brand-gold)_35%,_transparent),0_8px_26px_color-mix(in_srgb,_var(--color-brand-gold)_20%,_transparent)]' : 'border-[var(--color-border-default)] bg-[var(--color-surface-card)] hover:border-[color-mix(in_srgb,_var(--color-brand-gold)_35%,_transparent)] hover:bg-[color-mix(in_srgb,_var(--color-brand-gold)_4%,_transparent)]'}`}
            >
              <div className={`mb-2 inline-flex rounded-[999px] px-2.5 py-1 text-[11px] font-bold ${isSelected ? 'bg-[color-mix(in_srgb,_var(--color-brand-gold)_14%,_transparent)] text-[var(--color-brand-gold)]' : 'bg-[color-mix(in_srgb,_var(--color-text-secondary)_12%,_transparent)] text-[var(--color-text-secondary)]'}`}>
                {t(`home.roadmap.${item.id}.period`)}
              </div>
              <div className={`mb-1.5 text-sm font-bold ${isSelected ? 'text-[var(--color-brand-gold)]' : 'text-[var(--color-text-secondary)]'}`}>
                {t(`home.roadmap.${item.id}.phase`)}
              </div>
              <div className="text-xs leading-6 text-[var(--color-text-secondary)]">
                {t(`home.roadmap.${item.id}.description`)}
              </div>
            </button>
          )})}
        </div>
      </div>

      <div className="mb-8 flex flex-wrap gap-2 rounded-[12px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
        {stages.map((stage) => {
          const Icon = stage.icon
          const activeStages = ROADMAP_STAGE_ACTIVATIONS[selectedRoadmapPhase] ?? []
          const isActiveForSelectedRoadmap = activeStages.includes(stage.id)
          return (
            <div
              key={stage.id}
              className={`inline-flex items-center gap-2 rounded-[10px] border px-3 py-2 text-xs font-semibold transition-all duration-150 ${isActiveForSelectedRoadmap ? 'border-[color-mix(in_srgb,_var(--color-brand-gold)_80%,_transparent)] bg-[linear-gradient(135deg,color-mix(in_srgb,_var(--color-brand-gold)_22%,_transparent),color-mix(in_srgb,_var(--color-warning)_20%,_transparent))] text-[color-mix(in_srgb,_var(--color-brand-gold)_70%,_white)] shadow-[0_0_0_1px_color-mix(in_srgb,_var(--color-brand-gold)_40%,_transparent),0_0_18px_color-mix(in_srgb,_var(--color-brand-gold)_30%,_transparent)]' : 'border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-secondary)_8%,_transparent)] text-[var(--color-text-secondary)] opacity-75'}`}
            >
              <Icon {...iconProps('sm')} />
              {t(`home.stages.${stage.id}`)}
            </div>
          )
        })}
      </div>

      <div>
        <h2 className="mb-4 mt-0 text-lg font-bold text-[var(--color-text-primary)]">
          {t('home.quickLinks.title', 'Quick Links')}
        </h2>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2.5">
          {quickLinks.map((item) => {
            const Icon = item.icon
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => navigate(item.path)}
                className="flex cursor-pointer items-center gap-2.5 rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-3.5 py-3 text-left text-sm text-[var(--color-text-primary)] transition-colors hover:border-[color-mix(in_srgb,_var(--color-brand-gold)_40%,_transparent)]"
              >
                <Icon {...iconProps('sm')} className={item.iconClassName} />
                <span>{t(`home.quickLinks.${item.id}`)}</span>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
