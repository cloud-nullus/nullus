import { useEffect, useMemo, useRef, useState } from 'react'
import type React from 'react'
import { Server, GitBranch, BarChart3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../../../stores/auth-store'
import { Tabs } from '../../../components/ui/tabs'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { StackMonitoringOverview } from '../components/stack-monitoring-overview'
import { DashboardTabLayout } from "../components/monitoring-tab-layout"
import type { ViewType } from "../components/monitoring-tab-layout"
import { ClusterDefault } from "../components/monitoring-cluster-view"
import { CicdDefault, CICD_DEFAULT_TABS } from "../components/monitoring-cicd-view"
import { StackConnectPanel } from "../components/monitoring-connect-panel"
import { PlatformToolHealth } from '../components/platform-tool-health'
import { useDashboard } from '../api/observability-api'
import { PageHeader } from '../../../components/layout/page-header'

function StackDefault({ stackId }: { stackId: string }) {
  return <StackMonitoringOverview stackId={stackId} />
}

export function MonitoringPage() {
  const { t } = useTranslation()
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === 'admin'

  const [selectedClusterId, setSelectedClusterId] = useState('')
  const [selectedStackId, setSelectedStackId] = useState('')
  const [activeView, setActiveView] = useState<ViewType | null>(null)

  const { clusters, stacks, filteredStacks, selectedCluster, selectedStack, hasContext } =
    useClusterStackFilterState(selectedClusterId, selectedStackId)

  // 플랫폼 전체 도구 상태는 클러스터/스택 선택과 무관하게 항상 보여준다.
  const { data: platformDashboard, isLoading: platformLoading } = useDashboard(30_000)

  const didAutoSelectRef = useRef(false)

  useEffect(() => {
    if (didAutoSelectRef.current) return
    if (clusters.length === 0) return

    const firstCluster = clusters[0]
    if (!firstCluster) return

    setSelectedClusterId(firstCluster.id)

    const firstStackForCluster = stacks.find((stack) => stack.clusterId === firstCluster.id)
    if (firstStackForCluster) {
      setSelectedStackId(firstStackForCluster.id)
      setActiveView('stack')
    } else {
      setActiveView('cluster')
    }

    didAutoSelectRef.current = true
  }, [clusters, stacks])

  // Auto-select initial view
  function handleClusterChange(id: string) {
    const clusterChanged = id !== selectedClusterId
    setSelectedClusterId(id)
    if (clusterChanged) {
      setSelectedStackId('')
      if (activeView === 'stack') setActiveView('cluster')
    }
    if (id && !activeView) setActiveView('cluster')
  }
  function handleStackChange(id: string) {
    setSelectedStackId(id)
    if (id && !activeView) setActiveView('stack')
  }

  const supportsCicd = useMemo(() => {
    if (!selectedCluster) return false
    const types = Array.isArray(selectedCluster.types) && selectedCluster.types.length > 0
      ? selectedCluster.types
      : (selectedCluster.type ? [selectedCluster.type] : [])
    const normalizedTypes = Array.from(new Set(types))
    return normalizedTypes.includes('target')
  }, [selectedCluster])

  useEffect(() => {
    if (activeView === 'cicd' && !supportsCicd) {
      setActiveView(selectedClusterId ? 'cluster' : null)
    }
  }, [activeView, supportsCicd, selectedClusterId])

  const views: { id: ViewType; label: string; icon: React.ReactNode; disabled?: boolean }[] = [
    { id: 'cluster', label: 'Cluster', icon: <Server size={15} />, disabled: !selectedClusterId },
    { id: 'stack', label: 'Stack', icon: <BarChart3 size={15} />, disabled: !selectedStackId },
    { id: 'cicd', label: 'CI/CD', icon: <GitBranch size={15} />, disabled: !selectedClusterId || !supportsCicd },
  ]

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('observability.monitoring', 'Monitoring Dashboard') }]}
        icon={<BarChart3 size={16} />}
        tone="info"
        title={t('observability.monitoring', 'Monitoring Dashboard')}
        subtitle={t('observability.monitoringDesc', 'Select a Cluster or Stack to start monitoring')}
      />

      <PlatformToolHealth
        tools={platformDashboard?.tools}
        isLoading={platformLoading}
      />

      <ClusterStackFilter
        selectedClusterId={selectedClusterId}
        selectedStackId={selectedStackId}
        onClusterChange={handleClusterChange}
        onStackChange={handleStackChange}
        onClear={() => { setSelectedClusterId(''); setSelectedStackId(''); setActiveView(null) }}
        clusters={clusters}
        filteredStacks={filteredStacks}
        selectedCluster={selectedCluster}
        selectedStack={selectedStack}
      />

      {/* ── Empty state ── */}
      {
        !hasContext && (
          <div className="flex h-44 flex-col items-center justify-center gap-2 rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] text-[var(--color-text-secondary)]">
            <BarChart3 size={28} className="opacity-20" />
            <p className="text-sm font-medium text-[var(--color-text-primary)]">Select a Cluster or Stack above to begin</p>
            <p className="text-xs">You can select either one or both.</p>
          </div>
        )
      }

      {/* ── View switcher + content ── */}
      {
        hasContext && (
          <>
            {/* View tabs */}
            <Tabs
              value={activeView ?? ''}
              onChange={(id) => setActiveView(id as ViewType)}
              items={views.map((v) => ({
                id: v.id as string,
                icon: v.icon,
                label: v.label,
                disabled: v.disabled,
              }))}
            />

            {/* View content */}
            <div className="pt-5">
              {activeView === 'cluster' && (
                <DashboardTabLayout
                  viewId="cluster"
                  isAdmin={isAdmin}
                  defaultContent={
                    <ClusterDefault
                      clusterId={selectedClusterId}
                      clusterName={selectedCluster?.name ?? ''}
                      stackIds={stacks.filter((stack) => stack.clusterId === selectedClusterId).map((stack) => stack.id)}
                    />
                  }
                />
              )}
              {activeView === 'stack' && (
                <DashboardTabLayout
                  viewId="stack"
                  isAdmin={isAdmin}
                  defaultContent={<StackDefault stackId={selectedStack?.id ?? selectedStackId} />}
                  firstTimePanel={(onConnect, onSkip) => (
                    <StackConnectPanel
                      stackName={selectedStack?.name ?? selectedStackId}
                      onConnect={onConnect}
                      onSkip={onSkip}
                    />
                  )}
                />
              )}
              {activeView === 'cicd' && (
                <DashboardTabLayout
                  viewId="cicd"
                  isAdmin={isAdmin}
                  defaultContent={<CicdDefault selectedClusterId={selectedClusterId} />}
                  seedTabs={CICD_DEFAULT_TABS}
                />
              )}
            </div>
          </>
        )
      }
    </div >
  )
}
