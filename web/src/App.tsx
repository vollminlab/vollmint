import { Routes, Route, useSearchParams } from 'react-router-dom'
import type { View } from './api'
import { currentMonth } from './format'
import { Nav } from './components/Nav'
import { ViewSwitcher } from './components/ViewSwitcher'
import { MonthPager } from './components/MonthPager'
import { Dashboard } from './components/Dashboard'
import { Transactions } from './components/Transactions'
import { Budgets } from './components/Budgets'

const isView = (v: string): v is View =>
  v === 'scott' || v === 'nikki' || v === 'joint' || v === 'household'

export default function App() {
  const [params, setParams] = useSearchParams()
  const rawView = params.get('view') ?? 'household'
  const view: View = isView(rawView) ? rawView : 'household'
  const month = params.get('month') ?? currentMonth()

  const update = (next: { view?: View; month?: string }) => {
    const p = new URLSearchParams(params)
    if (next.view) p.set('view', next.view)
    if (next.month) p.set('month', next.month)
    setParams(p, { replace: true })
  }

  return (
    <div className="container">
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: '1rem',
          flexWrap: 'wrap',
          gap: '0.75rem',
        }}
      >
        <h1 style={{ margin: 0, fontSize: '1.3rem' }}>vollmint</h1>
        <MonthPager month={month} onChange={(m) => update({ month: m })} />
        <ViewSwitcher view={view} onChange={(v) => update({ view: v })} />
      </header>
      <Nav search={`?${params.toString()}`} />
      <Routes>
        <Route path="/" element={<Dashboard view={view} month={month} />} />
        <Route path="/transactions" element={<Transactions view={view} month={month} />} />
        <Route path="/budgets" element={<Budgets month={month} />} />
      </Routes>
    </div>
  )
}
