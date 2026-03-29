import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Database, Plus, Save } from 'lucide-react'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { useResourceDefaults, useUpsertResourceDefault } from '../api/stack-api'
import type { StackResourceDefault } from '../../../types'

type EditableRow = Omit<StackResourceDefault, 'updated_at'> & { updated_at?: string }

const EMPTY_ROW: EditableRow = {
  tool_key: '',
  display_name: '',
  cpu_request: 0.5,
  cpu_limit: 1,
  memory_request_gi: 1,
  memory_limit_gi: 2,
  storage_request_gi: 0,
  storage_limit_gi: 0,
  is_default: true,
}

export function StackOssResourceDefaultPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useResourceDefaults()
  const upsertMutation = useUpsertResourceDefault()
  const [draftRows, setDraftRows] = useState<Record<string, EditableRow>>({})
  const [newRow, setNewRow] = useState<EditableRow>(EMPTY_ROW)
  const [error, setError] = useState<string | null>(null)

  const rows = useMemo(
    () => (data?.items ?? []).map((row) => draftRows[row.tool_key] ?? row),
    [data?.items, draftRows]
  )

  const hasData = useMemo(() => rows.length > 0, [rows])

  const updateRow = (toolKey: string, patch: Partial<EditableRow>) => {
    setDraftRows((prev) => ({
      ...prev,
      [toolKey]: {
        ...(prev[toolKey] ?? rows.find((row) => row.tool_key === toolKey) ?? EMPTY_ROW),
        ...patch,
      },
    }))
  }

  const handleSave = (row: EditableRow) => {
    setError(null)
    upsertMutation.mutate(
      {
        tool_key: row.tool_key.trim().toLowerCase(),
        display_name: row.display_name.trim(),
        cpu_request: Number(row.cpu_request),
        cpu_limit: Number(row.cpu_limit),
        memory_request_gi: Number(row.memory_request_gi),
        memory_limit_gi: Number(row.memory_limit_gi),
        storage_request_gi: Number(row.storage_request_gi),
        storage_limit_gi: Number(row.storage_limit_gi),
        is_default: row.is_default,
      },
      {
        onSuccess: () => {
          setDraftRows((prev) => {
            const next = { ...prev }
            delete next[row.tool_key]
            return next
          })
        },
        onError: (e) => {
          const message = e instanceof Error ? e.message : 'OSS Default Resource 저장 중 오류가 발생했습니다.'
          setError(message)
        },
      }
    )
  }

  const handleCreate = () => {
    if (!newRow.tool_key.trim() || !newRow.display_name.trim()) {
      setError('tool_key와 display_name은 필수입니다.')
      return
    }

    handleSave(newRow)
    setNewRow(EMPTY_ROW)
  }

  return (
    <div>
      <Breadcrumb items={[{ label: 'OSS Default Resource' }]} />

      <div className="mb-7 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(59,130,246,0.15)] text-[#60a5fa]">
            <Database size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">OSS Default Resource</h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {t('stackOssDefault.description', 'View, edit, and register default OSS request/limit resources for DevSecOps Stack.')}
            </p>
          </div>
        </div>
      </div>

      <div className="mb-5 rounded-lg border border-[rgba(59,130,246,0.35)] bg-[rgba(59,130,246,0.08)] px-4 py-3 text-sm text-[var(--color-text-primary)]">
        {t('stackOssDefault.contract.prefix', 'Contract:')} <code>POST /api/v1/stacks/resource-defaults</code> {t('stackOssDefault.contract.middle', 'is an idempotent upsert by')} <code>tool_key</code>{t('stackOssDefault.contract.end', '.')}.
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-[rgba(239,68,68,0.35)] bg-[rgba(239,68,68,0.08)] px-4 py-3 text-sm text-[#fca5a5]">
          {error}
        </div>
      )}

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <div className="border-b border-[var(--color-border-default)] px-5 py-3 text-sm font-bold text-[var(--color-text-primary)]">
          OSS Default Resource Request/Limit Defaults
        </div>

        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-[rgba(255,255,255,0.02)]">
              {[
                'Tool Key',
                'Display Name',
                'CPU Request',
                'CPU Limit',
                'Memory Req (Gi)',
                'Memory Limit (Gi)',
                'Action',
              ].map((header) => (
                <th
                  key={header}
                  className="px-[14px] py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]"
                >
                  {header}
                </th>
              ))}
            </tr>
          </thead>

          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={7} className="border-t border-[var(--color-border-default)] px-[14px] py-6 text-center text-sm text-[var(--color-text-secondary)]">
                  Loading OSS default resources...
                </td>
              </tr>
            )}

            {!isLoading && !hasData && (
              <tr>
                <td colSpan={7} className="border-t border-[var(--color-border-default)] px-[14px] py-6 text-center text-sm text-[var(--color-text-secondary)]">
                  등록된 OSS Default Resource가 없습니다.
                </td>
              </tr>
            )}

            {rows.map((row) => (
              <tr key={row.tool_key}>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input value={row.tool_key} onChange={(e) => updateRow(row.tool_key, { tool_key: e.target.value })} className="w-[150px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input value={row.display_name} onChange={(e) => updateRow(row.tool_key, { display_name: e.target.value })} className="w-[180px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input type="number" step="0.01" value={row.cpu_request} onChange={(e) => updateRow(row.tool_key, { cpu_request: Number(e.target.value) })} className="w-[96px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input type="number" step="0.01" value={row.cpu_limit} onChange={(e) => updateRow(row.tool_key, { cpu_limit: Number(e.target.value) })} className="w-[96px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input type="number" step="0.01" value={row.memory_request_gi} onChange={(e) => updateRow(row.tool_key, { memory_request_gi: Number(e.target.value) })} className="w-[108px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Input type="number" step="0.01" value={row.memory_limit_gi} onChange={(e) => updateRow(row.tool_key, { memory_limit_gi: Number(e.target.value) })} className="w-[108px]" />
                </td>
                <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                  <Button size="sm" variant="secondary" onClick={() => handleSave(row)} loading={upsertMutation.isPending}>
                    <Save size={14} /> Save
                  </Button>
                </td>
              </tr>
            ))}

            <tr>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input value={newRow.tool_key} onChange={(e) => setNewRow((prev) => ({ ...prev, tool_key: e.target.value }))} placeholder="tool key" className="w-[150px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input value={newRow.display_name} onChange={(e) => setNewRow((prev) => ({ ...prev, display_name: e.target.value }))} placeholder="display name" className="w-[180px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input type="number" step="0.01" value={newRow.cpu_request} onChange={(e) => setNewRow((prev) => ({ ...prev, cpu_request: Number(e.target.value) }))} className="w-[96px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input type="number" step="0.01" value={newRow.cpu_limit} onChange={(e) => setNewRow((prev) => ({ ...prev, cpu_limit: Number(e.target.value) }))} className="w-[96px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input type="number" step="0.01" value={newRow.memory_request_gi} onChange={(e) => setNewRow((prev) => ({ ...prev, memory_request_gi: Number(e.target.value) }))} className="w-[108px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Input type="number" step="0.01" value={newRow.memory_limit_gi} onChange={(e) => setNewRow((prev) => ({ ...prev, memory_limit_gi: Number(e.target.value) }))} className="w-[108px]" />
              </td>
              <td className="border-t border-[var(--color-border-default)] px-[14px] py-3">
                <Button size="sm" variant="primary" onClick={handleCreate} loading={upsertMutation.isPending}>
                  <Plus size={14} /> Register
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  )
}
