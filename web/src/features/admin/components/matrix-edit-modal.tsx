import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2, Plus } from 'lucide-react'
import { Modal } from '../../../components/ui/modal'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { NativeSelect } from '../../../components/ui/native-select'
import type { CompatibilityMatrix } from '../../../types'
import { useCreateMatrix, useUpdateMatrix, type MatrixInput } from '../../stack/api/stack-api'

// F8-Phase5 admin matrix editor. Kept deliberately compact — the backend
// validates payload shape, so this component focuses on ergonomic input for
// the happy-path fields: identity / Kubernetes range / tools table.
// Deep validation (semver, dup id) is left to the server; the form only
// enforces presence and option-set constraints client-side.

interface MatrixEditModalProps {
  open: boolean
  onClose: () => void
  mode: 'create' | 'edit'
  initial?: CompatibilityMatrix
  onSaved?: (matrixId: string) => void
}

interface ToolRow {
  category: string
  name: string
  helmVersion: string
  appVersion: string
  minK8sVersion: string
  archSupport: string[]
  tier: 'stable' | 'beta' | 'deprecated'
}

const DEFAULT_ROW: ToolRow = {
  category: '',
  name: '',
  helmVersion: '',
  appVersion: '',
  minK8sVersion: '',
  archSupport: ['amd64'],
  tier: 'stable',
}

function toolsToRows(m?: CompatibilityMatrix): ToolRow[] {
  if (!m) return [{ ...DEFAULT_ROW }]
  // CompatibilityMatrix.tools is an array in the normalized frontend type;
  // we need a stable seed — if empty, start with one blank row.
  const rows: ToolRow[] = (m.tools ?? []).map((t) => ({
    category: (t as unknown as { category?: string }).category ?? '',
    name: t.name,
    helmVersion: t.helmVersion,
    appVersion: t.appVersion,
    minK8sVersion: t.minK8sVersion ?? '',
    archSupport: t.archSupport && t.archSupport.length > 0 ? t.archSupport : ['amd64'],
    tier: (t.tier ?? 'stable') as ToolRow['tier'],
  }))
  return rows.length > 0 ? rows : [{ ...DEFAULT_ROW }]
}

// F8-UIUX-MatrixEditValidation — client-side field-level rules. The backend
// still owns the final verdict, but surfacing these early avoids the "Save
// button is dead and I don't know why" state and catches typos before
// round-tripping.
const ID_PATTERN = /^[a-z0-9][a-z0-9-]{1,63}$/
const K8S_PATTERN = /^v\d+\.\d+(\.\d+)?$/

interface FieldErrors {
  id?: string
  name?: string
  k8sMin?: string
  k8sMax?: string
  k8sRec?: string
}

type Translator = (key: string, defaultValue?: string) => string

function validateFields(args: {
  id: string
  name: string
  k8sMin: string
  k8sMax: string
  k8sRec: string
  t: Translator
  isEdit: boolean
}): FieldErrors {
  const { id, name, k8sMin, k8sMax, k8sRec, t, isEdit } = args
  const errors: FieldErrors = {}
  // ID is immutable in edit mode, so don't block Save on a legacy-bad id.
  if (!isEdit && id.trim() && !ID_PATTERN.test(id.trim())) {
    errors.id = t(
      'stackVersionsAdmin.modal.validation.idFormat',
      'ID는 소문자·숫자·하이픈만 허용되며 소문자·숫자로 시작해야 합니다.',
    )
  }
  if (name.trim() && name.trim().length < 2) {
    errors.name = t(
      'stackVersionsAdmin.modal.validation.nameLength',
      '이름은 최소 2자 이상이어야 합니다.',
    )
  }
  if (k8sMin.trim() && !K8S_PATTERN.test(k8sMin.trim())) {
    errors.k8sMin = t('stackVersionsAdmin.modal.validation.k8sFormat', '형식 예: v1.28 또는 v1.28.3')
  }
  if (k8sMax.trim() && !K8S_PATTERN.test(k8sMax.trim())) {
    errors.k8sMax = t('stackVersionsAdmin.modal.validation.k8sFormat', '형식 예: v1.28 또는 v1.28.3')
  }
  if (k8sRec.trim() && !K8S_PATTERN.test(k8sRec.trim())) {
    errors.k8sRec = t('stackVersionsAdmin.modal.validation.k8sFormat', '형식 예: v1.28 또는 v1.28.3')
  }
  return errors
}

