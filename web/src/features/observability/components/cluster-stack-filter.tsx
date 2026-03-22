import { useMemo } from 'react'
import type { Cluster, Stack } from '../../../types'
import { NativeSelect } from '../../../components/ui/native-select'
import { cn } from '../../../lib/utils'
import { useStacks } from '../../stack/api/stack-api'
import { useClusters } from '../../admin/api/admin-api'

const statusDot = (status: string) => (
  <span
    className={cn(
      'inline-block h-2 w-2 shrink-0 rounded-full',
      status === 'running' || status === 'connected' || status === 'completed' || status === 'success'
        ? 'bg-emerald-400'
        : status === 'warning' || status === 'pending'
          ? 'bg-amber-400'
          : 'bg-red-400'
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

  const filteredStacks = useMemo(
    () => (selectedClusterId ? stacks.filter((stack) => stack.clusterId === selectedClusterId) : []),
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

interface ClusterStackFilterProps {
  selectedClusterId: string
  selectedStackId: string
  onClusterChange: (clusterId: string) => void
  onStackChange: (stackId: string) => void
  onClear: () => void
  clusters: Cluster[]
  filteredStacks: Stack[]
  selectedCluster?: Cluster
  selectedStack?: Stack
  className?: string
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
}: ClusterStackFilterProps) => {
  const hasContext = selectedClusterId !== '' || selectedStackId !== ''

  return (
    <div className={cn('mb-5 flex flex-wrap items-end gap-4 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4', className)}>
      <div className="flex items-end gap-3">
        <NativeSelect
          label="Cluster"
          value={selectedClusterId}
          onChange={(event) => onClusterChange(event.target.value)}
          className="min-w-[200px]"
        >
          <option value="">— Select Cluster —</option>
          {clusters.map((cluster) => (
            <option key={cluster.id} value={cluster.id}>{cluster.name}</option>
          ))}
        </NativeSelect>
        {selectedCluster && (
          <div className="mb-[9px] flex items-center gap-1.5 text-xs">
            {statusDot(selectedCluster.status)}
            <span className="capitalize text-[var(--color-text-secondary)]">{selectedCluster.status}</span>
          </div>
        )}
      </div>

      <div className="flex items-end gap-3">
        <NativeSelect
          label="Stack"
          value={selectedStackId}
          onChange={(event) => onStackChange(event.target.value)}
          className="min-w-[200px]"
          disabled={!selectedClusterId}
        >
          <option value="">{selectedClusterId ? '— Select Stack —' : '— Select Cluster First —'}</option>
          {filteredStacks.map((stack) => (
            <option key={stack.id} value={stack.id}>{stack.name}</option>
          ))}
        </NativeSelect>
        {selectedStack && (
          <div className="mb-[9px] flex items-center gap-1.5 text-xs">
            {statusDot(selectedStack.status)}
            <span className="capitalize text-[var(--color-text-secondary)]">{selectedStack.status}</span>
          </div>
        )}
      </div>

      {hasContext && (
        <button
          type="button"
          onClick={onClear}
          className="mb-[9px] text-xs text-[var(--color-text-secondary)] hover:text-red-400"
        >
          Clear
        </button>
      )}
    </div>
  )
}
