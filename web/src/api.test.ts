import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getSummary, getTransactions, patchTransaction, buildQuery } from './api'

describe('buildQuery', () => {
  it('omits empty params and encodes present ones', () => {
    expect(buildQuery({ view: 'household', month: '2026-07', q: '' })).toBe(
      '?view=household&month=2026-07',
    )
    expect(buildQuery({ view: 'scott', category: 3 })).toBe('?view=scott&category=3')
  })
})

describe('api client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('getSummary hits the right URL and returns parsed JSON', async () => {
    const payload = { summary: { in: '10.00', out: '5.00', vices: '0.00', budget_total: '0.00' }, categories: [] }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => payload,
    })
    vi.stubGlobal('fetch', fetchMock)
    const res = await getSummary('household', '2026-07')
    expect(fetchMock).toHaveBeenCalledWith('/api/summary?view=household&month=2026-07')
    expect(res.summary.out).toBe('5.00')
  })

  it('getTransactions passes filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ transactions: [] }) })
    vi.stubGlobal('fetch', fetchMock)
    await getTransactions({ view: 'scott', month: '2026-07', category: 4, uncategorized: true })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/transactions?view=scott&month=2026-07&category=4&uncategorized=true',
    )
  })

  it('patchTransaction sends PATCH with JSON body', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ status: 'ok' }) })
    vi.stubGlobal('fetch', fetchMock)
    await patchTransaction(7, { category_id: 3 })
    expect(fetchMock).toHaveBeenCalledWith('/api/transactions/7', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ category_id: 3 }),
    })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) })
    vi.stubGlobal('fetch', fetchMock)
    await expect(getSummary('household', '2026-07')).rejects.toThrow('boom')
  })
})
