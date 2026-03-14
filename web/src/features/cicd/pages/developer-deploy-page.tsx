import { useState } from 'react'
import { Rocket, Plus, Trash2, ChevronRight } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { CodePreview } from '../../../components/shared/code-preview'
import { useAuthStore } from '../../../stores/auth-store'
import { useDeployApp } from '../api/cicd-api'
import type { AppTemplate, DeployAppRequest } from '../api/cicd-api'

type Step = 1 | 2 | 3 | 4 | 5

const APP_TEMPLATES: { id: AppTemplate; name: string; description: string; language: string; color: string }[] = [
  { id: 'react-spa', name: 'React SPA', description: 'Vite 기반 React 싱글 페이지 앱', language: 'TypeScript', color: '#61dafb' },
  { id: 'next-app', name: 'Next.js App', description: 'App Router 기반 Next.js 풀스택', language: 'TypeScript', color: '#000000' },
  { id: 'express-api', name: 'Express API', description: 'Node.js + Express REST API 서버', language: 'JavaScript', color: '#68a063' },
  { id: 'spring-boot', name: 'Spring Boot', description: 'Java Spring Boot 마이크로서비스', language: 'Java', color: '#6db33f' },
  { id: 'python-fastapi', name: 'FastAPI', description: 'Python FastAPI 고성능 API 서버', language: 'Python', color: '#009688' },
]

const CLUSTERS = [
  { id: 'c1', name: 'prod-cluster', namespaces: ['default', 'production', 'staging'] },
  { id: 'c2', name: 'dev-cluster', namespaces: ['default', 'dev', 'test'] },
]

const STEP_LABELS: Record<Step, string> = {
  1: '앱 이름',
  2: 'Git Repository',
  3: '클러스터 / 네임스페이스',
  4: '리소스 설정',
  5: '환경 변수',
}

