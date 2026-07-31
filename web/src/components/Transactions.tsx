import { Fragment, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { View, Txn, Category } from '../api'
import { getTransactions, getCategories, patchTransaction, deleteSplits } from '../api'
import { money } from '../format'
import { SplitEditor } from './SplitEditor'

export function Transactions({ view, month }: { view: View; month: string }) {
  const [params] = useSearchParams()
  const categoryParam = params.get('category')
  const categoryId = categoryParam ? Number(categoryParam) : undefined
  const q = params.get('q') || undefined

  const [rows, setRows] = useState<Txn[]>([])
  const [cats, setCats] = useState<Category[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<number | null>(null)

  const load = () => {
    setLoading(true)
    getTransactions({ view, month, category: categoryId, q })
      .then((d) => setRows(d.transactions))
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, month, categoryParam, q])

  useEffect(() => {
    getCategories().then((d) => setCats(d.categories)).catch(() => {})
  }, [])

  const recategorize = async (id: number, catId: number) => {
    try {
      await patchTransaction(id, { category_id: catId })
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      load()
    }
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
      {q !== undefined && (
        <p style={{ color: 'var(--muted)' }}>
          Filtered by search: <strong>{q}</strong>
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
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((t) => {
              const canSplit = !t.pending && t.transfer_peer_id === null
              const isSplit = t.splits.length > 0
              return (
                <Fragment key={t.id}>
                  <tr>
                    <td>{t.posted}</td>
                    <td>{t.payee || t.description}</td>
                    <td>{t.account_name}</td>
                    <td
                      style={{ textAlign: 'right', color: t.amount.startsWith('-') ? 'var(--text)' : 'var(--good)' }}
                    >
                      {money(t.amount)}
                    </td>
                    <td>
                      {isSplit ? (
                        <div>
                          <span>
                            {t.splits.length === 2
                              ? 'Split · ' + t.splits.map((sp) => sp.category).join(' + ')
                              : `Split (${t.splits.length})`}
                          </span>
                          {t.splits.map((sp) => (
                            <div key={sp.id} style={{ color: 'var(--muted)', fontSize: '0.85em' }}>
                              {sp.category}: {money(sp.amount)}
                              {sp.note ? ` — ${sp.note}` : ''}
                            </div>
                          ))}
                        </div>
                      ) : (
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
                      )}
                    </td>
                    <td>
                      {canSplit && (
                        <button type="button" onClick={() => setEditing(editing === t.id ? null : t.id)}>
                          Split
                        </button>
                      )}
                      {isSplit && (
                        <button
                          type="button"
                          onClick={async () => {
                            try {
                              await deleteSplits(t.id)
                            } catch (e) {
                              setErr((e as Error).message)
                            } finally {
                              load()
                            }
                          }}
                        >
                          Unsplit
                        </button>
                      )}
                    </td>
                  </tr>
                  {editing === t.id && (
                    <tr>
                      <td colSpan={6}>
                        <SplitEditor
                          txn={t}
                          cats={cats}
                          onSaved={() => {
                            setEditing(null)
                            load()
                          }}
                          onCancel={() => setEditing(null)}
                        />
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
            {rows.length === 0 && (
              <tr>
                <td colSpan={6} style={{ color: 'var(--muted)' }}>
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