function rowsToPayload(rows: ToolRow[]): MatrixInput['tools'] {
  const out: MatrixInput['tools'] = {}
  for (const row of rows) {
    const cat = row.category.trim()
    if (!cat) continue
    out[cat] = {
      name: row.name.trim(),
      helmVersion: row.helmVersion.trim(),
      appVersion: row.appVersion.trim(),
      minK8sVersion: row.minK8sVersion.trim(),
      archSupport: row.archSupport,
      tier: row.tier,
    }
  }
  return out
}

export function MatrixEditModal({ open, onClose, mode, initial, onSaved }: MatrixEditModalProps) {
  const { t } = useTranslation()
  const isEdit = mode === 'edit'
  const createMutation = useCreateMatrix()
  const updateMutation = useUpdateMatrix()
  const mutation = isEdit ? updateMutation : createMutation

  const [id, setId] = useState('')
  const [name, setName] = useState('')
  const [status, setStatus] = useState<'verified' | 'untested' | 'unsupported'>('untested')
  const [k8sMin, setK8sMin] = useState('')
  const [k8sMax, setK8sMax] = useState('')
  const [k8sRec, setK8sRec] = useState('')
  const [rows, setRows] = useState<ToolRow[]>([{ ...DEFAULT_ROW }])
  const [error, setError] = useState<string | null>(null)
  // F8-UIUX-MatrixEditDirty — when editing, Save writes a full replacement.
  // A row whose category is empty is silently dropped by rowsToPayload(),
  // which can remove an existing tool definition the user never intended
  // to delete. `confirmDrop` makes the first Save click a preview: render a
  // warning banner with the rows that would vanish; require a second click
  // to actually commit.
  const [confirmDrop, setConfirmDrop] = useState(false)

  const droppedRows = useMemo(
    () =>
      rows.filter(
        (r) =>
          !r.category.trim() &&
          (r.name.trim() || r.helmVersion.trim() || r.appVersion.trim()),
      ),
    [rows],
  )

  // Re-seed state whenever the modal opens with a different `initial`.
  useEffect(() => {
    if (!open) return
    setError(null)
    setId(initial?.id ?? '')
    setName(initial?.name ?? '')
    setStatus((initial?.status ?? 'untested') as typeof status)
    // The CompatibilityMatrix type stores k8sRange as a single string; we
    // still allow split min/max/recommended input. When editing, seed from
    // the range string if min/max are embedded; otherwise leave empty.
    const range = initial?.k8sRange ?? ''
    const parts = range.split('-')
    setK8sMin(parts[0] ?? '')
    setK8sMax(parts[1] ?? parts[0] ?? '')
    setK8sRec(parts[1] ?? parts[0] ?? '')
    setRows(toolsToRows(initial))
    setConfirmDrop(false)
  }, [open, initial])

  // Reset the confirm gate as soon as the user clears the dirty rows —
  // otherwise Save would stay in "second-click commits" mode even after
  // the user has actually fixed the input.
  useEffect(() => {
    if (droppedRows.length === 0) setConfirmDrop(false)
  }, [droppedRows.length])

  const fieldErrors = useMemo(
    () => validateFields({ id, name, k8sMin, k8sMax, k8sRec, t, isEdit }),
    [id, name, k8sMin, k8sMax, k8sRec, t, isEdit],
  )
  const hasFieldErrors = Object.values(fieldErrors).some(Boolean)

  // F8-UIUX-MatrixEditValidation — snapshot the form on modal open so Cancel
  // can prompt the user before discarding work. Using JSON.stringify keeps
  // the comparison shallow-but-deterministic across field types.
  const initialSnapshot = useMemo(() => {
    if (!open) return ''
    return JSON.stringify({
      id: initial?.id ?? '',
      name: initial?.name ?? '',
      status: initial?.status ?? 'untested',
      k8sRange: initial?.k8sRange ?? '',
      tools: toolsToRows(initial),
    })
  }, [open, initial])
  const currentSnapshot = useMemo(
    () => JSON.stringify({ id, name, status, k8sRange: `${k8sMin}-${k8sMax}`, tools: rows }),
    [id, name, status, k8sMin, k8sMax, rows],
  )
  const isDirty = currentSnapshot !== initialSnapshot

  const canSubmit = useMemo(() => {
    if (hasFieldErrors) return false
    if (!id.trim() || !name.trim()) return false
    if (!k8sMin.trim() || !k8sMax.trim() || !k8sRec.trim()) return false
    const filled = rows.filter((r) => r.category.trim() && r.name.trim())
    if (filled.length === 0) return false
    return true
  }, [hasFieldErrors, id, name, k8sMin, k8sMax, k8sRec, rows])

  const handleSubmit = () => {
    if (!canSubmit) return
    if (isEdit && droppedRows.length > 0 && !confirmDrop) {
      setConfirmDrop(true)
      return
    }
    const input: MatrixInput = {
      id: id.trim(),
      name: name.trim(),
      status,
      kubernetes: { min: k8sMin.trim(), max: k8sMax.trim(), recommended: k8sRec.trim() },
      tools: rowsToPayload(rows),
    }
    mutation.mutate(input, {
      onSuccess: (saved) => {
        onSaved?.(saved.id ?? input.id)
        onClose()
      },
      onError: (err) => {
        const msg = (err as { message?: string })?.message ?? 'Save failed'
        setError(msg)
      },
    })
  }

  const addRow = () => setRows((prev) => [...prev, { ...DEFAULT_ROW }])
  const removeRow = (idx: number) =>
    setRows((prev) => (prev.length > 1 ? prev.filter((_, i) => i !== idx) : prev))

  const updateRow = (idx: number, patch: Partial<ToolRow>) =>
    setRows((prev) => prev.map((r, i) => (i === idx ? { ...r, ...patch } : r)))

  const toggleArch = (idx: number, arch: string) =>
    updateRow(idx, {
      archSupport: rows[idx].archSupport.includes(arch)
        ? rows[idx].archSupport.filter((a) => a !== arch)
        : [...rows[idx].archSupport, arch].sort(),
    })

  // F8-UIUX-MatrixEditValidation — guard close paths (Cancel button, ESC,
  // backdrop click) with a native confirm when the form is dirty so users
  // do not lose typed edits by accident.
  const handleCloseWithGuard = () => {
    if (isDirty && !window.confirm(
      t(
        'stackVersionsAdmin.modal.unsavedPrompt',
        '저장되지 않은 변경사항이 있습니다. 닫으시겠습니까?',
      ),
    )) {
      return
    }
    onClose()
  }

  return (
    <Modal
      open={open}
      onClose={handleCloseWithGuard}
      title={t(
        isEdit ? 'stackVersionsAdmin.modal.titleEdit' : 'stackVersionsAdmin.modal.titleCreate',
        isEdit ? 'Edit matrix' : 'New matrix',
      )}
      wide
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button variant="outline" onClick={handleCloseWithGuard} disabled={mutation.isPending}>
            {t('stackVersionsAdmin.modal.cancel', 'Cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || mutation.isPending}>
            {t('stackVersionsAdmin.modal.save', 'Save')}
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-3 text-sm">
        {error && (
          <div className="rounded border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-[#fca5a5]">
            {error}
          </div>
        )}

        {confirmDrop && droppedRows.length > 0 && (
          <div
            className="rounded border border-[rgba(245,158,11,0.35)] bg-[rgba(245,158,11,0.08)] px-3 py-2 text-[#fcd34d]"
            data-testid="matrix-drop-warn"
          >
            <div className="mb-1 font-semibold">
              {t(
                'stackVersionsAdmin.modal.dropWarn.title',
                '카테고리가 비어 있어 저장 시 제거되는 행',
              )}
            </div>
            <ul className="ml-4 list-disc text-[11px]">
              {droppedRows.map((r, i) => (
                <li key={i}>
                  {r.name || t('stackVersionsAdmin.modal.dropWarn.unnamed', '(이름 없음)')}
                  {r.helmVersion ? ` — helm ${r.helmVersion}` : ''}
                </li>
              ))}
            </ul>
            <div className="mt-2 text-[11px] text-[var(--color-text-secondary)]">
              {t(
                'stackVersionsAdmin.modal.dropWarn.hint',
                '카테고리를 입력하거나 해당 행을 삭제한 뒤 저장해 주세요. 그대로 저장하면 해당 도구 정의는 서버에서 제거됩니다. 계속하려면 Save 를 한 번 더 눌러 주세요.',
              )}
            </div>
          </div>
        )}

        <section className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input
            label="ID"
            placeholder="my-matrix-v1"
            value={id}
            disabled={isEdit}
            onChange={(e) => setId(e.target.value)}
            error={fieldErrors.id}
          />
          <Input
            label="Name"
            placeholder="My Matrix"
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={fieldErrors.name}
          />
          <NativeSelect
            label="Status"
            value={status}
            onChange={(e) => setStatus(e.target.value as typeof status)}
          >
            <option value="verified">verified</option>
            <option value="untested">untested</option>
            <option value="unsupported">unsupported</option>
          </NativeSelect>
        </section>

        <section className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Input
            label="K8s min"
            placeholder="v1.27"
            value={k8sMin}
            onChange={(e) => setK8sMin(e.target.value)}
            error={fieldErrors.k8sMin}
          />
          <Input
            label="K8s max"
            placeholder="v1.29"
            value={k8sMax}
            onChange={(e) => setK8sMax(e.target.value)}
            error={fieldErrors.k8sMax}
          />
          <Input
            label="K8s recommended"
            placeholder="v1.28"
            value={k8sRec}
            onChange={(e) => setK8sRec(e.target.value)}
            error={fieldErrors.k8sRec}
          />
        </section>

        <section>
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-medium text-[var(--color-text-primary)]">
              {t('stackVersionsAdmin.modal.tools', 'Tools')}
            </span>
            <Button size="sm" variant="outline" onClick={addRow} type="button">
              <Plus size={12} />
              {t('stackVersionsAdmin.modal.addTool', 'Add tool')}
            </Button>
          </div>
          <div className="flex flex-col gap-2">
            {rows.map((row, idx) => (
              <div
                key={idx}
                className="grid grid-cols-1 gap-2 rounded border border-[var(--color-border-default)] p-2 md:grid-cols-[1fr_1fr_0.9fr_0.9fr_0.9fr_1fr_auto]"
              >
                <Input
                  label="Category"
                  placeholder="db"
                  value={row.category}
                  onChange={(e) => updateRow(idx, { category: e.target.value })}
                />
                <Input
                  label="Name"
                  placeholder="Postgres"
                  value={row.name}
                  onChange={(e) => updateRow(idx, { name: e.target.value })}
                />
                <Input
                  label="Helm"
                  placeholder="12.0.0"
                  value={row.helmVersion}
                  onChange={(e) => updateRow(idx, { helmVersion: e.target.value })}
                />
                <Input
                  label="App"
                  placeholder="16.0"
                  value={row.appVersion}
                  onChange={(e) => updateRow(idx, { appVersion: e.target.value })}
                />
                <Input
                  label="Min K8s"
                  placeholder="(optional)"
                  value={row.minK8sVersion}
                  onChange={(e) => updateRow(idx, { minK8sVersion: e.target.value })}
                />
                <div className="flex flex-col gap-1">
                  <NativeSelect
                    label="Tier"
                    value={row.tier}
                    onChange={(e) => updateRow(idx, { tier: e.target.value as ToolRow['tier'] })}
                  >
                    <option value="stable">stable</option>
                    <option value="beta">beta</option>
                    <option value="deprecated">deprecated</option>
                  </NativeSelect>
                  <div className="flex items-center gap-2 pt-1 text-[11px]">
                    {['amd64', 'arm64'].map((a) => (
                      <label key={a} className="inline-flex items-center gap-1">
                        <input
                          type="checkbox"
                          checked={row.archSupport.includes(a)}
                          onChange={() => toggleArch(idx, a)}
                        />
                        {a}
                      </label>
                    ))}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => removeRow(idx)}
                  disabled={rows.length <= 1}
                  className="self-end rounded border border-[var(--color-border-default)] p-1 text-[var(--color-text-secondary)] hover:text-[#ef4444] disabled:opacity-40"
                  aria-label={t('stackVersionsAdmin.modal.removeTool', 'Remove tool')}
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        </section>
      </div>
    </Modal>
  )
}
