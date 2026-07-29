import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Recurring } from './Recurring'

const recurring = {
  recurring: [
    {
      payee: 'NETFLIX', count: 3, months: 3, avg_amount: '15.99',
      last_seen: '2026-07-10', first_seen: '2026-05-10', is_new: false,
    },
    {
      payee: 'HBO MAX', count: 3, months: 3, avg_amount: '10.00',
      last_seen: '2026-09-01', first_seen: '2026-07-01', is_new: true,
    },
  ],
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => recurring }))
})

describe('Recurring', () => {
  it('renders detected recurring charges with formatted amounts', async () => {
    render(
      <MemoryRouter>
        <Recurring view="household" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('NETFLIX')).toBeInTheDocument())
    expect(screen.getByText('HBO MAX')).toBeInTheDocument()
    expect(screen.getByText('$15.99')).toBeInTheDocument()
  })

  it('flags only new charges with a NEW badge', async () => {
    render(
      <MemoryRouter>
        <Recurring view="household" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('HBO MAX')).toBeInTheDocument())
    expect(screen.getAllByText('NEW')).toHaveLength(1)
  })

  it('deep-links each payee into Transactions with a q filter', async () => {
    render(
      <MemoryRouter>
        <Recurring view="scott" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('NETFLIX')).toBeInTheDocument())
    const link = screen.getByRole('link', { name: 'NETFLIX' })
    expect(link.getAttribute('href')).toContain('/transactions')
    expect(link.getAttribute('href')).toContain('q=NETFLIX')
    expect(link.getAttribute('href')).toContain('view=scott')
    expect(link.getAttribute('href')).toContain('month=2026-07')
  })

  it('shows an empty state when nothing recurs', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ recurring: [] }) }))
    render(
      <MemoryRouter>
        <Recurring view="household" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText(/No recurring charges/i)).toBeInTheDocument())
  })

  it('surfaces API errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) }),
    )
    render(
      <MemoryRouter>
        <Recurring view="household" month="2026-07" />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('Error: boom')).toBeInTheDocument())
  })
})
