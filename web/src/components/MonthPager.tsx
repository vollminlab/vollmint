import { monthLabel, shiftMonth } from '../format'

export function MonthPager({
  month,
  onChange,
}: {
  month: string
  onChange: (m: string) => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
      <button aria-label="previous month" onClick={() => onChange(shiftMonth(month, -1))}>
        ‹
      </button>
      <strong>{monthLabel(month)}</strong>
      <button aria-label="next month" onClick={() => onChange(shiftMonth(month, 1))}>
        ›
      </button>
    </div>
  )
}
