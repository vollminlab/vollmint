import { NavLink } from 'react-router-dom'

// Nav preserves the current query string (view+month) across page links so the
// shared state survives navigation.
export function Nav({ search }: { search: string }) {
  const link = (to: string, label: string) => (
    <NavLink
      to={{ pathname: to, search }}
      style={({ isActive }) => ({
        padding: '0.4rem 0.8rem',
        fontWeight: isActive ? 700 : 400,
        color: isActive ? 'var(--accent)' : 'var(--text)',
      })}
      end
    >
      {label}
    </NavLink>
  )
  return (
    <nav style={{ display: 'flex', gap: '0.5rem', borderBottom: '1px solid #262a33', marginBottom: '1rem' }}>
      {link('/', 'Dashboard')}
      {link('/transactions', 'Transactions')}
      {link('/budgets', 'Budgets')}
    </nav>
  )
}
