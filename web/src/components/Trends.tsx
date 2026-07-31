import { useEffect, useState } from 'react'
import { Bar, BarChart, CartesianGrid, Legend, Tooltip, XAxis, YAxis } from 'recharts'
import type { View, TrendPoint } from '../api'
import { getTrends } from '../api'
import { money } from '../format'
import { InsightCards } from './InsightCards'

// Fixed-size chart: ResponsiveContainer renders nothing under jsdom (zero
// width), and 1000px fits the app's 1100px container.
export function Trends({ view, month }: { view: View; month: string }) {
  const [months, setMonths] = useState(12)
  const [rows, setRows] = useState<TrendPoint[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let live = true
    setLoading(true)
    getTrends(view, month, months)
      .then((d) => { if (live) setRows(d.trends) })
      .catch((e) => { if (live) setErr(e.message) })
      .finally(() => { if (live) setLoading(false) })
    return () => { live = false }
  }, [view, month, months])

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>

  // Number() here is chart geometry only — display always goes through money().
  const data = rows.map((p) => ({ month: p.month, income: Number(p.in), spending: Number(p.out) }))

  return (
    <div>
      <p>
        <label style={{ color: 'var(--muted)' }}>
          Window:{' '}
          <select
            aria-label="months window"
            value={months}
            onChange={(e) => setMonths(Number(e.target.value))}
          >
            <option value={6}>6 months</option>
            <option value={12}>12 months</option>
            <option value={24}>24 months</option>
          </select>
        </label>
      </p>
      {loading ? (
        <p style={{ color: 'var(--muted)' }}>Loading…</p>
      ) : (
        <BarChart width={1000} height={360} data={data}>
          <CartesianGrid stroke="#262a33" />
          <XAxis dataKey="month" stroke="var(--muted)" />
          <YAxis stroke="var(--muted)" />
          <Tooltip
            formatter={(v) => money(Number(v).toFixed(2))}
            contentStyle={{ background: 'var(--panel)', border: '1px solid #262a33' }}
          />
          <Legend />
          <Bar dataKey="spending" name="Spending" fill="var(--danger)" />
          <Bar dataKey="income" name="Income" fill="var(--good)" />
        </BarChart>
      )}
      <InsightCards view={view} month={month} />
    </div>
  )
}
