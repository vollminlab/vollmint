import { useEffect, useState } from 'react'
import type { View, Insight } from '../api'
import { getInsights } from '../api'

const ICONS: Record<string, string> = {
  category_spike: '📈',
  budget_breach: '⚠️',
  subscription_total: '💳',
  price_increase: '⬆️',
  subscription_overlap: '🔁',
}

export function InsightCards({ view, month }: { view: View; month: string }) {
  const [items, setItems] = useState<Insight[]>([])

  useEffect(() => {
    let live = true
    getInsights(view, month)
      .then((d) => live && setItems(d.insights ?? []))
      .catch(() => {})
    return () => {
      live = false
    }
  }, [view, month])

  if (items.length === 0) return null

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
        gap: '1rem',
        marginTop: '1.5rem',
      }}
    >
      {items.map((in_, i) => (
        <div
          key={i}
          style={{
            border: '1px solid var(--border, rgba(128,128,128,0.3))',
            borderRadius: '8px',
            padding: '0.9rem',
          }}
        >
          <span style={{ fontSize: '1.4em' }}>{ICONS[in_.type] ?? '💡'}</span>
          <h3 style={{ margin: '0.4rem 0' }}>{in_.title}</h3>
          <p style={{ color: 'var(--muted)', margin: 0 }}>{in_.body}</p>
        </div>
      ))}
    </div>
  )
}
