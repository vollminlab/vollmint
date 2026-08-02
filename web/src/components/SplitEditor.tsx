import { useState } from 'react'
import type { Txn, Category, SplitInput } from '../api'
import { putSplits } from '../api'

const amountRe = /^\d+(\.\d{1,2})?$/

/** "32.5" → 3250; null when not a valid positive dollar amount. */
export function toCents(v: string): number | null {
  const s = v.trim()
  if (!amountRe.test(s)) return null
  const [whole, frac = ''] = s.split('.')
  return Number(whole) * 100 + Number(frac.padEnd(2, '0') || '0')
}

export function fromCents(c: number): string {
  const sign = c < 0 ? '-' : ''
  const abs = Math.abs(c)
  return `${sign}${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, '0')}`
}

interface Draft {
  category_id: number | ''
  amount: string
  note: string
}

export function SplitEditor({
  txn,
  cats,
  onSaved,
  onCancel,
}: {
  txn: Txn
  cats: Category[]
  onSaved: () => void
  onCancel: () => void
}) {
  const negative = txn.amount.startsWith('-')
  const parentCents = Math.abs(toCents(txn.amount.replace('-', '')) ?? 0)

  const seed: Draft[] =
    txn.splits.length > 0
      ? txn.splits.map((sp) => ({
          category_id: sp.category_id,
          amount: sp.amount.replace('-', ''),
          note: sp.note,
        }))
      : [
          { category_id: txn.category_id ?? '', amount: txn.amount.replace('-', ''), note: '' },
          { category_id: '', amount: '', note: '' },
        ]

  const [drafts, setDrafts] = useState<Draft[]>(seed)
  const [err, setErr] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const partCents = drafts.map((d) => toCents(d.amount))
  const validSum = partCents.reduce<number>((acc, c) => acc + (c ?? 0), 0)
  const remainder = parentCents - validSum
  const allValid = drafts.every((d, i) => d.category_id !== '' && partCents[i] !== null && partCents[i]! > 0)
  const canSave = remainder === 0 && allValid && !saving
  const remainderColor = remainder !== 0 ? 'var(--danger)' : allValid ? 'var(--good)' : 'var(--muted)'

  const update = (i: number, patch: Partial<Draft>) =>
    setDrafts(drafts.map((d, j) => (j === i ? { ...d, ...patch } : d)))

  const save = async () => {
    setSaving(true)
    setErr(null)
    const splits: SplitInput[] = drafts.map((d) => ({
      category_id: d.category_id as number,
      amount: (negative ? '-' : '') + fromCents(toCents(d.amount)!),
      note: d.note,
    }))
    try {
      await putSplits(txn.id, splits)
      onSaved()
    } catch (e) {
      setErr((e as Error).message)
      setSaving(false)
    }
  }

  return (
    <div style={{ padding: '0.75rem', background: 'var(--panel, rgba(128,128,128,0.08))' }}>
      {drafts.map((d, i) => (
        <div key={i} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.4rem', alignItems: 'center' }}>
          <select
            value={d.category_id}
            onChange={(e) => update(i, { category_id: e.target.value ? Number(e.target.value) : '' })}
            aria-label={`category for part ${i + 1}`}
          >
            <option value="">— category —</option>
            {cats.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
          <input
            value={d.amount}
            onChange={(e) => update(i, { amount: e.target.value })}
            placeholder="0.00"
            aria-label={`amount for part ${i + 1}`}
            style={{ width: '6rem', textAlign: 'right' }}
          />
          <input
            value={d.note}
            onChange={(e) => update(i, { note: e.target.value })}
            placeholder="note (optional)"
            aria-label={`note for part ${i + 1}`}
          />
          {drafts.length > 2 && (
            <button type="button" onClick={() => setDrafts(drafts.filter((_, j) => j !== i))}>
              ✕
            </button>
          )}
        </div>
      ))}
      <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
        <button type="button" onClick={() => setDrafts([...drafts, { category_id: '', amount: '', note: '' }])}>
          + Add part
        </button>
        <span style={{ color: remainderColor }}>
          Remaining: {fromCents(remainder).startsWith('-') ? '-$' + fromCents(remainder).slice(1) : '$' + fromCents(remainder)}
        </span>
        {remainder === 0 && !allValid && (
          <span style={{ color: 'var(--muted)' }}>assign a category to every part</span>
        )}
        <button type="button" onClick={save} disabled={!canSave}>
          Save
        </button>
        <button type="button" onClick={onCancel}>
          Cancel
        </button>
      </div>
      {err && <p style={{ color: 'var(--danger)' }}>Error: {err}</p>}
    </div>
  )
}
