import { useAuthStore } from '../stores/auth-store'

export type MessageCallback = (data: unknown) => void
export type StatusCallback = (connected: boolean) => void

// 브라우저 WebSocket API 는 임의 헤더를 못 붙인다. 통제 가능한 값은 서브프로토콜뿐이라
// 토큰을 여기에 실어 보내고, 서버가 Authorization 헤더로 옮겨 평소대로 검증한다.
// (서버: internal/shared/middleware/ws_subprotocol.go)
// 쿼리 파라미터를 쓰지 않는 이유는 토큰이 액세스 로그·프록시 로그에 남기 때문이다.
const BEARER_SUBPROTOCOL = 'bearer'

function authProtocols(): string[] | undefined {
  const { token } = useAuthStore.getState()
  // 토큰이 없으면 아예 생략한다. 빈 값을 넣으면 핸드셰이크가 깨진다.
  return token ? [BEARER_SUBPROTOCOL, token] : undefined
}

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
    // 재연결마다 다시 읽는다 — 토큰이 갱신됐을 수 있다.
    const protocols = authProtocols()
    ws = protocols ? new WebSocket(url, protocols) : new WebSocket(url)

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
