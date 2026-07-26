import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../../lib/api'
import { useExportStackConfig, useImportStackConfig, usePreviewImportStackConfig } from './stack-api'

vi.mock('../../../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

describe('useExportStackConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('requests blob export and parses the server filename', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: new Blob(['{"name":"demo"}'], { type: 'application/json' }),
      headers: {
        'content-disposition': 'attachment; filename="stack-demo.yaml"',
        'content-type': 'application/x-yaml',
      },
    } as never)

    const wrapper = ({ children }: { children: ReactNode }) => {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      })

      return <QueryClientProvider client={client}>{children}</QueryClientProvider>
    }

    const { result } = renderHook(() => useExportStackConfig(), { wrapper })
    const response = await result.current.mutateAsync({
      stackId: 'stack-123',
      format: 'yaml',
    })

    expect(vi.mocked(api.get)).toHaveBeenCalledWith('/stacks/stack-123/export', {
      params: { format: 'yaml' },
      responseType: 'blob',
    })
    expect(response.filename).toBe('stack-demo.yaml')
    expect(response.contentType).toBe('application/x-yaml')
  })

  it('posts raw import payload and returns the new stack id', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { id: 'stk-imported' },
    } as never)

    const wrapper = ({ children }: { children: ReactNode }) => {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      })

      return <QueryClientProvider client={client}>{children}</QueryClientProvider>
    }

    const { result } = renderHook(() => useImportStackConfig(), { wrapper })
    const response = await result.current.mutateAsync({ payload: '{"name":"demo"}' })

    expect(vi.mocked(api.post)).toHaveBeenCalledWith(
      '/stacks/import?replace_existing=false',
      '{"name":"demo"}',
      { headers: { 'Content-Type': 'text/plain' } },
    )
    expect(response.id).toBe('stk-imported')
  })

  it('requests import preview', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { mode: 'update', name: 'demo', cluster_id: 'cluster-1', existing_stack_id: 'stk-1' },
    } as never)

    const wrapper = ({ children }: { children: ReactNode }) => {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      })

      return <QueryClientProvider client={client}>{children}</QueryClientProvider>
    }

    const { result } = renderHook(() => usePreviewImportStackConfig(), { wrapper })
    const response = await result.current.mutateAsync('{"name":"demo"}')

    expect(vi.mocked(api.post)).toHaveBeenCalledWith(
      '/stacks/import/preview',
      '{"name":"demo"}',
      { headers: { 'Content-Type': 'text/plain' } },
    )
    expect(response.mode).toBe('update')
  })
})
