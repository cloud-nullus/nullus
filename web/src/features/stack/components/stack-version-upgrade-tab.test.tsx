import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import type { UpgradeCandidate, UpgradeCheck, UpgradeRun } from '../api/stack-api-types'
import { StackVersionUpgradeTab } from './stack-version-upgrade-tab'

const mockUseStackUpgrades = vi.fn()
const mockPreflight = vi.fn()
const mockStart = vi.fn()

vi.mock('../api/stack-api', async () => {
  const actual = await vi.importActual<typeof import('../api/stack-api')>('../api/stack-api')
  return {
    isUpgradeRunActive: actual.isUpgradeRunActive,
    createIdempotencyKey: actual.createIdempotencyKey,
    useStackUpgrades: (...args: unknown[]) => mockUseStackUpgrades(...args),
    usePreflightUpgrade: () => ({ mutateAsync: mockPreflight, isPending: false }),
    useStartUpgrade: () => ({ mutateAsync: mockStart, isPending: false }),
  }
})

const bundle = {
  id: 'b-20',
  tool: 'argocd',
  source_chart_version: '7.7.16',
  source_app_version: 'v2.13.3',
  target_chart_version: '7.7.20',
  target_app_version: 'v2.13.5',
  risk_level: 'low' as const,
  expected_disruption: 'API server restarts for about a minute',
  release_notes: 'https://github.com/argoproj/argo-cd/releases/tag/v2.13.5',
  requires_backup: true,
}

function candidate(overrides: Partial<UpgradeCandidate> = {}): UpgradeCandidate {
  return {
    tool: 'argocd',
    release_name: 'argo-cd',
    current_chart_version: '7.7.16',
    current_app_version: 'v2.13.3',
    current_revision: 3,
    release_status: 'deployed',
    bundle,
    checked_at: '2026-10-18T09:00:00Z',
    ...overrides,
  }
}

function run(overrides: Partial<UpgradeRun> = {}): UpgradeRun {
  return {
    id: 'run-1',
    stack_id: 'stk_1',
    bundle_id: 'b-20',
    tool: 'argocd',
    status: 'upgrading',
    current_step: 'helm_upgrade',
    requested_by: 'devops',
    reason: 'CVE',
    source_chart_version: '7.7.16',
    target_chart_version: '7.7.20',
    previous_revision: 3,
    checks: [],
    started_at: '2026-10-18T09:00:00Z',
    updated_at: '2026-10-18T09:00:00Z',
    ...overrides,
  }
}

const passing: UpgradeCheck[] = [
  { name: 'stack_state', status: 'pass', message: 'Stack installed' },
  { name: 'backup', status: 'pass', message: 'Last successful backup: 2026-10-18T08:00:00Z' },
  { name: 'helm_dry_run', status: 'pass', message: 'Helm server-side dry-run passed' },
]

function withItems(items: UpgradeCandidate[]) {
  mockUseStackUpgrades.mockReturnValue({ data: { items }, isLoading: false, isError: false })
}

beforeEach(() => {
  mockUseStackUpgrades.mockReset()
  mockPreflight.mockReset()
  mockStart.mockReset()
})

