import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MonthPager } from './MonthPager'

describe('MonthPager', () => {
  it('renders the label and calls onChange with shifted month', async () => {
    const onChange = vi.fn()
    render(<MonthPager month="2026-07" onChange={onChange} />)
    expect(screen.getByText('July 2026')).toBeInTheDocument()
    await userEvent.click(screen.getByLabelText('previous month'))
    expect(onChange).toHaveBeenCalledWith('2026-06')
    await userEvent.click(screen.getByLabelText('next month'))
    expect(onChange).toHaveBeenCalledWith('2026-08')
  })
})
