import { useState } from 'react'
import { GitBranch, Search } from 'lucide-react'
import { useCicdTemplates } from '../api/cicd-api'
import type { CicdTemplate } from '../api/cicd-api'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'

const MOCK_TEMPLATES: CicdTemplate[] = [
  {
    id: 'web-backend',
    name: 'Web Backend Pipeline',
    description: 'Node.js / Java / Python 백엔드 서비스를 위한 CI/CD 파이프라인. 유닛 테스트, 도커 빌드, 스테이징 배포를 자동화합니다.',
    appType: 'web-backend',
    stages: ['Build', 'Test', 'Deploy'],
  },
  {
    id: 'web-frontend',
    name: 'Web Frontend Pipeline',
    description: 'React / Vue / Angular 프론트엔드 앱을 위한 CI/CD 파이프라인. 빌드 최적화, E2E 테스트, CDN 배포를 포함합니다.',
    appType: 'web-frontend',
    stages: ['Build', 'Test', 'Deploy'],
  },
  {
    id: 'batch-job',
    name: 'Batch Job Pipeline',
    description: '배치 처리 작업을 위한 CI/CD 파이프라인. 스케줄 기반 실행, 결과 검증, 알림 통합을 지원합니다.',
    appType: 'batch-job',
    stages: ['Build', 'Test', 'Deploy'],
  },
]

const APP_TYPE_COLOR: Record<string, { bg: string; color: string }> = {
  'web-backend': { bg: 'rgba(99,102,241,0.12)', color: '#a5b4fc' },
  'web-frontend': { bg: 'rgba(16,185,129,0.12)', color: '#34d399' },
  'batch-job': { bg: 'rgba(245,158,11,0.12)', color: '#fbbf24' },
}

export function CicdTemplatePage() {
  const { data: apiTemplates } = useCicdTemplates()
  const templates = Array.isArray(apiTemplates) ? apiTemplates : MOCK_TEMPLATES
  const [search, setSearch] = useState('')

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: '28px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
          <div
            style={{
              width: 'var(--icon-size)',
              height: 'var(--icon-size)',
              background: 'rgba(99,102,241,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#818cf8',
            }}
          >
            <GitBranch size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              CI/CD Templates
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              파이프라인 템플릿을 선택하여 빠르게 시작하세요.
            </p>
          </div>
        </div>
      </div>

      {/* Search */}
      <div style={{ marginBottom: '20px', maxWidth: '360px' }}>
        <div style={{ position: 'relative' }}>
          <Search
            size={14}
            style={{
              position: 'absolute',
              left: '10px',
              top: '50%',
              transform: 'translateY(-50%)',
              color: 'var(--color-text-secondary)',
              pointerEvents: 'none',
            }}
          />
          <Input
            placeholder="템플릿 검색..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ paddingLeft: '32px' }}
          />
        </div>
      </div>

      {/* Template cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
          gap: '16px',
        }}
      >
        {filtered.map((template) => {
          const typeColor = APP_TYPE_COLOR[template.appType] ?? APP_TYPE_COLOR['web-backend']
          return (
            <div
              key={template.id}
              style={{
                background: 'var(--color-surface-card)',
                border: '1px solid var(--color-border-default)',
                borderRadius: 'var(--card-radius)',
                padding: 'var(--card-padding)',
                display: 'flex',
                flexDirection: 'column',
                gap: '14px',
                transition: 'border-color var(--transition-default)',
              }}
              onMouseEnter={(e) => {
                ;(e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-hover)'
              }}
              onMouseLeave={(e) => {
                ;(e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-default)'
              }}
            >
              {/* Card header */}
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                  <h3
                    style={{
                      margin: 0,
                      fontSize: '15px',
                      fontWeight: 700,
                      color: 'var(--color-text-primary)',
                    }}
                  >
                    {template.name}
                  </h3>
                  <span
                    style={{
                      padding: '2px 8px',
                      borderRadius: '5px',
                      background: typeColor.bg,
                      color: typeColor.color,
                      fontSize: '11px',
                      fontWeight: 600,
                    }}
                  >
                    {template.appType}
                  </span>
                </div>
                <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>
                  {template.description}
                </p>
              </div>

              {/* Stages */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
                {template.stages.map((stage, idx) => (
                  <div key={stage} style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <span
                      style={{
                        padding: '3px 10px',
                        borderRadius: '6px',
                        background: 'rgba(99,102,241,0.12)',
                        color: '#a5b4fc',
                        fontSize: '11px',
                        fontWeight: 600,
                      }}
                    >
                      {stage}
                    </span>
                    {idx < template.stages.length - 1 && (
                      <span style={{ color: 'var(--color-text-muted)', fontSize: '11px' }}>→</span>
                    )}
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div
                style={{
                  paddingTop: '10px',
                  borderTop: '1px solid var(--color-border-default)',
                  display: 'flex',
                  justifyContent: 'flex-end',
                }}
              >
                <Button variant="primary" size="sm">
                  Use Template
                </Button>
              </div>
            </div>
          )
        })}
      </div>

      {filtered.length === 0 && (
        <div
          style={{
            textAlign: 'center',
            padding: '60px 0',
            color: 'var(--color-text-secondary)',
            fontSize: '14px',
          }}
        >
          검색 결과가 없습니다.
        </div>
      )}
    </div>
  )
}
