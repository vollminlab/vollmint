import { useEffect, useState } from 'react'
import type { Category } from '../api'
import { getCategories, getBudgets, putBudgets } from '../api'

export function Budgets({ month }: { month: string }) {
  const [cats, setCats] = useState<Category[]>([])
  const [amounts, setAmounts] = useState<Record<number, string>>({})
  const [status, setStatus] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([getCategories(), getBudgets(month)])
      .then(([c, b]) => {
        setCats(c.categories)
        const m: Record<number, string> = {}
        for (const item of b.budgets) m[item.category_id] = item.amount
        setAmounts(m)
      })
      .catch((e) => setStatus(e.message))
  }, [month])

  const save = async () => {
    const items = Object.entries(amounts)
      .filter(([, v]) => v.trim() !== '')
      .map(([id, amount]) => ({ category_id: Number(id), amount: amount.trim() }))
    try {
      await putBudgets(month, items)
      setStatus('Saved.')
    } catch (e) {
      setStatus((e as Error).message)
    }
  }

  // Only spend/savings categories get budgets (income/transfer don't).
  const budgetable = cats.filter((c) => c.kind === 'spend' || c.kind === 'savings')

  return (
    <div className="grid" style={{ gap: '1rem', maxWidth: 480 }}>
      <h2 style={{ fontSize: '1rem', margin: 0 }}>Monthly budgets</h2>
      {budgetable.map((c) => (
        <div key={c.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <label htmlFor={`budget-${c.id}`}>{c.name}</label>
          <input
            id={`budget-${c.id}`}
            aria-label={`budget for ${c.name}`}
            inputMode="decimal"
            placeholder="0.00"
            value={amounts[c.id] ?? ''}
            onChange={(e) => setAmounts((a) => ({ ...a, [c.id]: e.target.value }))}
            style={{ width: 120, padding: '0.3rem', textAlign: 'right' }}
          />
        </div>
      ))}
      <button onClick={save} style={{ padding: '0.5rem', background: 'var(--accent)', border: 'none', borderRadius: 6 }}>
        Save budgets
      </button>
      {status && <p style={{ color: 'var(--muted)' }}>{status}</p>}
    </div>
  )
}
