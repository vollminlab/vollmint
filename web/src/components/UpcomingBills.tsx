import { useEffect, useState } from 'react'
import type { View, Forecast } from '../api'
import { getForecast } from '../api'
import { money, titleCase, monthDayLabel, dateLabel } from '../format'

export function UpcomingBills({ view, month }: { view: View; month: string }) {
  const [forecast, setForecast] = useState<Forecast | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    setForecast(null)
    setErr(null)
    getForecast(view, month)
      .then((d) => live && setForecast(d.forecast))
      .catch((e) => live && setErr(e.message))
    return () => {
      live = false
    }
  }, [view, month])

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>
  if (!forecast) return <p style={{ color: 'var(--muted)' }}>Loading…</p>

  return (
    <section>
      <h2 style={{ fontSize: '1rem' }}>
        Upcoming bills — {money(forecast.remaining_expected)} remaining expected
      </h2>
      {forecast.bills.length === 0 ? (
        <p style={{ color: 'var(--muted)' }}>No recurring bills detected yet.</p>
      ) : (
        <table>
          <tbody>
            {forecast.bills.map((b) => (
              <tr key={b.payee} style={b.paid ? { opacity: 0.55 } : undefined}>
                <td>{titleCase(b.payee)}</td>
                <td style={{ color: 'var(--muted)' }}>{b.category}</td>
                <td>{b.paid ? `${dateLabel(b.paid_date)} ✓` : `~${monthDayLabel(month, b.predicted_day)}`}</td>
                <td style={{ textAlign: 'right' }}>{money(b.expected_amount)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
