import { useState } from 'react'
import { z } from 'zod'
import { Check, Copy, Loader2 } from 'lucide-react'
import { Input } from '../../../components/ui/input'
import { cn } from '../../../lib/utils'
import type { CicdLogLevel } from '../hooks/use-cicd-deploy-log'

export type Step = 1 | 2 | 3 | 4 | 5 | 6

export interface EnvVar { key: string; value: string }

export type AppTemplate = 'react-spa' | 'next-app' | 'express-api' | 'spring-boot' | 'python-fastapi' | 'go-web-api'

export interface FormState {
  appName: string
  gitUrl: string
  dockerfilePath: string
  dockerContext: string
  template: AppTemplate
  clusterId: string
  namespace: string
  replicas: number
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  envVars: EnvVar[]
}

export const STEP_LABEL_DEFAULTS: Record<Step, string> = {
  1: 'App Name',
  2: 'Git Repository',
  3: 'Cluster / Namespace',
  4: 'Resource Configuration',
  5: 'Environment Variables',
  6: 'Manifest Review',
}

export const PROGRESS_SEGMENTS = Array.from({ length: 100 }, (_, i) => i + 1)

export const LOG_LEVEL_STYLE: Record<CicdLogLevel, string> = {
  info: 'bg-[rgba(59,130,246,0.15)] text-[#60a5fa]',
  success: 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]',
  error: 'bg-[rgba(239,68,68,0.15)] text-[#f87171]',
}

export const DEFAULT_FORM: FormState = {
  appName: '',
  gitUrl: '',
  dockerfilePath: '',
  dockerContext: '',
  template: 'react-spa',
  clusterId: '',
  namespace: 'default',
  replicas: 2,
  cpuRequest: '100m',
  cpuLimit: '500m',
  memoryRequest: '128Mi',
  memoryLimit: '512Mi',
  envVars: [],
}

export const deploySchema = z.object({
  appName: z.string().min(2, 'App name must be at least 2 characters').max(50, 'App name must be 50 characters or less'),
  gitUrl: z.string().min(1, 'Git URL is required'),
  dockerfilePath: z.string(),
  dockerContext: z.string(),
  template: z.enum(['react-spa', 'next-app', 'express-api', 'spring-boot', 'python-fastapi', 'go-web-api'] as const),
  clusterId: z.string().min(1, 'Cluster is required'),
  namespace: z.string().min(1, 'Namespace is required'),
  replicas: z.number().min(1).max(10),
  cpuRequest: z.string().min(1, 'CPU request is required'),
  cpuLimit: z.string().min(1, 'CPU limit is required'),
  memoryRequest: z.string().min(1, 'Memory request is required'),
  memoryLimit: z.string().min(1, 'Memory limit is required'),
  envVars: z
    .array(z.object({ key: z.string(), value: z.string() }))
    .superRefine((envVars, ctx) => {
      envVars.forEach((env, index) => {
        if (env.value.trim() && !env.key.trim()) {
          ctx.addIssue({ code: 'custom', message: 'Key is required when value exists', path: [index, 'key'] })
        }
      })
    }),
})

export const labelStyleClass = 'mb-1.5 block text-xs font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]'

function yamlSafe(value: string): string {
  if (/[:\n\r#"'\\{},&*?|><!%@`]/.test(value) || value.includes('[') || value.includes(']') || value !== value.trim()) {
    return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"`
  }
  return value
}

export function generateYaml(form: Partial<FormState>): string {
  const name = yamlSafe(form.appName ?? 'my-app')
  const ns = yamlSafe(form.namespace ?? 'default')
  const tpl = yamlSafe(form.template ?? 'react-spa')
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  const envLines = (form.envVars ?? [])
    .filter((e) => e.key)
    .map((e) => `            - name: ${yamlSafe(e.key)}\n              value: ${yamlSafe(e.value)}`)
    .join('\n')

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    app: ${name}
    template: ${tpl}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
        - name: ${name}
          image: harbor.nullus.io/${name}:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${envLines ? `          env:\n${envLines}` : ''}
---
apiVersion: v1
kind: Service
metadata:
  name: ${name}-svc
  namespace: ${ns}
spec:
  selector:
    app: ${name}
  ports:
    - port: 80
      targetPort: 8080`
}

export function PhaseStep({ label, index, progress, total }: { label: string; index: number; progress: number; total: number }) {
  const phaseProgress = 100 / total
  const phaseStart = index * phaseProgress
  const isDone = progress >= phaseStart + phaseProgress
  const isActive = progress >= phaseStart && !isDone

  return (
    <div className="flex flex-1 items-center gap-2">
      <div
        className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-all duration-300',
          isDone
            ? 'bg-[rgba(34,197,94,0.15)] text-[#22c55e]'
            : isActive
              ? 'bg-[rgba(99,102,241,0.15)] text-[#818cf8]'
              : 'bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]'
        )}
      >
        {isDone ? (
          <Check size={14} />
        ) : isActive ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          <span className="text-xs font-bold">{index + 1}</span>
        )}
      </div>
      <span
        className={cn(
          'text-[13px] font-semibold',
          isDone ? 'text-[#22c55e]' : isActive ? 'text-[#a5b4fc]' : 'text-[var(--color-text-secondary)]'
        )}
      >
        {label}
      </span>
      {index < total - 1 && (
        <div
          className={cn(
            'mx-1 h-px flex-1 transition-colors duration-300',
            isDone ? 'bg-[rgba(34,197,94,0.4)]' : 'bg-[var(--color-border-default)]'
          )}
        />
      )}
    </div>
  )
}

export function StepSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-4 mt-0 text-[15px] font-bold text-[var(--color-text-primary)]">
        {title}
      </p>
      {children}
    </div>
  )
}

export function ResourceSlider({
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
  const isCustom = idx === -1
  const sliderId = `resource-${label.toLowerCase().replace(/\s+/g, '-')}`
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <label htmlFor={sliderId} className={cn(labelStyleClass, 'mb-0')}>{label}</label>
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-24 text-right font-mono text-[13px]"
        />
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={options.length - 1}
        value={isCustom ? 0 : idx}
        onChange={(e) => onChange(options[Number(e.target.value)])}
        className="w-full accent-[#6366f1]"
      />
      <div className="mt-1 flex justify-between">
        {options.map((o) => (
          <span key={o} className="font-mono text-[10px] text-[var(--color-text-secondary)]">
            {o}
          </span>
        ))}
      </div>
    </div>
  )
}

export function CopyableCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="mt-3 flex items-center gap-2 rounded-md bg-[#0d1117] px-3 py-2">
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-[#c9d1d9]">
        <span className="mr-1.5 text-[#484f58]">$</span>{command}
      </code>
      <button
        type="button"
        onClick={() => { void navigator.clipboard.writeText(command); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
        className="shrink-0 cursor-pointer border-none bg-none p-1 text-[rgba(255,255,255,0.4)] transition-colors hover:text-white"
      >
        {copied ? <Check size={14} className="text-[#3fb950]" /> : <Copy size={14} />}
      </button>
    </div>
  )
}
