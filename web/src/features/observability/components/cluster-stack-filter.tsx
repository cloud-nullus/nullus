import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { Cluster, Stack } from '../../../types'
import { Select } from '../../../components/ui/select'
import { cn } from '../../../lib/utils'
import { useStacks } from '../../stack/api/stack-api'
import { useScopedClusters as useClusters } from '../../admin/api/admin-api'

const formatStatusLabel = (status: string) => status.replace(/_/g, ' ')

const statusDot = (status: string) => (
  <span
    className={cn(
      'inline-block h-2 w-2 shrink-0 rounded-full',
      status === 'running' || status === 'connected' || status === 'completed' || status === 'success'
        ? 'bg-[var(--color-success)]'
        : status === 'warning' || status === 'pending'
          ? 'bg-[var(--color-warning)]'
          : 'bg-[var(--color-error)]'
    )}
  />
)

export interface ClusterStackFilterState {
  clusters: Cluster[]
  stacks: Stack[]
  filteredStacks: Stack[]
  selectedCluster: Cluster | undefined
  selectedStack: Stack | undefined
  hasContext: boolean
}

export const useClusterStackFilterState = (selectedClusterId: string, selectedStackId: string): ClusterStackFilterState => {
  const { data: clustersData } = useClusters()
  const { data: stacksData } = useStacks()

  const clusters = clustersData?.items ?? []
  const stacks = stacksData?.items ?? []

  // When a cluster is selected show only its stacks; otherwise show all stacks
  // so users can pick a stack independently without selecting a cluster first.
  const filteredStacks = useMemo(
    () => (selectedClusterId ? stacks.filter((stack) => stack.clusterId === selectedClusterId) : stacks),
    [selectedClusterId, stacks]
  )

  const selectedCluster = clusters.find((cluster) => cluster.id === selectedClusterId)
  const selectedStack = filteredStacks.find((stack) => stack.id === selectedStackId)

  return {
    clusters,
    stacks,
    filteredStacks,
    selectedCluster,
    selectedStack,
    hasContext: selectedClusterId !== '' || selectedStackId !== '',
  }
}

/**
 * 선택지에 필요한 만큼만 받는다. 기존 호출부는 Cluster · Stack 을 그대로 넘기고, 보안
 * 대시보드는 자기 응답(스택 묶음)에서 만든 가벼운 선택지를 넘긴다 — 상태가 없으면 점을 안 그린다.
 */
export type FilterCluster = Pick<Cluster, 'id' | 'name'> & { status?: string }
export type FilterStack = Pick<Stack, 'id' | 'name'> & { clusterId?: string; status?: string }

interface ClusterStackFilterProps {
  selectedClusterId: string
  selectedStackId: string
  onClusterChange: (clusterId: string) => void
  onStackChange: (stackId: string) => void
  onClear: () => void
  clusters: FilterCluster[]
  filteredStacks: FilterStack[]
  selectedCluster?: FilterCluster
  selectedStack?: FilterStack
  className?: string
  /**
   * 빈 값을 "선택 안 함" 이 아니라 "전체" 로 읽는 화면(보안 대시보드 — 기본 범위가 조직
   * 전체다). 빈 칸의 문구만 바뀐다. 기존 화면은 그대로 "— 선택 —" 이고, 첫 클러스터를
   * 골라 여는 모니터링 화면의 동작도 그 화면의 것이라 바뀌지 않는다.
   */
  allOption?: boolean
  /** 스택 목록 뒤에 붙는 특수 선택지(예: 스택 없는 앱). */
  extraStackOptions?: { value: string; label: string }[]
  /** 같은 줄 오른쪽에 붙는 화면별 조작(검색 · 토글). */
  children?: ReactNode
}

export const ClusterStackFilter = ({
  selectedClusterId,
  selectedStackId,
  onClusterChange,
  onStackChange,
  onClear,
  clusters,
  filteredStacks,
  selectedCluster,
  selectedStack,
  className,
  allOption = false,
  extraStackOptions = [],
  children,
}: ClusterStackFilterProps) => {
  const { t } = useTranslation()
  const hasContext = selectedClusterId !== '' || selectedStackId !== ''

  return (
    <div className={cn('mb-5 flex flex-wrap items-end gap-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4', className)}>
      <div className="flex items-end gap-3">
        <Select
          label={t('clusterStackFilter.clusterLabel', 'Cluster')}
          value={selectedClusterId}
          onChange={(event) => onClusterChange(event.target.value)}
          className="min-w-[200px]"
        >
          <option value="">
            {allOption
              ? t('clusterStackFilter.allClusters', 'All clusters')
              : t('clusterStackFilter.selectCluster', '— Select Cluster —')}
          </option>
          {clusters.map((cluster) => (
            <option key={cluster.id} value={cluster.id}>{cluster.name}</option>
          ))}
        </Select>
        {selectedCluster?.status && (
          <div className="mb-[9px] flex items-center gap-1.5 text-xs">
            {statusDot(selectedCluster.status)}
            <span className="capitalize text-[var(--color-text-secondary)]">{selectedCluster.status}</span>
          </div>
        )}
      </div>

      <div className="flex items-end gap-3">
        <Select
          label={t('clusterStackFilter.stackLabel', 'Stack')}
          value={selectedStackId}
          onChange={(event) => onStackChange(event.target.value)}
          className="min-w-[200px]"
        >
          <option value="">
            {allOption
              ? t('clusterStackFilter.allStacks', 'All stacks ({{count}})', { count: filteredStacks.length })
              : t('clusterStackFilter.selectStack', '— Select Stack —')}
          </option>
          {filteredStacks.map((stack) => (
            <option key={stack.id} value={stack.id}>
              {stack.status ? `${stack.name} (${formatStatusLabel(stack.status)})` : stack.name}
            </option>
          ))}
          {extraStackOptions.map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </Select>
        {selectedStack?.status && (
          <div className="mb-[9px] flex items-center gap-1.5 text-xs">
            {statusDot(selectedStack.status)}
            <span className="capitalize text-[var(--color-text-secondary)]">{formatStatusLabel(selectedStack.status)}</span>
          </div>
        )}
      </div>

      {hasContext && (
        <button
          type="button"
          onClick={onClear}
          className="mb-[9px] text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-error)]"
        >
          {t('clusterStackFilter.clear', 'Clear')}
        </button>
      )}

      {children && <div className="ml-auto flex flex-wrap items-end gap-3">{children}</div>}
    </div>
  )
}
