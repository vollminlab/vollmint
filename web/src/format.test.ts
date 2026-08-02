import { describe, it, expect } from 'vitest'
import { money, shiftMonth, monthLabel, titleCase, monthDayLabel, dateLabel } from './format'

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

describe('titleCase', () => {
  it('title-cases an uppercase payee', () => {
    expect(titleCase('VERIZON WIRELESS')).toBe('Verizon Wireless')
  })
  it('handles single words', () => {
    expect(titleCase('NETFLIX')).toBe('Netflix')
  })
})

describe('monthDayLabel', () => {
  it('formats a month + day', () => {
    expect(monthDayLabel('2026-07', 14)).toBe('Jul 14')
  })

  it('clamps the day to the month length', () => {
    expect(monthDayLabel('2026-02', 30)).toBe('Feb 28')
    expect(monthDayLabel('2028-02', 30)).toBe('Feb 29')
    expect(monthDayLabel('2026-06', 31)).toBe('Jun 30')
  })
})

describe('dateLabel', () => {
  it('formats an ISO date', () => {
    expect(dateLabel('2026-07-13')).toBe('Jul 13')
  })
})
