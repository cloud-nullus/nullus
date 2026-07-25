import type { Cluster } from '../../../types'

export function findPlatformCluster(clusters: Cluster[]): Cluster | undefined {
  return clusters.find((cluster) => {
    const typeList = Array.isArray(cluster.types) ? cluster.types : []
    return typeList.includes('pipeline') || cluster.type === 'pipeline'
  })
}
