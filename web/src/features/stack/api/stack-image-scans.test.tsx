import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../../lib/api'
import { useStackImageScans } from './stack-api'
import { normalizeStackImageScanReport } from './stack-normalizers'

vi.mock('../../../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

const scannedResponse = {
  stack_id: 'stk_6da6f2e5f32c',
  status: 'scanned',
  reason: '',
  last_scanned_at: '2026-09-14T01:02:03Z',
  summary: { critical: 1, high: 10, medium: 40, low: 30, unknown: 2 },
  items: [
    {
      image: 'docker.io/bitnami/postgresql:16.4.0',
      image_digest: 'sha256:abc',
      release: 'nullus-postgresql',
      workloads: ['nullus-postgresql-0'],
      status: 'scanned',
      error: '',
      counts: { critical: 0, high: 2, medium: 5, low: 3, unknown: 0 },
      fixable_counts: { critical: 0, high: 1, medium: 2, low: 0, unknown: 0 },
      scanner_version: '0.74.0',
      db_updated_at: '2026-09-13T07:13:14Z',
      db_stale: false,
      scanned_at: '2026-09-14T01:02:03Z',
    },
  ],
  total: 1,
}

describe('normalizeStackImageScanReport', () => {
  it('응답을 화면 모델로 옮긴다', () => {
    const report = normalizeStackImageScanReport(scannedResponse)

    expect(report.stackId).toBe('stk_6da6f2e5f32c')
    expect(report.status).toBe('scanned')
    expect(report.lastScannedAt).toBe('2026-09-14T01:02:03Z')
    expect(report.summary).toEqual({ critical: 1, high: 10, medium: 40, low: 30, unknown: 2 })
    expect(report.total).toBe(1)
    expect(report.items[0]).toEqual({
      image: 'docker.io/bitnami/postgresql:16.4.0',
      imageDigest: 'sha256:abc',
      release: 'nullus-postgresql',
      workloads: ['nullus-postgresql-0'],
      status: 'scanned',
      error: undefined,
      counts: { critical: 0, high: 2, medium: 5, low: 3, unknown: 0 },
      fixableCounts: { critical: 0, high: 1, medium: 2, low: 0, unknown: 0 },
      scannerVersion: '0.74.0',
      dbUpdatedAt: '2026-09-13T07:13:14Z',
      dbStale: false,
      scannedAt: '2026-09-14T01:02:03Z',
    })
  })

  // 스캔하지 않은 스택의 summary 는 null 이다. 0 으로 채우면 "취약점 없음" 으로 읽힌다.
  it('스캔하지 않은 스택은 요약·시각을 모름으로 둔다', () => {
    const report = normalizeStackImageScanReport({
      stack_id: 'stk_1',
      status: 'not_scanned',
      reason: 'scanner_not_installed',
      summary: null,
      items: [],
      total: 0,
    })

    expect(report.status).toBe('not_scanned')
    expect(report.reason).toBe('scanner_not_installed')
    expect(report.summary).toBeUndefined()
    expect(report.lastScannedAt).toBeUndefined()
    expect(report.items).toEqual([])
  })

  it('실패한 항목은 건수 없이 오류를 남기고, 빠진 건수는 모름으로 둔다', () => {
    const report = normalizeStackImageScanReport({
      stack_id: 'stk_1',
      status: 'scanned',
      items: [
        { image: 'quay.io/x:1', status: 'failed', error: 'manifest unknown' },
        { image: 'quay.io/y:1', status: 'scanned', db_stale: true },
      ],
    })

    expect(report.items[0].status).toBe('failed')
    expect(report.items[0].error).toBe('manifest unknown')
    expect(report.items[0].counts).toBeUndefined()
    expect(report.items[1].counts).toBeUndefined()
    expect(report.items[1].fixableCounts).toBeUndefined()
    expect(report.items[1].dbStale).toBe(true)
    expect(report.items[1].workloads).toEqual([])
    expect(report.total).toBe(2)
  })

  it('모르는 상태 값은 스캔하지 않음으로 떨어진다', () => {
    const report = normalizeStackImageScanReport({ stack_id: 'stk_1', status: 'weird' })

    expect(report.status).toBe('not_scanned')
    expect(report.reason).toBe('')
  })
})

describe('useStackImageScans', () => {
  const wrapper = ({ children }: { children: ReactNode }) => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('스택의 이미지 스캔 보고를 읽어 정규화한다', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: scannedResponse } as never)

    const { result } = renderHook(() => useStackImageScans('stk_6da6f2e5f32c'), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(vi.mocked(api.get)).toHaveBeenCalledWith('/stacks/stk_6da6f2e5f32c/image-scans')
    expect(result.current.data?.summary?.critical).toBe(1)
  })

  // 엔드포인트가 없는 서버(404)나 배선 전(503)은 오류 상태로만 끝나야 한다 —
  // 화면은 그걸 "데이터 없음" 으로 그린다.
  it('503 은 재시도 없이 오류 상태가 된다', async () => {
    vi.mocked(api.get).mockRejectedValue({ status: 503, message: 'IMAGE_SCANS_NOT_CONFIGURED' })

    const { result } = renderHook(() => useStackImageScans('stk_1'), { wrapper })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(vi.mocked(api.get)).toHaveBeenCalledTimes(1)
  })

  it('stackId 가 없으면 조회하지 않는다', () => {
    renderHook(() => useStackImageScans(''), { wrapper })

    expect(vi.mocked(api.get)).not.toHaveBeenCalled()
  })
})
