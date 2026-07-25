import { Link } from 'react-router-dom'
import type { CategorySpend, View } from '../api'
import { money } from '../format'

// Bar width is the ONLY place a monetary value becomes a float — pixel geometry
// only, never displayed or stored.
function pct(spent: string, max: number): number {
  if (max <= 0) return 0
  return Math.min(100, (Number(spent) / max) * 100)
}

export function CategoryBars({
  categories,
  view,
  month,
}: {
  categories: CategorySpend[]
  view: View
  month: string
}) {
  const max = categories.reduce((m, c) => Math.max(m, Number(c.spent)), 0)
  if (categories.length === 0) {
    return <p style={{ color: 'var(--muted)' }}>No spending this month.</p>
  }
  return (
    <div className="grid" style={{ gap: '0.5rem' }}>
      {categories.map((c) => {
        const over = c.budget !== '' && Number(c.spent) > Number(c.budget)
        const to = `/transactions?view=${view}&month=${month}&category=${c.category_id}`
        return (
          <Link key={c.category_id} to={to} className="card" style={{ display: 'block' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>
                {c.category}
                {c.is_vice ? ' 🔥' : ''}
              </span>
              <span style={{ color: over ? 'var(--danger)' : 'var(--text)' }}>
                {money(c.spent)}
                {c.budget !== '' ? ` / ${money(c.budget)}` : ''}
              </span>
            </div>
            <div style={{ height: 6, background: '#262a33', borderRadius: 3, marginTop: 6 }}>
              <div
                style={{
                  width: `${pct(c.spent, max)}%`,
                  height: '100%',
                  borderRadius: 3,
                  background: over ? 'var(--danger)' : 'var(--accent)',
                }}
              />
            </div>
          </Link>
        )
      })}
    </div>
  )
}
