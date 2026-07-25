import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { Budgets } from './Budgets'

const cats = { categories: [
  { id: 2, name: 'Groceries', parent_id: null, kind: 'spend', is_vice: false },
  { id: 3, name: 'Dining', parent_id: null, kind: 'spend', is_vice: true },
] }
const budgets = { budgets: [{ category_id: 2, category_name: 'Groceries', amount: '120.00' }] }

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string) => {
    const body = (url as string).startsWith('/api/categories') ? cats : budgets
    return Promise.resolve({ ok: true, json: async () => body })
  }))
})

describe('Budgets', () => {
  it('lists categories with their current budget value', async () => {
    render(<Budgets month="2026-07" />)
    await waitFor(() => expect(screen.getByText('Groceries')).toBeInTheDocument())
    const grocInput = screen.getByLabelText('budget for Groceries') as HTMLInputElement
    expect(grocInput.value).toBe('120.00')
    const diningInput = screen.getByLabelText('budget for Dining') as HTMLInputElement
    expect(diningInput.value).toBe('')
  })
})
