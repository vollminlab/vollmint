import { useEffect, useState } from 'react'
import type { Category, Rule } from '../api'
import { createRule, deleteRule, getCategories, getRules } from '../api'

// Rules are applied first-match-wins, ordered by priority ASC then id ASC,
// case-insensitive substring against payee+description. Creating a rule
// re-runs the engine over all history server-side. There is no edit endpoint:
// to change a rule, delete it and add a replacement.
export function Rules() {
  const [rules, setRules] = useState<Rule[]>([])
  const [cats, setCats] = useState<Category[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [status, setStatus] = useState('')

  const [pattern, setPattern] = useState('')
  const [matchType, setMatchType] = useState('substring')
  const [priority, setPriority] = useState('500')
  const [categoryId, setCategoryId] = useState('')

  const load = () => {
    setLoading(true)
    Promise.all([getRules(), getCategories()])
      .then(([r, c]) => {
        setRules(r.rules)
        setCats(c.categories)
      })
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const catName = (id: number) => cats.find((c) => c.id === id)?.name ?? String(id)

  const add = async () => {
    if (!pattern || !categoryId) {
      setStatus('Pattern and category are required.')
      return
    }
    try {
      const res = await createRule({
        priority: Number(priority),
        match_type: matchType,
        pattern,
        category_id: Number(categoryId),
      })
      setStatus(`Rule added — ${res.recategorized} transactions recategorized.`)
      setPattern('')
      load()
    } catch (e) {
      setStatus((e as Error).message)
    }
  }

  const remove = async (id: number) => {
    try {
      await deleteRule(id)
      setStatus('Rule deleted.')
    } catch (e) {
      setStatus((e as Error).message)
    } finally {
      load()
    }
  }

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>

  return (
    <div>
      <div className="card" style={{ marginBottom: '1rem' }}>
        <p style={{ color: 'var(--muted)', marginTop: 0 }}>
          First match wins (lowest priority number first). Convention: 100 = money movement,
          400 = disambiguators, 500 = merchants, 1000 = fallback.
        </p>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            aria-label="new rule pattern"
            placeholder="pattern (e.g. spotify)"
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
          />
          <select aria-label="new rule match type" value={matchType} onChange={(e) => setMatchType(e.target.value)}>
            <option value="substring">substring</option>
            <option value="regex">regex</option>
          </select>
          <input
            aria-label="new rule priority"
            type="number"
            style={{ width: '5rem' }}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
          <select aria-label="new rule category" value={categoryId} onChange={(e) => setCategoryId(e.target.value)}>
            <option value="" disabled>
              — category —
            </option>
            {cats.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
          <button onClick={add}>Add rule</button>
        </div>
        {status && <p style={{ color: 'var(--muted)', marginBottom: 0 }}>{status}</p>}
      </div>
      {loading ? (
        <p style={{ color: 'var(--muted)' }}>Loading…</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th style={{ textAlign: 'right' }}>Priority</th>
              <th>Pattern</th>
              <th>Match</th>
              <th>Category</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rules.map((r) => (
              <tr key={r.id}>
                <td style={{ textAlign: 'right' }}>{r.priority}</td>
                <td>{r.pattern}</td>
                <td>{r.match_type}</td>
                <td>{catName(r.category_id)}</td>
                <td>
                  <button aria-label={`delete rule ${r.pattern}`} onClick={() => remove(r.id)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={5} style={{ color: 'var(--muted)' }}>
                  No rules.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}
