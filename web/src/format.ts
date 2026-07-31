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

const SHORT_MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

export function titleCase(s: string): string {
  return s
    .toLowerCase()
    .split(/\s+/)
    .map((w) => (w ? w[0].toUpperCase() + w.slice(1) : w))
    .join(' ')
}

/** "2026-07" + 14 → "Jul 14" (caller prefixes "~" for predictions) */
export function monthDayLabel(month: string, day: number): string {
  const m = Number(month.slice(5, 7))
  return `${SHORT_MONTHS[m - 1]} ${day}`
}

/** "2026-07-13" → "Jul 13" */
export function dateLabel(iso: string): string {
  const m = Number(iso.slice(5, 7))
  return `${SHORT_MONTHS[m - 1]} ${Number(iso.slice(8, 10))}`
}
