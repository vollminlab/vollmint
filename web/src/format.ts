// Display helpers. money() parses the decimal string ONLY to drive
// Intl.NumberFormat's grouping — the value is never stored or re-serialized as
// a number, so float imprecision cannot leak back into data.
const usd = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

export function money(v: string): string {
  if (!v) return '$0.00'
  const n = Number(v)
  if (Number.isNaN(n)) return v
  return usd.format(n)
}

// shiftMonth adds delta months to a YYYY-MM string.
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  const idx = (y * 12 + (m - 1)) + delta
  const ny = Math.floor(idx / 12)
  const nm = (idx % 12) + 1
  return `${ny.toString().padStart(4, '0')}-${nm.toString().padStart(2, '0')}`
}

const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]

export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number)
  return `${MONTHS[m - 1]} ${y}`
}

// currentMonth returns today's month as YYYY-MM.
export function currentMonth(): string {
  const d = new Date()
  return `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, '0')}`
}
