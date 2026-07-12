type ImportDiff = {
  added: Record<string, unknown>
  removed: Record<string, unknown>
  changed: Record<string, [unknown, unknown]>
}

type ImportPreview = {
  mode: 'create' | 'update'
  name: string
  existing_state?: string
  changes?: ImportDiff
}

const RESOURCE_FIELD_LABELS: Record<string, string> = {
  cpuRequest: 'CPU 요청',
  cpuLimit: 'CPU 제한',
  memoryRequestGi: '메모리 요청',
  memoryLimitGi: '메모리 제한',
  storageRequestGi: '스토리지 요청',
  storageLimitGi: '스토리지 제한',
}

const OPTION_FIELD_LABELS: Record<string, string> = {
  registryCallsPerDay: '일일 레지스트리 호출 수',
  deploymentsPerDay: '일일 배포 횟수',
}

const SLOT_LABELS: Record<string, string> = {
  'artifacts.packageRegistry': 'GitLab Package Registry',
  'artifacts.packageRegistry:gitlab': 'GitLab Package Registry',
  'pipeline.cdTool': 'Argo CD',
  'pipeline.cdTool:argocd': 'Argo CD',
  'monitoring.collection:prometheus': 'Prometheus',
}

const STATE_LABELS: Record<string, string> = {
  pending: '대기',
  validating: '검증 중',
  installing: '설치 중',
  configuring: '설정 중',
  health_check: '상태 점검 중',
  completed: '완료',
  failed: '실패',
  rolling_back: '롤백 중',
  rolled_back: '롤백 완료',
  cancelled: '취소됨',
}

function toSlotLabel(slot: string): string {
  return SLOT_LABELS[slot] ?? slot
}

function formatValue(value: unknown): string {
  if (typeof value === 'number') return String(value)
  if (typeof value === 'string') return value
  if (typeof value === 'boolean') return value ? '활성' : '비활성'
  if (value == null) return '없음'
  return JSON.stringify(value)
}

function formatState(value?: string): string {
  if (!value) return ''
  return STATE_LABELS[value] ?? value
}

function formatChangedEntry(path: string, before: unknown, after: unknown): string {
  const resourceMatch = path.match(/^config\.applied_resource_overrides\.(.+)\.(cpuRequest|cpuLimit|memoryRequestGi|memoryLimitGi|storageRequestGi|storageLimitGi)$/)
  if (resourceMatch) {
    const [, slot, field] = resourceMatch
    return `${toSlotLabel(slot)} ${RESOURCE_FIELD_LABELS[field] ?? field}: ${formatValue(before)} -> ${formatValue(after)}`
  }

  const optionMatch = path.match(/^config\.option_overrides\.(.+)\.([^.]*)$/)
  if (optionMatch) {
    const [, slot, field] = optionMatch
    return `${toSlotLabel(slot)} ${OPTION_FIELD_LABELS[field] ?? field}: ${formatValue(before)} -> ${formatValue(after)}`
  }

  if (path === 'template_id') {
    return `템플릿: ${formatValue(before)} -> ${formatValue(after)}`
  }
  if (path === 'namespace') {
    return `네임스페이스: ${formatValue(before)} -> ${formatValue(after)}`
  }

  return `${path}: ${formatValue(before)} -> ${formatValue(after)}`
}

function formatCount(prefix: string, entries: Record<string, unknown>): string[] {
  const count = Object.keys(entries).length
  return count > 0 ? [`${prefix}: ${count}`] : []
}

export function summarizeImportPreview(preview: ImportPreview): string[] {
  if (preview.mode === 'create') {
    return ['현재 같은 이름의 스택이 없어 새 스택으로 생성됩니다.']
  }

  if (!preview.changes) {
    return ['기존 스택에 import 설정이 적용됩니다.']
  }

  const lines = Object.entries(preview.changes.changed)
    .slice(0, 10)
    .map(([path, [before, after]]) => formatChangedEntry(path, before, after))

  return [
    `기존 스택이 업데이트됩니다${preview.existing_state ? ` (현재 상태: ${formatState(preview.existing_state)})` : ''}.`,
    ...lines,
    ...formatCount('추가되는 필드 수', preview.changes.added),
    ...formatCount('제거되는 필드 수', preview.changes.removed),
  ]
}
