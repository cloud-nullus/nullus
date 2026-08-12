import { useEffect, useMemo, useRef, useState } from 'react'
import type React from 'react'
import { ChartColumn, GitBranch, LayoutDashboard, Server } from 'lucide-react'
import { iconProps } from '../../../components/ui/icon'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../../../stores/auth-store'
import { Tabs } from '../../../components/ui/tabs'
import { ClusterStackFilter, useClusterStackFilterState } from '../components/cluster-stack-filter'
import { usePipelines } from '../../cicd/api/cicd-api'
import { StackMonitoringOverview } from '../components/stack-monitoring-overview'
import { DashboardTabLayout } from "../components/monitoring-tab-layout"
import type { ViewType } from "../components/monitoring-tab-layout"
import { ClusterDefault } from "../components/monitoring-cluster-view"
import { CicdDefault, CICD_DEFAULT_TABS } from "../components/monitoring-cicd-view"
import { StackConnectPanel } from "../components/monitoring-connect-panel"
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
  const { data: pipelinesData } = usePipelines()

  const didAutoSelectRef = useRef(false)

  useEffect(() => {
    if (didAutoSelectRef.current) return
    if (clusters.length === 0) return

    // 데이터가 있는 클러스터에서 시작한다. clusters[0] 을 그냥 골랐더니 스택도
    // 파이프라인도 없는 클러스터가 잡혀 모든 지표가 0 으로 떴다 — 필터는 제대로
    // 동작했지만 화면은 고장난 것처럼 보였다.
    const firstCluster = clusters.find((cluster) => stacks.some((stack) => stack.clusterId === cluster.id))
      ?? clusters[0]
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

  // CI/CD 탭은 클러스터가 선언한 타입이 아니라 그 클러스터에 파이프라인이 실제로
  // 있는지로 연다.
  //
  // 타입(types 에 'target' 포함)으로 걸었더니 정상 구성에서 지표가 어디서도 안 보였다.
  // 파이프라인 2개가 모두 사는 kind-nullus-platform 은 types=['pipeline'] 이라 탭이
  // 잠겼고, 탭이 열리는 kind-nullus-develop(types=['target'])에는 파이프라인이
  // 하나도 없었다. 등록 타입은 운영자가 손으로 적는 값이라 현실과 어긋날 수 있지만,
  // 파이프라인의 cluster_id 는 이 화면이 그리려는 그 데이터 자체다.
  const supportsCicd = useMemo(() => {
    if (!selectedClusterId) return false
    return (pipelinesData?.items ?? []).some((pipeline) => pipeline.clusterId === selectedClusterId)
  }, [pipelinesData?.items, selectedClusterId])

  useEffect(() => {
    if (activeView === 'cicd' && !supportsCicd) {
      setActiveView(selectedClusterId ? 'cluster' : null)
    }
  }, [activeView, supportsCicd, selectedClusterId])

  const views: { id: ViewType; label: string; icon: React.ReactNode; disabled?: boolean }[] = [
    { id: 'cluster', label: 'Cluster', icon: <Server {...iconProps('sm')} />, disabled: !selectedClusterId },
    { id: 'stack', label: 'Stack', icon: <ChartColumn {...iconProps('sm')} />, disabled: !selectedStackId },
    { id: 'cicd', label: 'CI/CD', icon: <GitBranch {...iconProps('sm')} />, disabled: !selectedClusterId || !supportsCicd },
  ]

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t('observability.monitoring', 'Monitoring Dashboard') }]}
        icon={<LayoutDashboard {...iconProps('sm')} />}
        tone="info"
        title={t('observability.monitoring', 'Monitoring Dashboard')}
        subtitle={t('observability.monitoringDesc', 'Select a Cluster or Stack to start monitoring')}
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
            <ChartColumn {...iconProps('lg')} className="opacity-20" />
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
                  defaultContent={<CicdDefault selectedClusterId={selectedClusterId} selectedStackId={selectedStackId} />}
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
