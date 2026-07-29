import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
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

  it('surfaces an error when recategorize PATCH fails', async () => {
    const fetchMock = vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'PATCH') {
        return Promise.resolve({ ok: false, status: 500, json: async () => ({ error: 'boom' }) })
      }
      const body = url.startsWith('/api/categories') ? cats : txns
      return Promise.resolve({ ok: true, json: async () => body })
    })
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('WHOLE FOODS')).toBeInTheDocument())
    const select = screen.getByLabelText('category for WHOLE FOODS')
    fireEvent.change(select, { target: { value: '2' } })
    await waitFor(() => expect(screen.getByText(/Error:/)).toBeInTheDocument())
  })
})

describe('Transactions q search filter', () => {
  it('reads q from the URL and passes it to the API', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07&q=NETFLIX']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('WHOLE FOODS')).toBeInTheDocument())
    const calledUrls = fetchMock.mock.calls.map((c) => c[0] as string)
    const txnCall = calledUrls.find((u) => u.startsWith('/api/transactions'))!
    expect(txnCall).toContain('q=NETFLIX')
  })

  it('shows a line noting the active search filter', async () => {
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07&q=NETFLIX']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText(/Filtered by search/i)).toBeInTheDocument())
  })

  it('treats an empty q param as no search filter', async () => {
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07&q=']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('WHOLE FOODS')).toBeInTheDocument())
    expect(screen.queryByText(/Filtered by search/i)).not.toBeInTheDocument()
  })
})
