import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Dashboard } from './Dashboard'

const summaryPayload = {
  summary: { in: '3000.00', out: '140.00', vices: '40.00', budget_total: '120.00', month: '2026-07', view: 'household' },
  categories: [
    { category_id: 2, category: 'Groceries', spent: '100.00', budget: '120.00', is_vice: false },
    { category_id: 3, category: 'Dining', spent: '40.00', budget: '', is_vice: true },
  ],
}

const forecastPayload = {
  forecast: { month: '2026-07', view: 'household', bills: [], remaining_expected: '0.00' },
}

beforeEach(() => {
  // Dashboard fires two fetches: the summary and UpcomingBills' forecast.
  // Answer by URL — a single mockResolvedValue would feed the summary
  // payload to the forecast call and strand UpcomingBills on "Loading…".
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation(async (url: string) => ({
      ok: true,
      json: async () => (String(url).includes('/api/forecast') ? forecastPayload : summaryPayload),
    })),
  )
})

describe('Dashboard', () => {
  it('shows summary cards and category bars', async () => {
    render(
      <MemoryRouter initialEntries={['/?view=household&month=2026-07']}>
        <Dashboard view="household" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('$3,000.00')).toBeInTheDocument()) // In
    expect(screen.getByText('$140.00')).toBeInTheDocument() // Out
    expect(screen.getByText('Groceries')).toBeInTheDocument()
    // UpcomingBills resolves past Loading… with the forecast payload.
    await waitFor(() => expect(screen.getByText('No recurring bills detected yet.')).toBeInTheDocument())
  })

  it('each category bar deep-links to Transactions pre-filtered by category', async () => {
    render(
      <MemoryRouter initialEntries={['/?view=scott&month=2026-07']}>
        <Dashboard view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    const link = await screen.findByRole('link', { name: /Groceries/ })
    const href = link.getAttribute('href')!
    expect(href).toContain('/transactions')
    expect(href).toContain('category=2')
    expect(href).toContain('view=scott')
    expect(href).toContain('month=2026-07')
  })
})
