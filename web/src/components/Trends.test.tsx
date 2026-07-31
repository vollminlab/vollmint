import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { Trends } from './Trends'

const trends = {
  trends: [
    { month: '2026-05', in: '3000.00', out: '1200.00' },
    { month: '2026-06', in: '3000.00', out: '900.50' },
    { month: '2026-07', in: '0', out: '450.00' },
  ],
}

function stubFetch() {
  return vi.fn().mockResolvedValue({ ok: true, json: async () => trends })
}

beforeEach(() => {
  vi.stubGlobal('fetch', stubFetch())
})

describe('Trends', () => {
  it('fetches 12 months by default for the current view+month', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Trends view="scott" month="2026-07" />)
    await waitFor(() => expect(screen.getByText('Income')).toBeInTheDocument())
    // Trends mounts InsightCards as a child, which fires its own fetch (to
    // /api/insights) concurrently — find the trends call rather than assuming
    // call order between sibling/child effects.
    const url = fetchMock.mock.calls.map((c) => c[0] as string).find((u) => u.includes('/api/trends'))!
    expect(url).toContain('/api/trends')
    expect(url).toContain('view=scott')
    expect(url).toContain('month=2026-07')
    expect(url).toContain('months=12')
  })

  it('renders income and spending series in the legend', async () => {
    render(<Trends view="household" month="2026-07" />)
    await waitFor(() => expect(screen.getByText('Income')).toBeInTheDocument())
    expect(screen.getByText('Spending')).toBeInTheDocument()
  })

  it('refetches when the window selector changes', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Trends view="household" month="2026-07" />)
    await waitFor(() => expect(screen.getByText('Income')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('months window'), { target: { value: '24' } })
    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((c) => c[0] as string)
      expect(urls.some((u) => u.includes('months=24'))).toBe(true)
    })
  })

  it('surfaces API errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) }),
    )
    render(<Trends view="household" month="2026-07" />)
    await waitFor(() => expect(screen.getByText('Error: boom')).toBeInTheDocument())
  })
})
