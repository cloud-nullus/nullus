import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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

// signal 로 진행 중인 질문을 끊는다 — 새 대화를 시작하면 이전 답을 기다리지 않는다.
export function useAssistantChat() {
  return useMutation({
    mutationFn: ({ input, signal }: { input: AssistantChatInput; signal: AbortSignal }) =>
      api.post<AssistantReply>('/assistant/chat', input, { signal }).then((r) => r.data),
  })
}

// ── 관리: 조직의 모델 공급자 · 모델 · API 키 ─────────────────────────────────────────────

export type AssistantProviderId = 'upstage' | 'openai' | 'anthropic' | 'gemini' | 'openai-compatible'

export interface AssistantProvider {
  id: AssistantProviderId
  protocol: 'openai' | 'anthropic'
  default_base_url?: string
  default_model?: string
  key_required: boolean
}

export interface AssistantSettings {
  enabled: boolean
  provider: AssistantProviderId
  base_url: string
  model: string
  has_key: boolean
  key_hint: string
  // 키가 저장돼 있지만 복호화하지 못했다(서버의 ENCRYPTION_KEY 가 바뀜) — 다시 넣어야 한다.
  key_unreadable?: boolean
  effective_base_url: string
  effective_model: string
  updated_at?: string
  updated_by?: string
  providers: AssistantProvider[]
}

// api_key 가 빠지면 저장된 키를 그대로 쓰고(공급자와 주소가 같을 때), 빈 문자열이면 지운다.
export interface AssistantSettingsInput {
  enabled: boolean
  provider: AssistantProviderId
  base_url: string
  model: string
  api_key?: string
}

export interface AssistantTestResult {
  ok: boolean
  model: string
  error: string
  latency_ms: number
}

const settingsKey = ['assistant', 'settings'] as const

export function useAssistantSettings() {
  return useQuery({
    queryKey: settingsKey,
    queryFn: () => api.get<AssistantSettings>('/admin/assistant').then((r) => r.data),
    retry: false,
  })
}

export function useSaveAssistantSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: AssistantSettingsInput) => api.put<AssistantSettings>('/admin/assistant', input).then((r) => r.data),
    onSuccess: (data) => {
      qc.setQueryData(settingsKey, data)
      // 켜고 끄면 패널 버튼이 바로 따라오게 한다.
      void qc.invalidateQueries({ queryKey: ['assistant', 'status'] })
    },
  })
}

export function useDeleteAssistantSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.delete('/admin/assistant').then(() => undefined),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: settingsKey })
      void qc.invalidateQueries({ queryKey: ['assistant', 'status'] })
    },
  })
}

export function useTestAssistantSettings() {
  return useMutation({
    mutationFn: (input: AssistantSettingsInput) =>
      api.post<AssistantTestResult>('/admin/assistant/test', input).then((r) => r.data),
  })
}
