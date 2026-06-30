import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { StackInfoTab } from './stack-info-tab'

const mockUseStackHistory = vi.hoisted(() => vi.fn())
const mockUseStackMonitoring = vi.hoisted(() => vi.fn())
const mockUseExportStackConfig = vi.hoisted(() => vi.fn())
const mockDownloadBlob = vi.hoisted(() => vi.fn())

vi.mock('../api/stack-api', () => ({
  useStackHistory: (...args: unknown[]) => mockUseStackHistory(...args),
  useStackMonitoring: (...args: unknown[]) => mockUseStackMonitoring(...args),
  useExportStackConfig: (...args: unknown[]) => mockUseExportStackConfig(...args),
}))

vi.mock('./stack-info-panels', () => ({
  ArtifactsPanel: () => <div data-testid="artifacts-panel" />,
  PipelineToolsPanel: () => <div data-testid="pipeline-panel" />,
  MonitoringToolsPanel: () => <div data-testid="monitoring-panel" />,
  LoggingToolsPanel: () => <div data-testid="logging-panel" />,
  ResourcesPanel: () => <div data-testid="resources-panel" />,
}))

vi.mock('./retry-stack-button', () => ({
  RetryStackButton: () => <div data-testid="retry-stack-button" />,
}))

vi.mock('../../../lib/utils', () => ({
  cn: (...classes: Array<string | false | null | undefined>) => classes.filter(Boolean).join(' '),
}))

vi.mock('../utils/export-utils', async () => {
  const actual = await vi.importActual<typeof import('../utils/export-utils')>(
    '../utils/export-utils',
  )
  return {
    ...actual,
    downloadBlob: mockDownloadBlob,
  }
})

vi.mock('../utils/stack-list-utils', () => ({
  buildPipelineNodesFromSnapshot: () => [],
  buildPipelineNodesFromMonitoring: () => [],
  buildInstalledToolsFromSnapshot: () => [],
  extractAccessDomain: () => '',
  toolLaunchURL: () => '',
  buildHostsText: () => '',
  extractConnectionInfo: () => ({
    accessDomain: '',
    database: {
      mode: 'none',
      providerOrEngine: '-',
      endpoint: '-',
      resourceName: '-',
      authId: '-',
      accessSecretRef: '-',
      authPasswordKey: '-',
    },
    objectStorage: {
      mode: 'none',
      providerOrEngine: '-',
      endpoint: '-',
      resourceName: '-',
      authId: '-',
      accessSecretRef: '-',
      authPasswordKey: '-',
    },
  }),
  buildConnectionInfoText: () => 'connection-info',
  buildOssLoginHint: () => 'hint',
  deriveGatewayName: () => 'gateway',
  toShellSingleQuoted: (value: string) => value,
  copyTextToClipboard: async () => undefined,
  getStackStatusLabel: (_t: unknown, status: string) => status,
}))

describe('StackInfoTab export flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseStackHistory.mockReturnValue({ data: [], isLoading: false })
    mockUseStackMonitoring.mockReturnValue({ data: null, isLoading: false })
    mockUseExportStackConfig.mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue({
        blob: new Blob(['demo'], { type: 'application/json' }),
        filename: 'server-name.json',
        contentType: 'application/json',
      }),
      isPending: false,
    })
  })

  it('opens the export modal and downloads the chosen filename', async () => {
    const stack = {
      id: 'stack-1',
      name: 'DevSecOps Core',
      namespace: 'nullus',
      status: 'success',
      templateName: 'GitLab + Argo CD',
      clusterName: 'prod-cluster',
    } as never

    render(
      <StackInfoTab
        stack={stack}
        displayStatus="success"
        isDeleting={false}
        onAddTools={vi.fn()}
        onDelete={vi.fn()}
        onBackToList={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: /stackList\.export\.open|Export/ }))
    fireEvent.change(screen.getByLabelText(/stackList\.export\.fileName|File name/), {
      target: { value: 'prod-backup' },
    })
    fireEvent.change(screen.getByLabelText(/stackList\.export\.format|Format/), {
      target: { value: 'yaml' },
    })
    fireEvent.click(screen.getByRole('button', { name: /stackList\.export\.confirm|Download/ }))

    await waitFor(() => {
      expect(mockDownloadBlob).toHaveBeenCalledWith(expect.any(Blob), 'prod-backup.yaml')
    })
  })
})
