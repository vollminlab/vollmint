import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { UpcomingBills } from './UpcomingBills'
import * as api from '../api'
import type { Forecast } from '../api'

vi.mock('../api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../api')>()
  return { ...mod, getForecast: vi.fn() }
})

const forecast: Forecast = {
  month: '2026-07',
  view: 'household',
  remaining_expected: '128.42',
  bills: [
    {
      payee: 'VERIZON WIRELESS', category_id: 12, category: 'Utilities',
      predicted_day: 14, expected_amount: '128.42',
      paid: false, paid_date: '', paid_amount: '',
    },
    {
      payee: 'NETFLIX', category_id: 3, category: 'Subscriptions',
      predicted_day: 20, expected_amount: '17.99',
      paid: true, paid_date: '2026-07-20', paid_amount: '17.99',
    },
  ],
}

describe('UpcomingBills', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders unpaid bills with a predicted date and paid bills with a checkmark', async () => {
    vi.mocked(api.getForecast).mockResolvedValue({ forecast })
    render(<UpcomingBills view="household" month="2026-07" />)
    expect(await screen.findByText(/Upcoming bills/)).toBeInTheDocument()
    expect(screen.getByText(/\$128\.42 remaining expected/)).toBeInTheDocument()
    expect(screen.getByText('Verizon Wireless')).toBeInTheDocument()
    expect(screen.getByText('~Jul 14')).toBeInTheDocument()
    expect(screen.getByText('Netflix')).toBeInTheDocument()
    expect(screen.getByText(/Jul 20 ✓/)).toBeInTheDocument()
  })

  it('shows an empty state when no bills are detected', async () => {
    vi.mocked(api.getForecast).mockResolvedValue({
      forecast: { month: '2026-07', view: 'household', bills: [], remaining_expected: '0' },
    })
    render(<UpcomingBills view="household" month="2026-07" />)
    expect(await screen.findByText('No recurring bills detected yet.')).toBeInTheDocument()
  })
})
