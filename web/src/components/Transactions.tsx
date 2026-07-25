import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { View, Txn, Category } from '../api'
import { getTransactions, getCategories, patchTransaction } from '../api'
import { money } from '../format'

export function Transactions({ view, month }: { view: View; month: string }) {
  const [params] = useSearchParams()
  const categoryParam = params.get('category')
  const categoryId = categoryParam ? Number(categoryParam) : undefined

  const [rows, setRows] = useState<Txn[]>([])
  const [cats, setCats] = useState<Category[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    getTransactions({ view, month, category: categoryId })
      .then((d) => setRows(d.transactions))
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, month, categoryParam])

  useEffect(() => {
    getCategories().then((d) => setCats(d.categories)).catch(() => {})
  }, [])

  const recategorize = async (id: number, catId: number) => {
    await patchTransaction(id, { category_id: catId })
    load()
  }

  const activeCat = cats.find((c) => c.id === categoryId)

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>

  return (
    <div>
      {categoryId !== undefined && (
        <p style={{ color: 'var(--muted)' }}>
          Filtered by category: <strong>{activeCat ? activeCat.name : categoryId}</strong>
        </p>
      )}
      {loading ? (
        <p style={{ color: 'var(--muted)' }}>Loading…</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Date</th>
              <th>Payee</th>
              <th>Account</th>
              <th style={{ textAlign: 'right' }}>Amount</th>
              <th>Category</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((t) => (
              <tr key={t.id}>
                <td>{t.posted}</td>
                <td>{t.payee || t.description}</td>
                <td>{t.account_name}</td>
                <td style={{ textAlign: 'right', color: t.amount.startsWith('-') ? 'var(--text)' : 'var(--good)' }}>
                  {money(t.amount)}
                </td>
                <td>
                  <select
                    value={t.category_id ?? ''}
                    onChange={(e) => recategorize(t.id, Number(e.target.value))}
                    aria-label={`category for ${t.payee || t.description}`}
                  >
                    <option value="" disabled>
                      — uncategorized —
                    </option>
                    {cats.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </td>
              </tr>
            ))}
            {rows.length === 0 && (
              <tr>
                <td colSpan={5} style={{ color: 'var(--muted)' }}>
                  No transactions.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}
