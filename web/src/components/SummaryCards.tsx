import type { Summary } from '../api'
import { money } from '../format'

// vsBudget is out − budget_total as a signed decimal, computed in JS for
// DISPLAY only (both operands are exact 2-dp strings, so string math is safe
// here via cents integers — no float rounding).
function centsDiff(a: string, b: string): string {
  const toCents = (s: string) => Math.round(Number(s) * 100)
  const d = toCents(a) - toCents(b)
  const sign = d < 0 ? '-' : ''
  const abs = Math.abs(d)
  return `${sign}${Math.floor(abs / 100)}.${(abs % 100).toString().padStart(2, '0')}`
}

export function SummaryCards({ s }: { s: Summary }) {
  const vs = centsDiff(s.out, s.budget_total)
  const overBudget = Number(vs) > 0
  const cards: { label: string; value: string; tone?: string }[] = [
    { label: 'Money In', value: money(s.in), tone: 'var(--good)' },
    { label: 'Money Out', value: money(s.out) },
    { label: 'vs Budget', value: money(vs), tone: overBudget ? 'var(--danger)' : 'var(--good)' },
    { label: 'Vices', value: money(s.vices), tone: 'var(--danger)' },
  ]
  return (
    <div className="grid" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))' }}>
      {cards.map((c) => (
        <div key={c.label} className="card">
          <div style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>{c.label}</div>
          <div style={{ fontSize: '1.6rem', fontWeight: 700, color: c.tone ?? 'var(--text)' }}>{c.value}</div>
        </div>
      ))}
    </div>
  )
}
