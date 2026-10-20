import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../../lib/api'
import { AssistantSettingsPage } from './assistant-settings-page'

vi.mock('../../../lib/api', () => ({
  api: { get: vi.fn(), put: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const providers = [
  { id: 'upstage', protocol: 'openai', default_base_url: 'https://api.upstage.ai/v1', default_model: 'solar-pro4', key_required: true },
  { id: 'openai', protocol: 'openai', default_base_url: 'https://api.openai.com/v1', key_required: true },
  { id: 'anthropic', protocol: 'anthropic', default_base_url: 'https://api.anthropic.com', default_model: 'claude-sonnet-5-5', key_required: true },
  { id: 'gemini', protocol: 'openai', default_base_url: 'https://generativelanguage.googleapis.com/v1beta/openai', key_required: true },
  { id: 'openai-compatible', protocol: 'openai', key_required: false },
]

function settings(overrides: Record<string, unknown> = {}) {
  return {
    enabled: true, provider: 'upstage', base_url: '', model: '', has_key: true, key_hint: '••••1234',
    effective_base_url: 'https://api.upstage.ai/v1', effective_model: 'solar-pro4', providers, ...overrides,
  }
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AssistantSettingsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.mocked(api.get).mockReset()
  vi.mocked(api.put).mockReset()
  vi.mocked(api.post).mockReset()
  vi.mocked(api.delete).mockReset()
})

describe('AssistantSettingsPage', () => {
  // 키 원문은 다시 보이지 않는다 — 끝 네 자리만. 키 칸을 비워 두고 저장하면 저장된 키를 그대로 쓴다.
  it('keeps the saved key when the key field is left empty', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings() } as never)
    vi.mocked(api.put).mockResolvedValue({ data: settings({ model: 'solar-mini' }) } as never)
    renderPage()

    expect(await screen.findByText(/Saved key ••••1234/)).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Upstage Solar/ })).toBeChecked()
    fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'solar-mini' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.put).toHaveBeenCalled())
    expect(vi.mocked(api.put).mock.calls[0]).toEqual([
      '/admin/assistant',
      { enabled: true, provider: 'upstage', base_url: '', model: 'solar-mini' },
    ])
  })

  // 다른 벤더로 바꾸면 그 벤더의 키가 필요하다 — 이전 벤더의 키는 쓰지 않는다.
  it('switches vendor and sends the new key', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings() } as never)
    vi.mocked(api.put).mockResolvedValue({ data: settings({ provider: 'anthropic' }) } as never)
    renderPage()

    fireEvent.click(await screen.findByRole('radio', { name: /Anthropic Claude/ }))
    expect(screen.getByText('Enter the API key for this provider.')).toBeInTheDocument()
    expect(screen.getByLabelText('Model')).toHaveAttribute('placeholder', 'claude-sonnet-5-5')
    fireEvent.change(screen.getByLabelText('API key'), { target: { value: 'sk-ant-new' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.put).toHaveBeenCalled())
    expect(vi.mocked(api.put).mock.calls[0][1]).toEqual({
      enabled: true, provider: 'anthropic', base_url: '', model: '', api_key: 'sk-ant-new',
    })
  })

  it('requires an address for an OpenAI-compatible server', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings({ enabled: false, has_key: false, key_hint: '' }) } as never)
    renderPage()

    fireEvent.click(await screen.findByRole('radio', { name: /OpenAI-compatible/ }))
    expect(screen.getByLabelText('Base URL')).toHaveAttribute('placeholder', 'https://llm.internal/v1')
    expect(screen.getByText('Optional for an in-house server.')).toBeInTheDocument()
  })

  it('tests the connection without saving', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings() } as never)
    vi.mocked(api.post)
      .mockResolvedValueOnce({ data: { ok: true, model: 'solar-pro4', error: '', latency_ms: 812 } } as never)
      .mockResolvedValueOnce({ data: { ok: false, model: 'solar-pro4', error: '모델 응답 401: invalid key', latency_ms: 90 } } as never)
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Test connection' }))
    expect(await screen.findByText('Connected — solar-pro4 answered in 812 ms.')).toBeInTheDocument()
    expect(vi.mocked(api.post).mock.calls[0]).toEqual([
      '/admin/assistant/test',
      { enabled: true, provider: 'upstage', base_url: '', model: '' },
    ])
    expect(api.put).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Test connection' }))
    expect(await screen.findByText('Connection failed: 모델 응답 401: invalid key')).toBeInTheDocument()
  })

  it('shows why the server rejected the settings', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings({ has_key: false, key_hint: '' }) } as never)
    vi.mocked(api.put).mockRejectedValue({ status: 400, message: 'assistant: invalid settings: API 키가 필요하다' })
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('API 키가 필요하다')
  })

  it('removes the saved key', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings({ enabled: false }) } as never)
    vi.mocked(api.put).mockResolvedValue({ data: settings({ enabled: false, has_key: false, key_hint: '' }) } as never)
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Remove key' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.put).toHaveBeenCalled())
    expect(vi.mocked(api.put).mock.calls[0][1]).toMatchObject({ api_key: '' })
  })

  it('deletes the settings after confirmation', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings() } as never)
    vi.mocked(api.delete).mockResolvedValue({ data: null } as never)
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Delete settings' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(api.delete).toHaveBeenCalledWith('/admin/assistant'))
  })

  // 주소를 바꾸면 저장된 키를 이어 쓰지 않는다 — 다른 서버로 키를 보내지 않게.
  it('asks for the key again when the address changes', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings() } as never)
    renderPage()

    fireEvent.change(await screen.findByLabelText('Base URL'), { target: { value: 'https://proxy.example/v1' } })
    expect(screen.getByText('The address changed — enter the API key again.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove key' })).not.toBeInTheDocument()

    // 기본 주소를 그대로 적어 넣은 것은 같은 곳이다.
    fireEvent.change(screen.getByLabelText('Base URL'), { target: { value: 'https://api.upstage.ai/v1/' } })
    expect(screen.getByText(/Saved key ••••1234/)).toBeInTheDocument()
  })

  it('explains a saved key that can no longer be read', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: settings({ has_key: false, key_hint: '', key_unreadable: true }) } as never)
    renderPage()

    expect(await screen.findByText(/The saved key cannot be read/)).toBeInTheDocument()
  })
})
