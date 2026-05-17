import { useEffect, useState } from 'react'
import { connect } from '../../../lib/websocket'

export interface PodWatchRow {
  name: string
  ready: string
  status: string
  restarts: string
  age: string
  namespace?: string
  updatedAt: string
}

interface PodWatchPayload {
  type: 'pod' | 'error'
  timestamp?: string
  namespace?: string
  name?: string
  ready?: string
  status?: string
  restarts?: string
  age?: string
  message?: string
}

interface UsePodWatchResult {
  pods: PodWatchRow[]
  error: string | null
  isConnected: boolean
}

export function usePodWatch(deploymentId: string): UsePodWatchResult {
  const [pods, setPods] = useState<PodWatchRow[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isConnected, setIsConnected] = useState(false)

  useEffect(() => {
    if (!deploymentId) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws/deployments/${deploymentId}/pods`
    const client = connect(wsUrl, {
      onMessage: (data) => {
        const payload = data as PodWatchPayload
        if (payload.type === 'error') {
          setError(payload.message ?? 'pod watch failed')
          return
        }
        if (payload.type !== 'pod' || !payload.name) return
        const podName = payload.name
        setError(null)
        setPods((prev) => {
          const next = prev.filter((pod) => pod.name !== podName)
          next.push({
            name: podName,
            ready: payload.ready ?? '-',
            status: payload.status ?? 'Unknown',
            restarts: payload.restarts ?? '0',
            age: payload.age ?? '-',
            namespace: payload.namespace,
            updatedAt: payload.timestamp ?? new Date().toISOString(),
          })
          return next.slice(-80)
        })
      },
      onStatusChange: setIsConnected,
    })

    return () => client.close()
  }, [deploymentId])

  return { pods, error, isConnected }
}
