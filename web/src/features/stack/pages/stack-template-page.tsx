import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BookOpen, Clock, Search } from 'lucide-react'
import { useTemplates } from '../api/stack-api'
import { useStackConfigStore } from '../stores/stack-config-store'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'

const MOCK_TEMPLATES = [
  {
    id: 'gitlab-all-in-one',
    name: 'GitLab All-in-One',
    description: 'GitLab을 중심으로 소스, 컨테이너 레지스트리, CI/CD, 모니터링을 통합하는 올인원 스택.',
    tools: ['GitLab', 'GitLab CI', 'GitLab Registry', 'Prometheus', 'Grafana', 'OpenSearch'],
    estimatedMinutes: 25,
    category: 'gitlab',
  },
  {
    id: 'gitlab-argocd',
    name: 'GitLab + ArgoCD',
    description: 'GitLab으로 소스/CI를 관리하고 ArgoCD로 GitOps 기반 CD를 구현하는 하이브리드 스택.',
    tools: ['GitLab', 'GitLab CI', 'ArgoCD', 'Prometheus', 'Grafana', 'OpenTelemetry'],
    estimatedMinutes: 30,
    category: 'hybrid',
  },
  {
    id: 'github-argocd',
    name: 'GitHub + ArgoCD',
    description: 'GitHub Actions로 CI를 처리하고 ArgoCD로 쿠버네티스 배포를 자동화하는 클라우드 네이티브 스택.',
    tools: ['GitHub', 'GitHub Actions', 'ArgoCD', 'Prometheus', 'Grafana', 'OpenTelemetry', 'OpenSearch'],
    estimatedMinutes: 20,
    category: 'github',
  },
]

export function StackTemplatePage() {
  const navigate = useNavigate()
  const { data: apiTemplates } = useTemplates()
  const { setTemplate, loadFromTemplate } = useStackConfigStore()
  const [search, setSearch] = useState('')

  const templates = apiTemplates ?? MOCK_TEMPLATES

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.description.toLowerCase().includes(search.toLowerCase()) ||
      t.tools.some((tool) => tool.toLowerCase().includes(search.toLowerCase()))
  )

  const handleUseTemplate = (templateId: string) => {
    setTemplate(templateId)
    loadFromTemplate(templateId)
    navigate('/stack/install')
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: '28px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
          <div
            style={{
              width: 'var(--icon-size)',
              height: 'var(--icon-size)',
              background: 'rgba(16,185,129,0.15)',
              borderRadius: 'var(--icon-radius)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#34d399',
            }}
          >
            <BookOpen size={18} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
              Golden Path Templates
            </h1>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              검증된 DevSecOps 스택 템플릿을 선택하여 빠르게 시작하세요.
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
          gap: 'var(--grid-gap)',
        }}
      >
        {filtered.map((template) => (
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
              <h3
                style={{
                  margin: '0 0 6px 0',
                  fontSize: '15px',
                  fontWeight: 700,
                  color: 'var(--color-text-primary)',
                }}
              >
                {template.name}
              </h3>
              <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>
                {template.description}
              </p>
            </div>

            {/* Tools */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
              {template.tools.map((tool) => (
                <span
                  key={tool}
                  style={{
                    padding: '3px 8px',
                    borderRadius: '6px',
                    background: 'rgba(99,102,241,0.12)',
                    color: '#a5b4fc',
                    fontSize: '11px',
                    fontWeight: 500,
                  }}
                >
                  {tool}
                </span>
              ))}
            </div>

            {/* Footer */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                paddingTop: '10px',
                borderTop: '1px solid var(--color-border-default)',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '5px',
                  fontSize: '12px',
                  color: 'var(--color-text-secondary)',
                }}
              >
                <Clock size={13} />
                <span>약 {template.estimatedMinutes}분</span>
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={() => handleUseTemplate(template.id)}
              >
                Use Template
              </Button>
            </div>
          </div>
        ))}
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
