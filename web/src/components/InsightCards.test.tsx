import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { InsightCards } from './InsightCards'
import * as api from '../api'

vi.mock('../api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../api')>()
  return { ...mod, getInsights: vi.fn() }
})

describe('InsightCards', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders one card per insight with the type icon', async () => {
    vi.mocked(api.getInsights).mockResolvedValue({
      insights: [
        {
          type: 'category_spike',
          title: 'Dining is running hot',
          body: "You've spent $200.00 on Dining this month — $100.00 above your 3-month average of $100.00.",
          amount: '100.00',
        },
        {
          type: 'price_increase',
          title: 'Netflix price went up',
          body: 'Netflix went from $15.49 to $17.99 (+$2.50).',
          amount: '2.50',
        },
      ],
    })
    render(<InsightCards view="household" month="2026-07" />)
    expect(await screen.findByText('Dining is running hot')).toBeInTheDocument()
    expect(screen.getByText('📈')).toBeInTheDocument()
    expect(screen.getByText('Netflix price went up')).toBeInTheDocument()
    expect(screen.getByText('⬆️')).toBeInTheDocument()
  })

  it('renders nothing when there are no insights', async () => {
    vi.mocked(api.getInsights).mockResolvedValue({ insights: [] })
    const { container } = render(<InsightCards view="household" month="2026-07" />)
    await waitFor(() => expect(api.getInsights).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})