describe('StackVersionUpgradeTab', () => {
  it('shows a loading message while inspecting releases', () => {
    mockUseStackUpgrades.mockReturnValue({ data: undefined, isLoading: true, isError: false })
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/Checking installed releases/i)).toBeInTheDocument()
  })

  it('shows an error when releases cannot be inspected', () => {
    mockUseStackUpgrades.mockReturnValue({ data: undefined, isLoading: false, isError: true })
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/Could not inspect installed releases/i)).toBeInTheDocument()
  })

  it('says when no upgradable release is installed', () => {
    withItems([])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/No upgradable release/i)).toBeInTheDocument()
  })

  // 검증 경로가 없는 것을 "최신" 이라고 말하면 운영자가 패치를 미룬다.
  it('does not claim up to date when there is no verified path', () => {
    withItems([candidate({ bundle: undefined })])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText('No verified upgrade path')).toBeInTheDocument()
    expect(screen.queryByText(/up to date/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upgrade' })).not.toBeInTheDocument()
    expect(screen.getByText('0 available')).toBeInTheDocument()
  })

  it('shows the verified target, risk and release notes', () => {
    withItems([candidate()])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/7\.7\.20/)).toBeInTheDocument()
    expect(screen.getByText('Low risk')).toBeInTheDocument()
    expect(screen.getByText('1 available')).toBeInTheDocument()
    const notes = screen.getByRole('link', { name: 'Release notes' })
    expect(notes).toHaveAttribute('href', bundle.release_notes)
    expect(notes).toHaveAttribute('rel', expect.stringContaining('noopener'))
    expect(screen.getByRole('button', { name: 'Upgrade' })).toBeEnabled()
  })

  it('disables Upgrade when the release is blocked', () => {
    withItems([candidate({ blocked_reason: 'Helm release is pending-upgrade' })])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText('Helm release is pending-upgrade')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Upgrade' })).toBeDisabled()
  })

  // 화면을 다시 열어도 서버에서 이어지는 실행을 보여 준다.
  it('shows the running upgrade and blocks another one', () => {
    withItems([candidate({ last_run: run({ current_step: 'health_check' }) })])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/Upgrading — verifying health/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Upgrade' })).toBeDisabled()
  })

  it('reports a rolled back run with its cause', () => {
    withItems([candidate({ last_run: run({ status: 'rolled_back', current_step: 'rollback', error: 'argocd-server not ready' }) })])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/Failed and rolled back to 7\.7\.16/)).toBeInTheDocument()
    expect(screen.getByText(/argocd-server not ready/)).toBeInTheDocument()
  })

  it('reports a run that needs manual intervention', () => {
    withItems([candidate({ last_run: run({ status: 'manual_intervention_required', error: 'rollback failed' }) })])
    render(<StackVersionUpgradeTab stackId="stk_1" />)
    expect(screen.getByText(/Manual intervention required/)).toBeInTheDocument()
    expect(screen.getByText(/rollback failed/)).toBeInTheDocument()
  })

  it('runs preflight, requires a reason and starts with a stable idempotency key', async () => {
    withItems([candidate()])
    mockPreflight.mockResolvedValue({ checks: passing })
    mockStart.mockRejectedValueOnce(new Error('network down')).mockResolvedValueOnce(run())
    render(<StackVersionUpgradeTab stackId="stk_1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Upgrade' }))
    const dialog = await screen.findByRole('dialog')
    expect(mockPreflight).toHaveBeenCalledWith({ stackId: 'stk_1', bundleId: 'b-20' })
    await within(dialog).findByText('Helm server-side dry-run passed')
    expect(within(dialog).getByText(/API server restarts/)).toBeInTheDocument()

    const start = within(dialog).getByRole('button', { name: 'Start upgrade' })
    expect(start).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'CVE-2026-1234 patch' } })
    expect(start).toBeEnabled()

    fireEvent.click(start)
    await waitFor(() => expect(mockStart).toHaveBeenCalledTimes(1))
    expect(await within(dialog).findByText(/network down/)).toBeInTheDocument()

    fireEvent.click(start)
    await waitFor(() => expect(mockStart).toHaveBeenCalledTimes(2))
    const [first, second] = mockStart.mock.calls.map((call) => call[0])
    expect(second).toMatchObject({ stackId: 'stk_1', bundleId: 'b-20', reason: 'CVE-2026-1234 patch' })
    expect(second.idempotencyKey).toBeTruthy()
    expect(second.idempotencyKey).toBe(first.idempotencyKey)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps Start disabled and shows how to fix a blocked check', async () => {
    withItems([candidate()])
    mockPreflight.mockResolvedValue({
      checks: [{ name: 'backup', status: 'blocked', message: 'No restorable backup', remediation: 'Take a backup first' }],
    })
    render(<StackVersionUpgradeTab stackId="stk_1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Upgrade' }))
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByText('No restorable backup')
    expect(within(dialog).getByText('Take a backup first')).toBeInTheDocument()
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'x' } })
    expect(within(dialog).getByRole('button', { name: 'Start upgrade' })).toBeDisabled()
  })

  it('keeps the dialog open when the server records the attempt as blocked', async () => {
    withItems([candidate()])
    mockPreflight.mockResolvedValue({ checks: passing })
    mockStart.mockResolvedValue(
      run({ status: 'blocked', checks: [{ name: 'active_run', status: 'blocked', message: 'Another upgrade is running' }] }),
    )
    render(<StackVersionUpgradeTab stackId="stk_1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Upgrade' }))
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByText('Helm server-side dry-run passed')
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'x' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Start upgrade' }))
    expect(await within(dialog).findByText('Another upgrade is running')).toBeInTheDocument()
  })
})
