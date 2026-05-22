import { useCallback, useEffect, useRef, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from 'react-router-dom'
import { AlignLeft, Check, Copy, Download, Rocket, Save } from 'lucide-react'
import Editor from '@monaco-editor/react'
import type { Monaco } from '@monaco-editor/react'
import { configureMonacoYaml } from 'monaco-yaml'
import YAML from 'yaml'
import { Breadcrumb } from '../../../components/shared/breadcrumb'
import { useStackConfigStore } from '../stores/stack-config-store'
import type { InstallTab } from '../stores/stack-config-store'
import { useCreateStack, useSaveDraft, useEstimateResources, useClusters, toCreateStackBody } from '../api/stack-api'
import { api } from '../../../lib/api'
import { useAppToast } from '../../../hooks/use-toast'
import { useClusterNamespaces } from '../../admin/api/admin-api'
import { Button } from '../../../components/ui/button'
import { NativeSelect } from '../../../components/ui/native-select'
import { Input } from '../../../components/ui/input'
import { Modal } from '../../../components/ui/modal'
import { CodePreview } from '../../../components/shared/code-preview'
import { cn } from '../../../lib/utils'
import { useThemeStore } from '../../../stores/theme-store'
import {
  ARTIFACTS_OPTIONS, PIPELINE_OPTIONS, MONITORING_OPTIONS, LOGGING_OPTIONS,
  TABS, stackInstallSchema, createDeployScript, createK8sObjects,
  draftToYaml, parseDraftFromYaml, ToolSelector,
} from './stack-install-data'
import type { K8sPreviewTab, StackInstallFormData } from './stack-install-data'

export function StackInstallPage() {
  const navigate = useNavigate()
  const theme = useThemeStore((state) => state.theme)
  const isDarkMode = theme === 'dark'
  const { draft, setActiveTab, setTool, setStackName, setCluster, setNamespace, updateResources } = useStackConfigStore()
  const createStack = useCreateStack()
  const saveDraft = useSaveDraft()
  const toast = useAppToast()
  const estimateResources = useEstimateResources()
  const { data: clusters } = useClusters()
  const { data: namespaces } = useClusterNamespaces(draft.clusterId ?? '')
  const [createNewNs, setCreateNewNs] = useState(false)
  const [activeTab, setLocalTab] = useState<InstallTab>(draft.activeTab)
  const [deployScriptModalOpen, setDeployScriptModalOpen] = useState(false)
  const [k8sPreviewModalOpen, setK8sPreviewModalOpen] = useState(false)
  const [activeK8sPreviewTab, setActiveK8sPreviewTab] = useState<K8sPreviewTab>('namespace')
  const [yamlContent, setYamlContent] = useState(() => draftToYaml(draft))
  const [yamlCopied, setYamlCopied] = useState(false)
  const yamlContentRef = useRef(yamlContent)
  const syncFromYamlTimerRef = useRef<number | null>(null)
  const syncFromDraftTimerRef = useRef<number | null>(null)
  const applyingYamlToStoreRef = useRef(false)
  const monacoConfiguredRef = useRef(false)
  const {
    control,
    trigger,
    setValue,
    formState: { errors, isValid, isSubmitting },
  } = useForm<StackInstallFormData>({
    resolver: zodResolver(stackInstallSchema),
    defaultValues: {
      stackName: draft.stackName,
      developerCount: draft.resources.developerCount,
      concurrentRunners: draft.resources.concurrentRunners,
    },
    mode: 'onChange',
  })

  const deployScript = createDeployScript(draft)
  const k8sObjects = createK8sObjects(draft)

  const handleYamlChange = useCallback((value?: string) => {
    const nextYaml = value ?? ''
    yamlContentRef.current = nextYaml
    setYamlContent(nextYaml)

    if (syncFromDraftTimerRef.current !== null) {
      window.clearTimeout(syncFromDraftTimerRef.current)
      syncFromDraftTimerRef.current = null
    }

    if (syncFromYamlTimerRef.current !== null) {
      window.clearTimeout(syncFromYamlTimerRef.current)
    }

    syncFromYamlTimerRef.current = window.setTimeout(() => {
      try {
        const currentDraft = useStackConfigStore.getState().draft
        const parsedDraft = parseDraftFromYaml(nextYaml, currentDraft)
        if (!parsedDraft) return

        applyingYamlToStoreRef.current = true
        useStackConfigStore.setState((state) => ({
          draft: {
            ...parsedDraft,
            activeTab: state.draft.activeTab,
          },
          isDirty: true,
        }))
      } catch {
        // Ignore invalid YAML while the user is still typing.
      }
    }, 300)
  }, [])

  const handleCopyYaml = useCallback(() => {
    void navigator.clipboard.writeText(yamlContentRef.current).then(() => {
      setYamlCopied(true)
      window.setTimeout(() => setYamlCopied(false), 1500)
    })
  }, [])

  const handleFormatYaml = useCallback(() => {
    try {
      const parsed = YAML.parse(yamlContentRef.current)
      const formatted = YAML.stringify(parsed, { indent: 2, lineWidth: 0 })
      handleYamlChange(formatted)
    } catch {
      // Keep the existing YAML when formatting invalid input.
    }
  }, [handleYamlChange])

  const handleMonacoBeforeMount = useCallback((monaco: Monaco) => {
    if (monacoConfiguredRef.current) return
    configureMonacoYaml(monaco, {
      validate: true,
      completion: false,
      hover: true,
      format: true,
      enableSchemaRequest: false,
      schemas: [],
    })
    monacoConfiguredRef.current = true
  }, [])

  useEffect(() => {
    yamlContentRef.current = yamlContent
  }, [yamlContent])

  useEffect(() => {
    if (applyingYamlToStoreRef.current) {
      applyingYamlToStoreRef.current = false
      return
    }

    const nextYaml = draftToYaml(draft)
    if (nextYaml === yamlContentRef.current) return

    if (syncFromDraftTimerRef.current !== null) {
      window.clearTimeout(syncFromDraftTimerRef.current)
    }

    syncFromDraftTimerRef.current = window.setTimeout(() => {
      yamlContentRef.current = nextYaml
      setYamlContent(nextYaml)
    }, 300)
  }, [draft])

  useEffect(() => {
    setValue('stackName', draft.stackName)
    setValue('developerCount', draft.resources.developerCount)
    setValue('concurrentRunners', draft.resources.concurrentRunners)
  }, [draft.resources.concurrentRunners, draft.resources.developerCount, draft.stackName, setValue])

  useEffect(() => {
    return () => {
      if (syncFromYamlTimerRef.current !== null) {
        window.clearTimeout(syncFromYamlTimerRef.current)
      }
      if (syncFromDraftTimerRef.current !== null) {
        window.clearTimeout(syncFromDraftTimerRef.current)
      }
    }
  }, [])

  const switchTab = (tab: InstallTab) => {
    setLocalTab(tab)
    setActiveTab(tab)
  }

  const validateCoreFields = async () => {
    return trigger(['stackName', 'developerCount', 'concurrentRunners'])
  }

  const handleDeploy = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return

    const body = toCreateStackBody({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      namespace: draft.namespace,
      stackName: draft.stackName,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
    })

    try {
      const createRes = await api.post<{ id: string }>('/stacks', body)
      const stackId = createRes.data?.id
      if (!stackId) {
        toast.error('스택 생성 응답에 id가 없습니다')
        return
      }
      await api.post(`/stacks/${stackId}/deploy`).catch((err) => {
        const msg = (err && typeof err === 'object' && 'message' in err) ? String((err as { message: unknown }).message) : '배포 시작 실패'
        toast.error(`배포 시작 실패: ${msg}`)
      })
      navigate(`/stack/deploy/${stackId}`)
    } catch (err) {
      const e = err as { status?: number; message?: string; details?: unknown }
      const detail = (e?.details && typeof e.details === 'object' && 'error' in e.details && typeof (e.details as { error: { message?: string } }).error?.message === 'string')
        ? (e.details as { error: { message: string } }).error.message
        : e?.message ?? '알 수 없는 오류'
      console.error('stack create failed', { status: e?.status, body, error: err })
      toast.error(`스택 생성 실패 (${e?.status ?? '?'}): ${detail}`)
    }
  }

  const handleSaveDraft = async () => {
    const isFormValid = await validateCoreFields()
    if (!isFormValid) return

    saveDraft.mutate({
      templateId: draft.selectedTemplateId,
      clusterId: draft.clusterId,
      stackName: draft.stackName,
      artifacts: draft.artifacts as unknown as Record<string, { tool: string; version: string }>,
      pipeline: draft.pipeline as unknown as Record<string, { tool: string; version: string }>,
      monitoring: draft.monitoring as unknown as Record<string, { tool: string; version: string }>,
      logging: draft.logging as unknown as Record<string, { tool: string; version: string }>,
      resources: draft.resources,
    })
  }

  const handleEstimate = () => {
    estimateResources.mutate(draft.resources)
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Stack List', path: '/stack/list' },
        { label: 'New Stack', path: '/stack/templates' },
        { label: 'Stack Template', path: '/stack/templates' },
        { label: 'Stack Install' },
      ]} />

      {/* Page header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
          >
            <Download size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Stack Install
            </h1>
            <p className="mt-0.5 m-0 text-[13px] text-[var(--color-text-secondary)]">
              5단계 워크플로우로 DevSecOps 스택을 구성하세요.
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="md"
            loading={saveDraft.isPending}
            onClick={handleSaveDraft}
            disabled={!isValid || isSubmitting}
            type="button"
          >
            <Save size={14} />
            Save Draft
          </Button>
          <Button variant="ghost" size="md" onClick={() => setDeployScriptModalOpen(true)} type="button">
            Preview Deploy Script
          </Button>
          <Button variant="ghost" size="md" onClick={() => setK8sPreviewModalOpen(true)} type="button">
            Preview K8s Objects
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={createStack.isPending}
            onClick={handleDeploy}
            disabled={isSubmitting || !draft.stackName || draft.stackName.length < 2 || !draft.clusterId}
            type="button"
          >
            <Rocket size={14} />
            Deploy
          </Button>
        </div>
      </div>

      <div className="mb-5 flex items-start gap-4">
        <div className="max-w-[400px] flex-1">
          <Controller
            control={control}
            name="stackName"
            render={({ field }) => (
              <>
                <Input
                  label="Stack Name"
                  placeholder="예: prod-gitlab-stack"
                  value={field.value}
                  onChange={(e) => {
                    field.onChange(e.target.value)
                    setStackName(e.target.value)
                  }}
                  onBlur={field.onBlur}
                />
                {errors.stackName && <span className="text-xs text-[#ef4444]">{errors.stackName.message}</span>}
              </>
            )}
          />
        </div>
        <div className="flex max-w-[300px] flex-1 flex-col gap-1">
          <NativeSelect
            label="Target Cluster"
            value={draft.clusterId ?? ''}
            onChange={(e) => setCluster(e.target.value)}
            className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
          >
            <option value="">클러스터를 선택하세요</option>
            {(clusters ?? []).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.connection_status})
              </option>
            ))}
          </NativeSelect>
          {!draft.clusterId && <span className="text-xs text-[#f59e0b]">배포에 필요합니다</span>}
        </div>
        {draft.clusterId && (
          <div className="flex max-w-[300px] flex-1 flex-col gap-1">
            <NativeSelect
              label="Namespace"
              value={createNewNs ? '__new__' : draft.namespace}
              onChange={(e) => {
                if (e.target.value === '__new__') {
                  setCreateNewNs(true)
                  setNamespace('')
                } else {
                  setCreateNewNs(false)
                  setNamespace(e.target.value)
                }
              }}
              className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
            >
              <option value="">기본 (nullus)</option>
              {(namespaces ?? []).map((ns) => (
                <option key={ns.name} value={ns.name}>{ns.name}</option>
              ))}
              <option value="__new__">새 네임스페이스 생성...</option>
            </NativeSelect>
            {createNewNs && (
              <input
                type="text"
                placeholder="my-namespace"
                value={draft.namespace}
                onChange={(e) => setNamespace(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
              />
            )}
            <span className="text-[11px] text-[var(--color-text-secondary)]">배포 대상 네임스페이스</span>
          </div>
        )}
      </div>

      <div className="flex items-start gap-5">
        {/* Left: tabs + content */}
        <div className="min-w-0 flex-1">
          {/* Tabs */}
          <div className="mb-5 flex gap-0 border-b border-[var(--color-border-default)]">
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => switchTab(tab.id)}
                  className={cn(
                    '-mb-px cursor-pointer border-b-2 border-b-transparent bg-none px-[18px] py-2.5 text-sm transition-all duration-150',
                    isActive
                      ? 'border-b-[#6366f1] font-semibold text-[#a5b4fc]'
                      : 'font-normal text-[var(--color-text-secondary)]'
                  )}
                >
                  {tab.label}
                </button>
              )
            })}
          </div>

          {/* Tab content */}
          <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-5">
            {activeTab === 'artifacts' && (
              <>
                <ToolSelector
                  label="Package Registry"
                  options={ARTIFACTS_OPTIONS.packageRegistry}
                  value={draft.artifacts.packageRegistry}
                  onChange={(v) => setTool('artifacts', 'packageRegistry', v)}
                />
                <ToolSelector
                  label="Source Repository"
                  options={ARTIFACTS_OPTIONS.sourceRepository}
                  value={draft.artifacts.sourceRepository}
                  onChange={(v) => setTool('artifacts', 'sourceRepository', v)}
                />
                <ToolSelector
                  label="Container Registry"
                  options={ARTIFACTS_OPTIONS.containerRegistry}
                  value={draft.artifacts.containerRegistry}
                  onChange={(v) => setTool('artifacts', 'containerRegistry', v)}
                />
                <ToolSelector
                  label="Storage Backend"
                  options={ARTIFACTS_OPTIONS.storageBackend}
                  value={draft.artifacts.storageBackend}
                  onChange={(v) => setTool('artifacts', 'storageBackend', v)}
                />
              </>
            )}

            {activeTab === 'pipeline' && (
              <>
                <ToolSelector
                  label="CI/CD Platform"
                  options={PIPELINE_OPTIONS.cicdPlatform}
                  value={draft.pipeline.cicdPlatform}
                  onChange={(v) => setTool('pipeline', 'cicdPlatform', v)}
                />
                <ToolSelector
                  label="CD Tool"
                  options={PIPELINE_OPTIONS.cdTool}
                  value={draft.pipeline.cdTool}
                  onChange={(v) => setTool('pipeline', 'cdTool', v)}
                />
              </>
            )}

            {activeTab === 'monitoring' && (
              <>
                <ToolSelector
                  label="Visualization"
                  options={MONITORING_OPTIONS.visualization}
                  value={draft.monitoring.visualization}
                  onChange={(v) => setTool('monitoring', 'visualization', v)}
                />
                <ToolSelector
                  label="Metrics"
                  options={MONITORING_OPTIONS.collection}
                  value={draft.monitoring.collection}
                  onChange={(v) => setTool('monitoring', 'collection', v)}
                />
                <ToolSelector
                  label="Logs"
                  options={LOGGING_OPTIONS.search}
                  value={draft.logging.search}
                  onChange={(v) => setTool('logging', 'search', v)}
                />
                <ToolSelector
                  label="Traces"
                  options={MONITORING_OPTIONS.traceLayer}
                  value={draft.logging.traceLayer}
                  onChange={(v) => setTool('logging', 'traceLayer', v)}
                />
              </>
            )}

            {activeTab === 'yaml' && (
              <div>
                <p className="mb-[14px] mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  폼과 YAML이 300ms 단위로 동기화됩니다. YAML 문법이 유효할 때만 설정에 반영됩니다.
                </p>
                <div className="mb-2 flex items-center justify-end gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={handleFormatYaml}>
                    <AlignLeft size={12} />
                    Format
                  </Button>
                  <Button variant="outline" size="sm" type="button" onClick={handleCopyYaml}>
                    {yamlCopied ? <Check size={12} /> : <Copy size={12} />}
                    {yamlCopied ? 'Copied!' : 'Copy'}
                  </Button>
                </div>
                <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)]">
                  <Editor
                    beforeMount={handleMonacoBeforeMount}
                    height="500px"
                    language="yaml"
                    theme={isDarkMode ? 'vs-dark' : 'vs-light'}
                    value={yamlContent}
                    onChange={handleYamlChange}
                    options={{
                      minimap: { enabled: false },
                      fontSize: 13,
                      lineNumbers: 'on',
                      scrollBeyondLastLine: false,
                      wordWrap: 'on',
                      tabSize: 2,
                    }}
                  />
                </div>
              </div>
            )}

            {activeTab === 'resources' && (
              <div>
                <p className="mb-4 mt-0 text-[13px] text-[var(--color-text-secondary)]">
                  팀 규모와 사용 패턴을 입력하면 필요한 리소스를 계산합니다.
                </p>
                <div className="mb-4 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <label htmlFor="resource-mode-auto" className="text-xs font-medium text-[var(--color-text-secondary)]">
                      리소스 모드
                    </label>
                    <div className="flex gap-2 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-1">
                      {(['auto', 'manual'] as const).map((mode) => (
                        <button
                          key={mode}
                          id={mode === 'auto' ? 'resource-mode-auto' : 'resource-mode-manual'}
                          type="button"
                          onClick={() => updateResources({ mode })}
                          className={cn(
                            'px-3 py-1.5 text-xs font-medium rounded transition-all duration-150',
                            draft.resources.mode === mode
                              ? 'bg-[#6366f1] text-white'
                              : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
                          )}
                        >
                          {mode === 'auto' ? 'Auto' : 'Manual'}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="flex flex-col gap-1">
                    <label htmlFor="currency-select" className="text-xs font-medium text-[var(--color-text-secondary)]">
                      통화
                    </label>
                    <NativeSelect
                      id="currency-select"
                      value={draft.resources.currency}
                      onChange={(e) =>
                        updateResources({ currency: e.target.value as 'USD' | 'KRW' | 'CNY' })
                      }
                      className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                    >
                      <option value="USD">USD ($)</option>
                      <option value="KRW">KRW (₩)</option>
                      <option value="CNY">CNY (¥)</option>
                    </NativeSelect>
                  </div>
                </div>

                {draft.resources.mode === 'auto' && (
                  <div className="mb-4 grid grid-cols-2 gap-[14px]">
                    <Controller
                      control={control}
                      name="developerCount"
                      render={({ field }) => (
                        <>
                          <Input
                            label="개발자 수"
                            type="number"
                            min={1}
                            value={field.value}
                            onChange={(e) => {
                              const value = Number(e.target.value)
                              field.onChange(value)
                              updateResources({ developerCount: value })
                            }}
                            onBlur={field.onBlur}
                          />
                          {errors.developerCount && <span className="text-xs text-[#ef4444]">{errors.developerCount.message}</span>}
                        </>
                      )}
                    />
                    <Controller
                      control={control}
                      name="concurrentRunners"
                      render={({ field }) => (
                        <>
                          <Input
                            label="동시 러너 수"
                            type="number"
                            min={1}
                            value={field.value}
                            onChange={(e) => {
                              const value = Number(e.target.value)
                              field.onChange(value)
                              updateResources({ concurrentRunners: value })
                            }}
                            onBlur={field.onBlur}
                          />
                          {errors.concurrentRunners && <span className="text-xs text-[#ef4444]">{errors.concurrentRunners.message}</span>}
                        </>
                      )}
                    />
                    <Input
                      label="일일 커밋 수"
                      type="number"
                      min={1}
                      value={draft.resources.commitsPerDay}
                      onChange={(e) => updateResources({ commitsPerDay: Number(e.target.value) })}
                    />
                    <div className="flex flex-col gap-1">
                      <label htmlFor="build-frequency" className="text-xs font-medium text-[var(--color-text-secondary)]">
                        빌드 빈도
                      </label>
                      <select
                        id="build-frequency"
                        value={draft.resources.buildFrequency}
                        onChange={(e) =>
                          updateResources({ buildFrequency: e.target.value as 'low' | 'medium' | 'high' })
                        }
                        className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
                      >
                        <option value="low">낮음 (Low)</option>
                        <option value="medium">보통 (Medium)</option>
                        <option value="high">높음 (High)</option>
                      </select>
                    </div>
                  </div>
                )}

                {draft.resources.mode === 'manual' && (
                  <div className="mb-4 grid grid-cols-2 gap-[14px]">
                    <Input
                      label="CPU 요청"
                      placeholder="예: 4"
                      value={draft.resources.cpuRequest || ''}
                      onChange={(e) => updateResources({ cpuRequest: e.target.value })}
                    />
                    <Input
                      label="메모리 요청"
                      placeholder="예: 8Gi"
                      value={draft.resources.memoryRequest || ''}
                      onChange={(e) => updateResources({ memoryRequest: e.target.value })}
                    />
                    <Input
                      label="스토리지 요청"
                      placeholder="예: 100Gi"
                      value={draft.resources.storageRequest || ''}
                      onChange={(e) => updateResources({ storageRequest: e.target.value })}
                    />
                  </div>
                )}
                <Button
                  variant="secondary"
                  size="sm"
                  loading={estimateResources.isPending}
                  onClick={handleEstimate}
                  type="button"
                  className="mb-4"
                >
                  리소스 계산
                </Button>
                {estimateResources.data && (
                  <div className="grid grid-cols-4 gap-3 rounded-lg border border-[rgba(99,102,241,0.3)] bg-[rgba(99,102,241,0.08)] p-[14px]">
                    {[
                      ['CPU', estimateResources.data.cpu],
                      ['Memory', estimateResources.data.memory],
                      ['Storage', estimateResources.data.storage],
                      ['월 비용', `${estimateResources.data.estimatedCostMonthly.toLocaleString()} ${estimateResources.data.currency}`],
                    ].map(([label, val]) => (
                      <div key={label}>
                        <div className="mb-1 text-[11px] text-[var(--color-text-secondary)]">{label}</div>
                        <div className="text-[15px] font-bold text-[#a5b4fc]">{val}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Right: Configuration Summary */}
        <div className="sticky top-6 w-[260px] shrink-0 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-4">
          <h3 className="mb-[14px] mt-0 text-[13px] font-bold uppercase tracking-[0.06em] text-[var(--color-text-primary)]">
            Configuration Summary
          </h3>
          {[
            ['Template', draft.selectedTemplateId ?? '—'],
            ['Stack Name', draft.stackName || '—'],
            ['Package Registry', draft.artifacts.packageRegistry.tool],
            ['Source Repo', draft.artifacts.sourceRepository.tool],
            ['Container Registry', draft.artifacts.containerRegistry.tool],
            ['Storage', draft.artifacts.storageBackend.tool],
            ['CI/CD', draft.pipeline.cicdPlatform.tool],
            ['CD Tool', draft.pipeline.cdTool.tool],
            ['Visualization', draft.monitoring.visualization.tool],
            ['Metrics', draft.monitoring.collection.tool],
            ['Logs', draft.logging.search.tool],
            ['Traces', draft.logging.traceLayer.tool],
          ].map(([label, val]) => (
            <div
              key={label}
              className="flex items-baseline justify-between gap-2 border-b border-[rgba(255,255,255,0.04)] py-1.5"
            >
              <span className="shrink-0 text-[11px] text-[var(--color-text-secondary)]">{label}</span>
              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-right text-xs font-semibold text-[var(--color-text-primary)]">
                {val}
              </span>
            </div>
          ))}
        </div>
      </div>

      <Modal
        open={deployScriptModalOpen}
        onClose={() => setDeployScriptModalOpen(false)}
        title="Deploy Script Preview"
        wide
        footer={
          <Button variant="outline" size="sm" onClick={() => setDeployScriptModalOpen(false)} type="button">
            Close
          </Button>
        }
      >
        <CodePreview
          code={deployScript}
          language="bash"
          title={`${draft.stackName || 'nullus-stack'}-deploy.sh`}
          maxHeight="520px"
        />
      </Modal>

      <Modal
        open={k8sPreviewModalOpen}
        onClose={() => setK8sPreviewModalOpen(false)}
        title="K8s Object Preview"
        wide
        footer={
          <Button variant="outline" size="sm" onClick={() => setK8sPreviewModalOpen(false)} type="button">
            Close
          </Button>
        }
      >
        <div className="mb-[14px] flex flex-wrap gap-2">
          {[
            { id: 'namespace', label: 'Namespace' },
            { id: 'deployment', label: 'Deployment' },
            { id: 'service', label: 'Service' },
            { id: 'ingress', label: 'Ingress' },
          ].map((tab) => {
            const isActive = activeK8sPreviewTab === tab.id
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveK8sPreviewTab(tab.id as K8sPreviewTab)}
                className={cn(
                  'cursor-pointer rounded-lg border px-3 py-[7px] text-[13px]',
                  isActive
                    ? 'border-[#ca8a04] bg-[rgba(202,138,4,0.18)] font-bold text-[#fcd34d]'
                    : 'border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] font-medium text-[var(--color-text-secondary)]'
                )}
              >
                {tab.label}
              </button>
            )
          })}
        </div>
        <CodePreview
          code={k8sObjects[activeK8sPreviewTab]}
          language="yaml"
          title={`${activeK8sPreviewTab}.yaml`}
          maxHeight="500px"
        />
      </Modal>
    </div>
  )
}
