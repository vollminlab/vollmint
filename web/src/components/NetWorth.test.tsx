import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { NetWorth } from './NetWorth'

const fixture = {
  series: [
    { date: '2026-08-01', total: '1500.00', accounts: { 'act-1': '2000.00', 'manual-mortgage': '-500.00' } },
    { date: '2026-08-02', total: '1600.00', accounts: { 'act-1': '2100.00', 'manual-mortgage': '-500.00' } },
  ],
  accounts: [
    {
      id: 'act-1', name: 'Ally Checking', owner: 'scott',
      is_manual: false, balance: '2100.00', balance_date: '2026-08-02',
    },
    {
      id: 'manual-mortgage', name: 'Mortgage', owner: 'joint',
      is_manual: true, balance: '-500.00', balance_date: '2026-08-02',
    },
  ],
}

function stubFetch(body: unknown = fixture) {
  return vi.fn().mockResolvedValue({ ok: true, json: async () => body })
}

beforeEach(() => {
  vi.stubGlobal('fetch', stubFetch())
})

describe('NetWorth', () => {
  it('fetches the 3m range by default and renders the summary', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Ally Checking')).toBeInTheDocument())
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/networth')
    expect(url).toContain('view=household')
    expect(url).toContain('range=3m')
    expect(screen.getByText('Assets')).toBeInTheDocument()
    // These amounts appear in BOTH the summary and an account row — use getAllByText.
    expect(screen.getAllByText('$2,100.00').length).toBeGreaterThan(0)  // assets
    expect(screen.getAllByText('-$500.00').length).toBeGreaterThan(0)   // liabilities
    expect(screen.getByText('$1,600.00')).toBeInTheDocument()           // net worth (summary only)
  })

  it('refetches when a range button is clicked', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Ally Checking')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '1Y' }))
    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((c) => c[0] as string)
      expect(urls.some((u) => u.includes('range=1y'))).toBe(true)
    })
  })

  it('drills down to one account and back', async () => {
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Ally Checking')).toBeInTheDocument())
    expect(screen.getByRole('heading', { name: 'Net Worth' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Ally Checking' }))
    expect(screen.getByRole('heading', { name: 'Ally Checking' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'All accounts' }))
    expect(screen.getByRole('heading', { name: 'Net Worth' })).toBeInTheDocument()
  })

  it('resets drill-down when the view changes', async () => {
    const { rerender } = render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Ally Checking')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Ally Checking' }))
    expect(screen.getByRole('heading', { name: 'Ally Checking' })).toBeInTheDocument()
    rerender(<NetWorth view="scott" />)
    expect(screen.getByRole('heading', { name: 'Net Worth' })).toBeInTheDocument()
  })

  it('saves an inline manual balance edit via PUT', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Mortgage')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    fireEvent.change(screen.getByLabelText('new balance'), { target: { value: '-490.00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      const put = fetchMock.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'PUT')
      expect(put).toBeTruthy()
      expect(put![0]).toContain('/api/accounts/manual-mortgage/balance')
      expect(put![1]!.body).toBe(JSON.stringify({ balance: '-490.00' }))
    })
  })

  it('adds a manual account via POST', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Mortgage')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('new account name'), { target: { value: '401k' } })
    fireEvent.change(screen.getByLabelText('new account owner'), { target: { value: 'scott' } })
    fireEvent.change(screen.getByLabelText('new account balance'), { target: { value: '412000.00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add account' }))
    await waitFor(() => {
      const post = fetchMock.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'POST')
      expect(post).toBeTruthy()
      expect(post![0]).toContain('/api/accounts/manual')
      expect(post![1]!.body).toBe(
        JSON.stringify({ name: '401k', owner: 'scott', balance: '412000.00' }),
      )
    })
  })

  it('shows a stale hint for manual balances older than 60 days', async () => {
    const stale = {
      series: fixture.series,
      accounts: [
        { ...fixture.accounts[1], balance_date: '2020-01-01' },
      ],
    }
    vi.stubGlobal('fetch', stubFetch(stale))
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Mortgage')).toBeInTheDocument())
    expect(screen.getByText('update?')).toBeInTheDocument()
  })

  it('renders the empty state when there are no snapshots', async () => {
    vi.stubGlobal('fetch', stubFetch({ series: [], accounts: fixture.accounts }))
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Ally Checking')).toBeInTheDocument())
    expect(screen.getByText(/No balance history yet/)).toBeInTheDocument()
  })

  it('surfaces API errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) }),
    )
    render(<NetWorth view="household" />)
    await waitFor(() => expect(screen.getByText('Error: boom')).toBeInTheDocument())
  })
})
