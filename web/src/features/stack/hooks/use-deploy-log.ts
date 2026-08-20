import { useState, useEffect, useRef } from 'react'
import { connect } from '../../../lib/websocket'

export type LogLevel = 'info' | 'warn' | 'error' | 'success'
export type DeployStatus = 'connecting' | 'running' | 'success' | 'failed'

export interface LogEntry {
  id: string
  timestamp: string
  level: LogLevel
  phase?: string
  step?: string
  message: string
}

interface DeployLogPayload {
  type: 'log' | 'status' | 'progress'
  level?: LogLevel
  phase?: string
  step?: string
  message?: string
  timestamp?: string
  status?: DeployStatus
  progress?: number
  progress_ceiling?: number
}

interface UseDeployLogResult {
  logs: LogEntry[]
  status: DeployStatus
  progress: number
  /** 이 스텝이 끝났을 때 닿을 값. 화면은 그 안에서만 움직인다. */
  progressCeiling: number
  isConnected: boolean
}

export function useDeployLog(deploymentId: string): UseDeployLogResult {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [status, setStatus] = useState<DeployStatus>('connecting')
  const [progress, setProgress] = useState(0)
  const [progressCeiling, setProgressCeiling] = useState(0)
  const [isConnected, setIsConnected] = useState(false)
  const counterRef = useRef(0)

  useEffect(() => {
    if (!deploymentId) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws/deployments/${deploymentId}/logs`

    const client = connect(wsUrl, {
      onMessage: (data) => {
        const payload = data as DeployLogPayload
        if (payload.progress !== undefined && payload.progress > 0) {
          setProgress(payload.progress)
          // 상한도 함께 온다. 없으면 0 으로 두고 화면이 대비책을 쓴다.
          setProgressCeiling(payload.progress_ceiling ?? 0)
        }
        if (payload.message) {
          const entry: LogEntry = {
            id: String(++counterRef.current),
            timestamp: payload.timestamp ?? new Date().toISOString(),
            level: payload.level ?? 'info',
            phase: payload.phase,
            step: payload.step,
            message: payload.message,
          }
          setLogs((prev) => [...prev, entry])
        }
        if (payload.type === 'status' && payload.status) {
          setStatus(payload.status)
        }
      },
      onStatusChange: (connected) => {
        setIsConnected(connected)
        if (connected) {
          setStatus('running')
        }
      },
    })

    return () => {
      client.close()
    }
  }, [deploymentId])

  return { logs, status, progress, progressCeiling, isConnected }
}
