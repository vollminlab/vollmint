import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import type { View, Recurring as RecurringItem } from '../api'
import { getRecurring } from '../api'
import { money } from '../format'

// Recurring charges detected across all history (>=3 distinct months per
// payee). Each payee deep-links into Transactions pre-filtered by q, so no
// number is a dead end.
export function Recurring({ view, month }: { view: View; month: string }) {
  const [rows, setRows] = useState<RecurringItem[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let live = true
    setLoading(true)
    getRecurring(view, month)
      .then((d) => { if (live) setRows(d.recurring) })
      .catch((e) => { if (live) setErr(e.message) })
      .finally(() => { if (live) setLoading(false) })
    return () => { live = false }
  }, [view, month])

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>
  if (loading) return <p style={{ color: 'var(--muted)' }}>Loading…</p>

  return (
    <table>
      <thead>
        <tr>
          <th>Payee</th>
          <th style={{ textAlign: 'right' }}>Avg amount</th>
          <th style={{ textAlign: 'right' }}>Charges</th>
          <th style={{ textAlign: 'right' }}>Months</th>
          <th>First seen</th>
          <th>Last seen</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.payee}>
            <td>
              <Link
                to={{
                  pathname: '/transactions',
                  search: `?view=${view}&month=${month}&q=${encodeURIComponent(r.payee)}`,
                }}
              >
                {r.payee}
              </Link>
              {r.is_new && (
                <span
                  style={{ marginLeft: '0.5rem', color: 'var(--accent)', fontSize: '0.8rem', fontWeight: 700 }}
                >
                  NEW
                </span>
              )}
            </td>
            <td style={{ textAlign: 'right' }}>{money(r.avg_amount)}</td>
            <td style={{ textAlign: 'right' }}>{r.count}</td>
            <td style={{ textAlign: 'right' }}>{r.months}</td>
            <td>{r.first_seen}</td>
            <td>{r.last_seen}</td>
          </tr>
        ))}
        {rows.length === 0 && (
          <tr>
            <td colSpan={6} style={{ color: 'var(--muted)' }}>
              No recurring charges detected yet.
            </td>
          </tr>
        )}
      </tbody>
    </table>
  )
}
