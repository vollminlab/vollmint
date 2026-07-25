import type { View } from '../api'

const VIEWS: View[] = ['scott', 'nikki', 'joint', 'household']
const LABELS: Record<View, string> = {
  scott: 'Scott',
  nikki: 'Nikki',
  joint: 'Joint',
  household: 'Household',
}

export function ViewSwitcher({
  view,
  onChange,
}: {
  view: View
  onChange: (v: View) => void
}) {
  return (
    <div style={{ display: 'flex', gap: '0.4rem' }}>
      {VIEWS.map((v) => (
        <button
          key={v}
          onClick={() => onChange(v)}
          aria-pressed={v === view}
          style={{
            padding: '0.3rem 0.7rem',
            borderRadius: '999px',
            border: '1px solid #2c313b',
            background: v === view ? 'var(--accent)' : 'transparent',
            color: v === view ? '#04121f' : 'var(--text)',
          }}
        >
          {LABELS[v]}
        </button>
      ))}
    </div>
  )
}
