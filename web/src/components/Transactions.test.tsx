import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Transactions } from './Transactions'

const txns = {
  transactions: [
    {
      id: 5, source: 'simplefin', account_id: 'ally-s', account_name: 'Ally',
      posted: '2026-07-05', amount: '-100.00', description: 'WHOLE FOODS', payee: 'WHOLE FOODS',
      pending: false, category_id: 2, category_name: 'Groceries',
      owner_override: null, effective_owner: 'scott', transfer_peer_id: null,
    },
  ],
}
const cats = { categories: [{ id: 2, name: 'Groceries', parent_id: null, kind: 'spend', is_vice: false }] }

function stubFetch() {
  return vi.fn((url: string) => {
    const body = url.startsWith('/api/categories') ? cats : txns
    return Promise.resolve({ ok: true, json: async () => body })
  })
}

beforeEach(() => {
  vi.stubGlobal('fetch', stubFetch())
})

describe('Transactions drill-down plumbing', () => {
  it('reads the category filter from the URL and passes it to the API', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07&category=2']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('WHOLE FOODS')).toBeInTheDocument())
    // the transactions request must have carried category=2 and view=scott
    const calledUrls = fetchMock.mock.calls.map((c) => c[0] as string)
    const txnCall = calledUrls.find((u) => u.startsWith('/api/transactions'))!
    expect(txnCall).toContain('category=2')
    expect(txnCall).toContain('view=scott')
    expect(txnCall).toContain('month=2026-07')
  })

  it('shows a heading noting the active category filter', async () => {
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07&category=2']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText(/Filtered by category/i)).toBeInTheDocument())
  })
})
