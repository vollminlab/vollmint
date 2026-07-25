import { describe, it, expect } from 'vitest'
import { money, shiftMonth, monthLabel } from './format'

describe('money', () => {
  it('formats a decimal string as USD', () => {
    expect(money('1234.50')).toBe('$1,234.50')
    expect(money('0.00')).toBe('$0.00')
    expect(money('-42.00')).toBe('-$42.00')
  })
  it('handles empty/undefined as $0.00', () => {
    expect(money('')).toBe('$0.00')
  })
})

describe('shiftMonth', () => {
  it('moves forward and backward across year boundaries', () => {
    expect(shiftMonth('2026-07', 1)).toBe('2026-08')
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
  })
})

describe('monthLabel', () => {
  it('renders a human label', () => {
    expect(monthLabel('2026-07')).toBe('July 2026')
  })
})
