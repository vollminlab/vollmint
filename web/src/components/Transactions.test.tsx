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
      splits: [],
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

describe('Transactions splits', () => {
  it('shows a split badge with part names instead of the category select', async () => {
    const splitTxns = {
      transactions: [
        {
          id: 9, source: 'simplefin', account_id: 'ally-s', account_name: 'Ally',
          posted: '2026-07-06', amount: '-50.00', description: 'VENMO PAYMENT', payee: 'VENMO',
          pending: false, category_id: 1, category_name: 'Dining',
          owner_override: null, effective_owner: 'scott', transfer_peer_id: null,
          splits: [
            { id: 1, category_id: 1, category: 'Dining', amount: '-30.00', note: '' },
            { id: 2, category_id: 2, category: 'Groceries', amount: '-20.00', note: '' },
          ],
        },
      ],
    }
    const fetchMock = vi.fn((url: string) => {
      const body = url.startsWith('/api/categories') ? cats : splitTxns
      return Promise.resolve({ ok: true, json: async () => body })
    })
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    expect(await screen.findByText('Split · Dining + Groceries')).toBeInTheDocument()
    expect(screen.queryByLabelText('category for VENMO')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /unsplit/i })).toBeInTheDocument()
  })

  it('hides the Split action for pending and transfer rows', async () => {
    const rowsBody = {
      transactions: [
        {
          id: 10, source: 'simplefin', account_id: 'ally-s', account_name: 'Ally',
          posted: '2026-07-07', amount: '-10.00', description: 'PENDING CHARGE', payee: 'PENDING CHARGE',
          pending: true, category_id: 2, category_name: 'Groceries',
          owner_override: null, effective_owner: 'scott', transfer_peer_id: null,
          splits: [],
        },
        {
          id: 11, source: 'simplefin', account_id: 'ally-s', account_name: 'Ally',
          posted: '2026-07-08', amount: '-25.00', description: 'TRANSFER OUT', payee: 'TRANSFER OUT',
          pending: false, category_id: 2, category_name: 'Groceries',
          owner_override: null, effective_owner: 'scott', transfer_peer_id: 99,
          splits: [],
        },
      ],
    }
    const fetchMock = vi.fn((url: string) => {
      const body = url.startsWith('/api/categories') ? cats : rowsBody
      return Promise.resolve({ ok: true, json: async () => body })
    })
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('PENDING CHARGE')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: /^split$/i })).not.toBeInTheDocument()
  })

  it('opens and closes the inline split editor via the Split button', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('WHOLE FOODS')).toBeInTheDocument())
    expect(screen.queryByLabelText(/amount for part 1/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^split$/i }))
    expect(screen.getByLabelText(/amount for part 1/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^split$/i }))
    expect(screen.queryByLabelText(/amount for part 1/)).not.toBeInTheDocument()
  })

  it('unsplits a row via DELETE and reverts to the category select after reload', async () => {
    const splitTxn = {
      id: 9, source: 'simplefin', account_id: 'ally-s', account_name: 'Ally',
      posted: '2026-07-06', amount: '-50.00', description: 'VENMO PAYMENT', payee: 'VENMO',
      pending: false, category_id: 1, category_name: 'Dining',
      owner_override: null, effective_owner: 'scott', transfer_peer_id: null,
      splits: [
        { id: 1, category_id: 1, category: 'Dining', amount: '-30.00', note: '' },
        { id: 2, category_id: 2, category: 'Groceries', amount: '-20.00', note: '' },
      ],
    }
    const unsplitTxn = { ...splitTxn, splits: [] }
    let deleted = false
    const fetchMock = vi.fn((url: string, init?: RequestInit) => {
      if (url.startsWith('/api/categories')) return Promise.resolve({ ok: true, json: async () => cats })
      if (init?.method === 'DELETE') {
        deleted = true
        return Promise.resolve({ ok: true, json: async () => ({ status: 'ok' }) })
      }
      const body = { transactions: [deleted ? unsplitTxn : splitTxn] }
      return Promise.resolve({ ok: true, json: async () => body })
    })
    vi.stubGlobal('fetch', fetchMock)
    render(
      <MemoryRouter initialEntries={['/transactions?view=scott&month=2026-07']}>
        <Transactions view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    expect(await screen.findByText('Split · Dining + Groceries')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /unsplit/i }))

    await waitFor(() => expect(screen.getByLabelText('category for VENMO')).toBeInTheDocument())
    expect(screen.queryByText('Split · Dining + Groceries')).not.toBeInTheDocument()

    const deleteCall = fetchMock.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === 'DELETE')!
    expect(deleteCall[0]).toBe('/api/transactions/9/splits')
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
