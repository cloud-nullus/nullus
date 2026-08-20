type TimeRange = '1h' | '6h' | '24h' | '7d'

export interface EmbedTab {
  id: string
  label: string
  url: string
  order: number
}

// 탭은 뷰(cluster/stack/cicd)가 아니라 "무엇을 보고 있는가" 단위로 저장한다.
//
// v1 은 뷰 단위 키였다. 탭에 담기는 주소는 그 스택의 접속 도메인에서 나오므로
// 스택 A 에서 등록한 탭이 스택 B 를 골라도 그대로 떴다 — 스택별로 주소를 미리
// 채워 주기 시작하면 그대로 남의 도메인을 안내하는 오작동이 된다.
// v1 키는 건드리지 않고 남겨 둔다(지우지 않으므로 되돌릴 여지가 있다).
export const STORAGE_KEY = (viewId: string, scopeId: string) =>
  `nullus_tabs_${viewId}_${scopeId || 'none'}_v2`
export const SKIP_KEY = (viewId: string, scopeId: string) =>
  `nullus_skip_connect_${viewId}_${scopeId || 'none'}_v2`

export function timeAgo(dateStr: string | null): string {
  if (!dateStr) return '—'
  const ms = Date.now() - new Date(dateStr).getTime()
  if (ms < 60_000) return 'just now'
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ago`
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)}h ago`
  return `${Math.floor(ms / 86_400_000)}d ago`
}

export function selectSeries<T extends { ts: number }>(samples: T[], range: TimeRange): T[] {
  if (samples.length === 0) return []
  const now = Date.now()
  const windowMs: Record<TimeRange, number> = {
    '1h': 60 * 60 * 1000,
    '6h': 6 * 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
    '7d': 7 * 24 * 60 * 60 * 1000,
  }
  const cutoff = now - windowMs[range]
  const ranged = samples.filter((s) => s.ts >= cutoff)
  if (ranged.length <= 120) return ranged
  const stride = Math.ceil(ranged.length / 120)
  return ranged.filter((_, idx) => idx % stride === 0)
}

export function formatRangeLabel(ts: number, range: TimeRange): string {
  const date = new Date(ts)
  if (range === '7d') {
    return date.toLocaleDateString('en', { weekday: 'short' })
  }
  return date.toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false })
}

export function formatDuration(startedAt: string | null, completedAt: string | null): string {
  if (!startedAt) return '—'
  const start = new Date(startedAt).getTime()
  const end = completedAt ? new Date(completedAt).getTime() : Date.now()
  const ms = Math.max(0, end - start)
  const totalSec = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSec / 60)
  const seconds = totalSec % 60
  if (minutes === 0) return `${seconds}s`
  return `${minutes}m ${seconds.toString().padStart(2, '0')}s`
}

export function loadTabs(viewId: string, scopeId: string, seedTabs?: EmbedTab[]): EmbedTab[] {
  try {
    const r = localStorage.getItem(STORAGE_KEY(viewId, scopeId))
    if (r) return JSON.parse(r) as EmbedTab[]
    if (seedTabs?.length) {
      localStorage.setItem(STORAGE_KEY(viewId, scopeId), JSON.stringify(seedTabs))
      return seedTabs
    }
  } catch { /* */ }
  return []
}

export function persistTabs(viewId: string, scopeId: string, tabs: EmbedTab[]) {
  try { localStorage.setItem(STORAGE_KEY(viewId, scopeId), JSON.stringify(tabs)) } catch { /* */ }
}

export const normalizeEmbedUrl = (rawUrl: string): string => {
  const trimmed = rawUrl.trim()
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

export const isValidEmbedUrl = (rawUrl: string): boolean => {
  try {
    const parsed = new URL(rawUrl)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export const isKnownNonEmbeddableHost = (rawUrl: string): boolean => {
  try {
    const parsed = new URL(rawUrl)
    return parsed.hostname.toLowerCase() === 'play.grafana.org'
  } catch {
    return false
  }
}
