import { useEffect, useState } from 'react'
import type { View, SummaryResponse } from '../api'
import { getSummary } from '../api'
import { SummaryCards } from './SummaryCards'
import { CategoryBars } from './CategoryBars'

export function Dashboard({ view, month }: { view: View; month: string }) {
  const [data, setData] = useState<SummaryResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    setData(null)
    setErr(null)
    getSummary(view, month)
      .then((d) => live && setData(d))
      .catch((e) => live && setErr(e.message))
    return () => {
      live = false
    }
  }, [view, month])

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>
  if (!data) return <p style={{ color: 'var(--muted)' }}>Loading…</p>

  return (
    <div className="grid" style={{ gap: '1.5rem' }}>
      <SummaryCards s={data.summary} />
      <section>
        <h2 style={{ fontSize: '1rem' }}>Spending by category</h2>
        <CategoryBars categories={data.categories} view={view} month={month} />
      </section>
    </div>
  )
}
