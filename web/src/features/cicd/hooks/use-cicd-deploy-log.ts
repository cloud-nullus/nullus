import { useState, useEffect, useRef } from 'react'
import { connect } from '../../../lib/websocket'

export type CicdLogLevel = 'info' | 'success' | 'error'
export type CicdDeployStatus = 'connecting' | 'running' | 'success' | 'failed'

export interface CicdLogEntry {
  id: string
  timestamp: string
  level: CicdLogLevel
  message: string
}

interface CicdDeployLogPayload {
  type: 'log' | 'status'
  level?: CicdLogLevel
  message?: string
  timestamp?: string
  status?: CicdDeployStatus
  progress?: number
}

export function useCicdDeployLog(deploymentId: string | null): {
  logs: CicdLogEntry[]
  status: CicdDeployStatus
  progress: number
  isConnected: boolean
} {
  const [logs, setLogs] = useState<CicdLogEntry[]>([])
  const [status, setStatus] = useState<CicdDeployStatus>('connecting')
  const [progress, setProgress] = useState(0)
  const [isConnected, setIsConnected] = useState(false)
  const counterRef = useRef(0)

  useEffect(() => {
    if (!deploymentId) return

    setLogs([])
    setStatus('connecting')
    setProgress(0)
    setIsConnected(false)
    counterRef.current = 0

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const wsUrl = `${protocol}://${window.location.host}/ws/cicd/deployments/${deploymentId}/logs`

    const client = connect(wsUrl, {
      onMessage: (data) => {
        const payload = data as CicdDeployLogPayload
        if (payload.progress !== undefined) setProgress(payload.progress)

        if (payload.type === 'log' && payload.message) {
          const message = payload.message
          setLogs((prev) => [
            ...prev,
            {
              id: String(++counterRef.current),
              timestamp: payload.timestamp ?? new Date().toISOString(),
              level: payload.level ?? 'info',
              message,
            },
          ])
        } else if (payload.type === 'status' && payload.status) {
          setStatus(payload.status)
        }
      },
      onStatusChange: (connected) => {
        setIsConnected(connected)
        if (connected) setStatus('running')
      },
    })

    return () => client.close()
  }, [deploymentId])

  return { logs, status, progress, isConnected }
}
