import { useEffect, useState } from 'react'
import { CartesianGrid, Line, LineChart, Tooltip, XAxis, YAxis } from 'recharts'
import type { NetWorthAccount, NetWorthRange, NetWorthResponse, View } from '../api'
import { createManualAccount, getNetWorth, updateManualBalance } from '../api'
import { money, titleCase } from '../format'

const RANGES: NetWorthRange[] = ['1m', '3m', '6m', '1y', 'all']
const RANGE_LABELS: Record<NetWorthRange, string> = {
  '1m': '1M', '3m': '3M', '6m': '6M', '1y': '1Y', all: 'All',
}

// Manual balances older than this many days get a nudge to update.
const STALE_DAYS = 60

function isStale(a: NetWorthAccount): boolean {
  if (!a.is_manual || !a.balance_date) return false
  const age = (Date.now() - new Date(a.balance_date + 'T00:00:00').getTime()) / 86400000
  return age > STALE_DAYS
}

// Fixed-size chart: ResponsiveContainer renders nothing under jsdom (zero
// width), and 1000px fits the app's 1100px container.
export function NetWorth({ view }: { view: View }) {
  const [range, setRange] = useState<NetWorthRange>('3m')
  const [data, setData] = useState<NetWorthResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<string | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  // inline manual-balance edit
  const [editId, setEditId] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')

  // add-account form
  const [newName, setNewName] = useState('')
  const [newOwner, setNewOwner] = useState('scott')
  const [newBalance, setNewBalance] = useState('')
  const [formErr, setFormErr] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    setLoading(true)
    getNetWorth(view, range)
      .then((d) => { if (live) { setData(d); setErr(null) } })
      .catch((e) => { if (live) setErr(e.message) })
      .finally(() => { if (live) setLoading(false) })
    return () => { live = false }
  }, [view, range, reloadKey])

  // Drill-down and inline edit are per-view; a stale account id from another
  // view would blank the chart. Reset them when the view switches.
  useEffect(() => {
    setSelected(null)
    setEditId(null)
  }, [view])

  if (err) return <p style={{ color: 'var(--danger)' }}>Error: {err}</p>

  const accounts = data?.accounts ?? []
  const series = data?.series ?? []
  const selectedAccount = accounts.find((a) => a.id === selected)

  // Number() below is display totals / chart geometry only — money strings
  // stay strings everywhere else and render through money().
  const assets = accounts.reduce((sum, a) => {
    const n = Number(a.balance)
    return sum + (n > 0 ? n : 0)
  }, 0)
  const liabilities = accounts.reduce((sum, a) => {
    const n = Number(a.balance)
    return sum + (n < 0 ? n : 0)
  }, 0)
  const chartData = series.map((p) => ({
    date: p.date,
    value: selected
      ? p.accounts[selected] !== undefined ? Number(p.accounts[selected]) : null
      : Number(p.total),
  }))

  const saveEdit = async (id: string) => {
    try {
      setFormErr(null)
      await updateManualBalance(id, editValue)
      setEditId(null)
      setReloadKey((k) => k + 1)
    } catch (e) {
      setFormErr((e as Error).message)
    }
  }

  const addAccount = async () => {
    try {
      setFormErr(null)
      await createManualAccount({ name: newName, owner: newOwner, balance: newBalance })
      setNewName('')
      setNewBalance('')
      setReloadKey((k) => k + 1)
    } catch (e) {
      setFormErr((e as Error).message)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', gap: '2rem', marginBottom: '1rem' }}>
        <div>
          <div style={{ color: 'var(--muted)' }}>Assets</div>
          <div style={{ fontSize: '1.2rem' }}>{money(assets.toFixed(2))}</div>
        </div>
        <div>
          <div style={{ color: 'var(--muted)' }}>Liabilities</div>
          <div style={{ fontSize: '1.2rem' }}>{money(liabilities.toFixed(2))}</div>
        </div>
        <div>
          <div style={{ color: 'var(--muted)' }}>Net worth</div>
          <div style={{ fontSize: '1.2rem', fontWeight: 700 }}>{money((assets + liabilities).toFixed(2))}</div>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '0.5rem' }}>
        <h2 style={{ margin: 0 }}>{selectedAccount ? selectedAccount.name : 'Net Worth'}</h2>
        {selected && (
          <button onClick={() => setSelected(null)}>All accounts</button>
        )}
        <div style={{ display: 'flex', gap: '0.4rem', marginLeft: 'auto' }}>
          {RANGES.map((r) => (
            <button key={r} onClick={() => setRange(r)} aria-pressed={r === range}>
              {RANGE_LABELS[r]}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <p style={{ color: 'var(--muted)' }}>Loading…</p>
      ) : series.length === 0 ? (
        <p style={{ color: 'var(--muted)' }}>
          No balance history yet — snapshots start accruing with the next sync.
        </p>
      ) : (
        <LineChart width={1000} height={360} data={chartData}>
          <CartesianGrid stroke="#262a33" />
          <XAxis dataKey="date" stroke="var(--muted)" />
          <YAxis stroke="var(--muted)" domain={['auto', 'auto']} />
          <Tooltip
            formatter={(v) => money(Number(v).toFixed(2))}
            contentStyle={{ background: 'var(--panel)', border: '1px solid #262a33' }}
          />
          <Line
            type="monotone"
            dataKey="value"
            name={selectedAccount ? selectedAccount.name : 'Net Worth'}
            stroke="var(--accent)"
            dot={false}
          />
        </LineChart>
      )}

      {formErr && <p style={{ color: 'var(--danger)' }}>Error: {formErr}</p>}

      <table style={{ width: '100%', marginTop: '1rem' }}>
        <thead>
          <tr>
            <th>Name</th>
            <th>Owner</th>
            <th style={{ textAlign: 'right' }}>Balance</th>
            <th>Date</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((a) => (
            <tr key={a.id}>
              <td>
                <button onClick={() => setSelected(a.id)} style={{ fontWeight: a.id === selected ? 700 : 400 }}>
                  {a.name}
                </button>
              </td>
              <td style={{ color: 'var(--muted)' }}>{titleCase(a.owner)}</td>
              <td style={{ textAlign: 'right' }}>{a.balance ? money(a.balance) : '—'}</td>
              <td style={{ color: 'var(--muted)' }}>
                {a.balance_date}
                {isStale(a) && <span style={{ marginLeft: '0.4rem', color: 'var(--muted)' }}>update?</span>}
              </td>
              <td>
                {a.is_manual && editId !== a.id && (
                  <button onClick={() => { setEditId(a.id); setEditValue(a.balance) }}>Edit</button>
                )}
                {a.is_manual && editId === a.id && (
                  <span style={{ display: 'inline-flex', gap: '0.3rem' }}>
                    <input
                      aria-label="new balance"
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      style={{ width: '8rem' }}
                    />
                    <button onClick={() => saveEdit(a.id)}>Save</button>
                    <button onClick={() => setEditId(null)}>Cancel</button>
                  </span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <form
        onSubmit={(e) => { e.preventDefault(); addAccount() }}
        style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}
      >
        <input
          aria-label="new account name"
          placeholder="Name (e.g. 401k)"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
        />
        <select aria-label="new account owner" value={newOwner} onChange={(e) => setNewOwner(e.target.value)}>
          <option value="scott">Scott</option>
          <option value="nikki">Nikki</option>
          <option value="joint">Joint</option>
        </select>
        <input
          aria-label="new account balance"
          placeholder="Balance (negative = liability)"
          value={newBalance}
          onChange={(e) => setNewBalance(e.target.value)}
        />
        <button type="submit">Add account</button>
      </form>
    </div>
  )
}