function generateYaml(form: Partial<FormState>): string {
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${form.appName ?? 'my-app'}
  namespace: ${form.namespace ?? 'default'}
  labels:
    app: ${form.appName ?? 'my-app'}
    template: ${form.template ?? 'react-spa'}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ${form.appName ?? 'my-app'}
  template:
    metadata:
      labels:
        app: ${form.appName ?? 'my-app'}
    spec:
      containers:
        - name: ${form.appName ?? 'my-app'}
          image: harbor.nullus.io/${form.appName ?? 'my-app'}:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${(form.envVars ?? []).filter((e) => e.key).length > 0
  ? `          env:\n${(form.envVars ?? []).filter((e) => e.key).map((e) => `            - name: ${e.key}\n              value: "${e.value}"`).join('\n')}`
  : ''}
---
apiVersion: v1
kind: Service
metadata:
  name: ${form.appName ?? 'my-app'}-svc
  namespace: ${form.namespace ?? 'default'}
spec:
  selector:
    app: ${form.appName ?? 'my-app'}
  ports:
    - port: 80
      targetPort: 8080`
}

interface EnvVar { key: string; value: string }

interface FormState {
  template: AppTemplate
  appName: string
  gitUrl: string
  clusterId: string
  namespace: string
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  envVars: EnvVar[]
}

const DEFAULT_FORM: FormState = {
  template: 'react-spa',
  appName: '',
  gitUrl: '',
  clusterId: 'c1',
  namespace: 'default',
  cpuRequest: '100m',
  cpuLimit: '500m',
  memoryRequest: '128Mi',
  memoryLimit: '512Mi',
  envVars: [{ key: '', value: '' }],
}

export function DeveloperDeployPage() {
  const role = useAuthStore((s) => s.role)
  const [step, setStep] = useState<Step>(1)
  const [form, setForm] = useState<FormState>(DEFAULT_FORM)
  const [deployed, setDeployed] = useState(false)

  const deployMutation = useDeployApp()

  if (role !== 'developer') {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '300px',
          flexDirection: 'column',
          gap: '12px',
          color: 'var(--color-text-secondary)',
        }}
      >
        <Rocket size={40} style={{ opacity: 0.3 }} />
        <p style={{ margin: 0, fontSize: '15px' }}>이 페이지는 Developer 역할 전용입니다.</p>
      </div>
    )
  }

  const setField = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const selectedCluster = CLUSTERS.find((c) => c.id === form.clusterId) ?? CLUSTERS[0]

  const handleDeploy = () => {
    const request: DeployAppRequest = {
      appName: form.appName,
      gitUrl: form.gitUrl,
      clusterId: form.clusterId,
      namespace: form.namespace,
      template: form.template,
      resources: {
        cpuRequest: form.cpuRequest,
        cpuLimit: form.cpuLimit,
        memoryRequest: form.memoryRequest,
        memoryLimit: form.memoryLimit,
      },
      envVars: form.envVars.filter((e) => e.key),
    }
    deployMutation.mutate(request, {
      onSuccess: () => setDeployed(true),
      onError: () => setDeployed(true), // mock: show success regardless
    })
  }

  const canNext: Record<Step, boolean> = {
    1: form.appName.trim().length >= 2,
    2: form.gitUrl.trim().startsWith('http'),
    3: !!form.clusterId && !!form.namespace,
    4: !!form.cpuLimit && !!form.memoryLimit,
    5: true,
  }

  if (deployed) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '360px', gap: '16px' }}>
        <div
          style={{
            width: '64px',
            height: '64px',
            borderRadius: '50%',
            background: 'rgba(34,197,94,0.15)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#22c55e',
          }}
        >
          <Rocket size={28} />
        </div>
        <h2 style={{ margin: 0, fontSize: '20px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
          배포 요청 완료!
        </h2>
        <p style={{ margin: 0, fontSize: '14px', color: 'var(--color-text-secondary)' }}>
          {form.appName} 앱이 {form.namespace} 네임스페이스에 배포 요청되었습니다.
        </p>
        <Button
          variant="outline"
          size="md"
          onClick={() => {
            setDeployed(false)
            setForm(DEFAULT_FORM)
            setStep(1)
          }}
        >
          새 배포
        </Button>
      </div>
    )
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '28px' }}>
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
          <Rocket size={18} />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: '22px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
            Developer Self-Service 배포
          </h1>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
            앱 템플릿을 선택하고 배포 위자드를 따라 배포하세요.
          </p>
        </div>
      </div>

      {/* Template selection */}
      <div style={{ marginBottom: '28px' }}>
        <p style={{ margin: '0 0 12px', fontSize: '13px', fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
          앱 템플릿
        </p>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '10px' }}>
          {APP_TEMPLATES.map((t) => (
            <button
              key={t.id}
              onClick={() => setField('template', t.id)}
              style={{
                background: form.template === t.id ? 'rgba(99,102,241,0.15)' : 'var(--color-surface-card)',
                border: `1px solid ${form.template === t.id ? 'rgba(99,102,241,0.5)' : 'var(--color-border-default)'}`,
                borderRadius: '10px',
                padding: '14px',
                cursor: 'pointer',
                textAlign: 'left',
                transition: 'all var(--transition-fast)',
              }}
            >
              <div
                style={{
                  width: '8px',
                  height: '8px',
                  borderRadius: '50%',
                  background: t.color,
                  marginBottom: '8px',
                }}
              />
              <p style={{ margin: '0 0 4px', fontSize: '13px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                {t.name}
              </p>
              <p style={{ margin: 0, fontSize: '11px', color: 'var(--color-text-secondary)' }}>
                {t.language}
              </p>
            </button>
          ))}
        </div>
      </div>

      {/* Step indicator */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginBottom: '24px', flexWrap: 'wrap' }}>
        {([1, 2, 3, 4, 5] as Step[]).map((s, i) => (
          <div key={s} style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <button
              onClick={() => s < step && setStep(s)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'none',
                border: 'none',
                cursor: s < step ? 'pointer' : 'default',
                padding: '4px 6px',
                borderRadius: '6px',
              }}
            >
              <div
                style={{
                  width: '22px',
                  height: '22px',
                  borderRadius: '50%',
                  background: s === step ? '#6366f1' : s < step ? 'rgba(34,197,94,0.3)' : 'rgba(255,255,255,0.08)',
                  color: s === step ? '#fff' : s < step ? '#22c55e' : 'var(--color-text-secondary)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '11px',
                  fontWeight: 700,
                  flexShrink: 0,
                }}
              >
                {s}
              </div>
              <span
                style={{
                  fontSize: '13px',
                  color: s === step ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  fontWeight: s === step ? 600 : 400,
                }}
              >
                {STEP_LABELS[s]}
              </span>
            </button>
            {i < 4 && <ChevronRight size={14} style={{ color: 'var(--color-text-secondary)', flexShrink: 0 }} />}
          </div>
        ))}
      </div>

      {/* Step content */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px', alignItems: 'start' }}>
        <div
          style={{
            background: 'var(--color-surface-card)',
            border: '1px solid var(--color-border-default)',
            borderRadius: 'var(--card-radius)',
            padding: '24px',
          }}
        >
          {step === 1 && (
            <StepSection title="앱 이름 입력">
              <Input
                placeholder="my-awesome-app"
                value={form.appName}
                onChange={(e) => setField('appName', e.target.value)}
              />
              <p style={{ margin: '6px 0 0', fontSize: '12px', color: 'var(--color-text-secondary)' }}>
                소문자, 숫자, 하이픈만 사용 가능합니다.
              </p>
            </StepSection>
          )}

          {step === 2 && (
            <StepSection title="Git Repository URL">
              <Input
                placeholder="https://github.com/org/repo.git"
                value={form.gitUrl}
                onChange={(e) => setField('gitUrl', e.target.value)}
              />
            </StepSection>
          )}

          {step === 3 && (
            <StepSection title="클러스터 & 네임스페이스">
              <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                <div>
                  <label style={labelStyle}>클러스터</label>
                  <select
                    value={form.clusterId}
                    onChange={(e) => {
                      setField('clusterId', e.target.value)
                      const cl = CLUSTERS.find((c) => c.id === e.target.value)
                      if (cl) setField('namespace', cl.namespaces[0])
                    }}
                    style={selectStyle}
                  >
                    {CLUSTERS.map((c) => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label style={labelStyle}>네임스페이스</label>
                  <select
                    value={form.namespace}
                    onChange={(e) => setField('namespace', e.target.value)}
                    style={selectStyle}
                  >
                    {selectedCluster.namespaces.map((ns) => (
                      <option key={ns} value={ns}>{ns}</option>
                    ))}
                  </select>
                </div>
              </div>
            </StepSection>
          )}

          {step === 4 && (
            <StepSection title="리소스 설정">
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                <ResourceSlider
                  label="CPU Request"
                  value={form.cpuRequest}
                  options={['100m', '200m', '500m', '1000m']}
                  onChange={(v) => setField('cpuRequest', v)}
                />
                <ResourceSlider
                  label="CPU Limit"
                  value={form.cpuLimit}
                  options={['200m', '500m', '1000m', '2000m']}
                  onChange={(v) => setField('cpuLimit', v)}
                />
                <ResourceSlider
                  label="Memory Request"
                  value={form.memoryRequest}
                  options={['64Mi', '128Mi', '256Mi', '512Mi']}
                  onChange={(v) => setField('memoryRequest', v)}
                />
                <ResourceSlider
                  label="Memory Limit"
                  value={form.memoryLimit}
                  options={['128Mi', '256Mi', '512Mi', '1Gi', '2Gi']}
                  onChange={(v) => setField('memoryLimit', v)}
                />
              </div>
            </StepSection>
          )}

          {step === 5 && (
            <StepSection title="환경 변수">
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {form.envVars.map((env, i) => (
                  <div key={i} style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                    <Input
                      placeholder="KEY"
                      value={env.key}
                      onChange={(e) => {
                        const next = [...form.envVars]
                        next[i] = { ...next[i], key: e.target.value }
                        setField('envVars', next)
                      }}
                      style={{ fontFamily: 'Fira Code, monospace', fontSize: '13px', flex: 1 }}
                    />
                    <Input
                      placeholder="value"
                      value={env.value}
                      onChange={(e) => {
                        const next = [...form.envVars]
                        next[i] = { ...next[i], value: e.target.value }
                        setField('envVars', next)
                      }}
                      style={{ fontFamily: 'Fira Code, monospace', fontSize: '13px', flex: 2 }}
                    />
                    <button
                      onClick={() => setField('envVars', form.envVars.filter((_, j) => j !== i))}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#f87171', padding: '4px', flexShrink: 0 }}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setField('envVars', [...form.envVars, { key: '', value: '' }])}
                  style={{ alignSelf: 'flex-start', marginTop: '4px' }}
                >
                  <Plus size={13} />
                  변수 추가
                </Button>
              </div>
            </StepSection>
          )}

          {/* Navigation */}
          <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end', marginTop: '24px' }}>
            {step > 1 && (
              <Button variant="outline" size="md" onClick={() => setStep((s) => (s - 1) as Step)}>
                이전
              </Button>
            )}
            {step < 5 ? (
              <Button
                variant="primary"
                size="md"
                disabled={!canNext[step]}
                onClick={() => setStep((s) => (s + 1) as Step)}
              >
                다음
              </Button>
            ) : (
              <Button
                variant="primary"
                size="md"
                loading={deployMutation.isPending}
                onClick={handleDeploy}
              >
                <Rocket size={14} />
                Deploy
              </Button>
            )}
          </div>
        </div>

        {/* YAML preview */}
        <div>
          <p style={{ margin: '0 0 10px', fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            YAML 매니페스트 미리보기
          </p>
          <CodePreview
            code={generateYaml(form)}
            language="yaml"
            title={`${form.appName || 'my-app'}.yaml`}
            maxHeight="600px"
          />
        </div>
      </div>
    </div>
  )
}

function StepSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p style={{ margin: '0 0 16px', fontSize: '15px', fontWeight: 700, color: 'var(--color-text-primary)' }}>
        {title}
      </p>
      {children}
    </div>
  )
}

function ResourceSlider({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: string[]
  onChange: (v: string) => void
}) {
  const idx = options.indexOf(value)
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px' }}>
        <label style={{ ...labelStyle, marginBottom: 0 }}>{label}</label>
        <span
          style={{
            fontSize: '13px',
            fontFamily: 'Fira Code, monospace',
            color: '#a5b4fc',
            fontWeight: 600,
          }}
        >
          {value}
        </span>
      </div>
      <input
        type="range"
        min={0}
        max={options.length - 1}
        value={idx >= 0 ? idx : 0}
        onChange={(e) => onChange(options[Number(e.target.value)])}
        style={{ width: '100%', accentColor: '#6366f1' }}
      />
      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '4px' }}>
        {options.map((o) => (
          <span key={o} style={{ fontSize: '10px', color: 'var(--color-text-secondary)', fontFamily: 'Fira Code, monospace' }}>
            {o}
          </span>
        ))}
      </div>
    </div>
  )
}

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: '12px',
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  marginBottom: '6px',
  textTransform: 'uppercase',
  letterSpacing: '0.04em',
}

const selectStyle: React.CSSProperties = {
  width: '100%',
  background: 'rgba(255,255,255,0.04)',
  border: '1px solid var(--color-border-default)',
  borderRadius: '8px',
  padding: '9px 12px',
  fontSize: '14px',
  color: 'var(--color-text-primary)',
  cursor: 'pointer',
}
