export type MessageCallback = (data: unknown) => void
export type StatusCallback = (connected: boolean) => void

interface WebSocketClientOptions {
  onMessage: MessageCallback
  onStatusChange?: StatusCallback
  maxRetries?: number
}

export interface WebSocketClient {
  close: () => void
}

export function connect(url: string, options: WebSocketClientOptions): WebSocketClient {
  const { onMessage, onStatusChange, maxRetries = 10 } = options
  let ws: WebSocket | null = null
  let retryCount = 0
  let closed = false
  let retryTimer: ReturnType<typeof setTimeout> | null = null

  function open() {
    if (closed) return
    ws = new WebSocket(url)

    ws.onopen = () => {
      retryCount = 0
      onStatusChange?.(true)
    }

    ws.onmessage = (event) => {
      try {
        const parsed: unknown = JSON.parse(event.data as string)
        onMessage(parsed)
      } catch {
        onMessage(event.data)
      }
    }

    ws.onclose = () => {
      onStatusChange?.(false)
      if (!closed && retryCount < maxRetries) {
        const delay = Math.min(1000 * 2 ** retryCount, 30000)
        retryCount++
        retryTimer = setTimeout(open, delay)
      }
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  open()

  return {
    close() {
      closed = true
      if (retryTimer !== null) clearTimeout(retryTimer)
      ws?.close()
    },
  }
}
