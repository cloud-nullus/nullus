import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Search } from 'lucide-react'
import { useCicdTemplates } from '../api/cicd-api'
import type { CicdTemplate } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'

const APP_TYPE_COLOR: Record<string, { bg: string; color: string }> = {
  'web-backend': { bg: 'rgba(99,102,241,0.12)', color: '#a5b4fc' },
  'web-frontend': { bg: 'rgba(16,185,129,0.12)', color: '#34d399' },
  'batch-job': { bg: 'rgba(245,158,11,0.12)', color: '#fbbf24' },
}

export function CicdTemplatePage() {
  const navigate = useNavigate()
  const { data: apiTemplates } = useCicdTemplates()
  const templates = Array.isArray(apiTemplates) ? apiTemplates : []
  const [search, setSearch] = useState('')

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div>
      {/* Page header */}
      <div className="mb-7">
        <div className="mb-2 flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <GitBranch size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              CI/CD Templates
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              파이프라인 템플릿을 선택하여 빠르게 시작하세요.
            </p>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="mb-5 max-w-[360px]">
        <div className="relative">
          <Search
            size={14}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
          />
          <Input
            placeholder="템플릿 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      {/* Template cards */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
        {filtered.map((template) => {
          const typeColor = APP_TYPE_COLOR[template.appType] ?? APP_TYPE_COLOR['web-backend']
          return (
            <div
              key={template.id}
              className="flex flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-150"
            >
              {/* Card header */}
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
                    {template.name}
                  </h3>
                  <span
                    className="rounded-[5px] px-2 py-0.5 text-[11px] font-semibold"
                    style={{ backgroundColor: typeColor.bg, color: typeColor.color }}
                  >
                    {template.appType}
                  </span>
                </div>
                <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                  {template.description}
                </p>
              </div>

              {/* Stages */}
              <div className="flex flex-wrap items-center gap-1">
                {template.stages.map((stage, idx) => (
                  <div key={stage} className="flex items-center gap-1">
                    <span
                      className="rounded-md bg-[rgba(99,102,241,0.12)] px-2.5 py-[3px] text-[11px] font-semibold text-[#a5b4fc]"
                    >
                      {stage}
                    </span>
                    {idx < template.stages.length - 1 && (
                      <span className="text-[11px] text-[var(--color-text-muted)]">→</span>
                    )}
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div className="flex justify-end border-t border-[var(--color-border-default)] pt-2.5">
                <Button
                   variant="primary"
                   size="sm"
                   type="button"
                   onClick={() => navigate('/cicd/list?template=' + template.id)}
                 >
                   Use Template
                 </Button>
              </div>
            </div>
          )
        })}
      </div>

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          검색 결과가 없습니다.
        </div>
      )}
    </div>
  )
}
