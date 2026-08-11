import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

// stub the API so the dashboard's initial load doesn't hit the network
beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        summary: { in: '0.00', out: '0.00', vices: '0.00', budget_total: '0.00', month: '2026-07', view: 'household' },
        categories: [],
        transactions: [],
        recurring: [],
      }),
    }),
  )
})

describe('App', () => {
  it('renders the nav with all seven pages', () => {
    render(
      <MemoryRouter initialEntries={['/?view=household&month=2026-07']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Transactions' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Recurring' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Trends' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Net Worth' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Budgets' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Rules' })).toBeInTheDocument()
  })
})
