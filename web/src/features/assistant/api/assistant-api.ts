import { useMutation, useQuery } from '@tanstack/react-query'
import { api } from '../../../lib/api'

// AI 어시스턴트(nullus-plan#94). 서버가 Nullus MCP 도구를 Upstage 모델에 물려 답한다.
// 대화는 서버에 저장하지 않는다 — 화면이 들고 있다가 매 질문에 통째로 보낸다.

export interface AssistantStatus {
  enabled: boolean
  provider?: string
  model?: string
}

export interface AssistantTurn {
  role: 'user' | 'assistant'
  content: string
}

export interface AssistantToolTrace {
  name: string
  ok: boolean
}

export interface AssistantReply {
  reply: string
  tools: AssistantToolTrace[]
}

export interface AssistantChatInput {
  messages: AssistantTurn[]
  context: { path: string }
}

export function useAssistantStatus() {
  return useQuery({
    queryKey: ['assistant', 'status'] as const,
    queryFn: () => api.get<AssistantStatus>('/assistant/status').then((r) => r.data),
    // 엔드포인트가 없는 옛 서버(404)도 "꺼짐" 으로 본다 — 다시 물어도 바뀌지 않는다.
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function useAssistantChat() {
  return useMutation({
    mutationFn: (input: AssistantChatInput) =>
      api.post<AssistantReply>('/assistant/chat', input).then((r) => r.data),
  })
}
