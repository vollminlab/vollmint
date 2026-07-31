import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { SplitEditor, toCents, fromCents } from './SplitEditor'
import type { Txn, Category } from '../api'
import * as api from '../api'

vi.mock('../api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../api')>()
  return { ...mod, putSplits: vi.fn() }
})

const cats: Category[] = [
  { id: 1, name: 'Dining', parent_id: null, kind: 'spend', is_vice: true },
  { id: 2, name: 'Groceries', parent_id: null, kind: 'spend', is_vice: false },
]

const txn: Txn = {
  id: 7,
  source: 'simplefin',
  account_id: 'ally-s',
  account_name: 'Checking',
  posted: '2026-07-10',
  amount: '-50.00',
  description: 'VENMO PAYMENT',
  payee: 'VENMO',
  pending: false,
  category_id: 1,
  category_name: 'Dining',
  owner_override: null,
  effective_owner: 'scott',
  transfer_peer_id: null,
  splits: [],
}

describe('toCents', () => {
  it('parses dollars and cents', () => {
    expect(toCents('32.50')).toBe(3250)
    expect(toCents('50')).toBe(5000)
    expect(toCents('0.05')).toBe(5)
  })
  it('rejects garbage', () => {
    expect(toCents('')).toBeNull()
    expect(toCents('abc')).toBeNull()
    expect(toCents('1.234')).toBeNull()
    expect(toCents('-5')).toBeNull()
  })
})

describe('fromCents', () => {
  it('renders cents as dollars', () => {
    expect(fromCents(3250)).toBe('32.50')
    expect(fromCents(5)).toBe('0.05')
  })
})

describe('SplitEditor', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the remainder and disables Save until it hits zero', async () => {
    render(<SplitEditor txn={txn} cats={cats} onSaved={() => {}} onCancel={() => {}} />)
    // Seeded: row 1 = parent category + full 50.00, row 2 empty → remainder 0
    // until we change row 1.
    const amounts = screen.getAllByLabelText(/amount for part/)
    fireEvent.change(amounts[0], { target: { value: '30.00' } })
    expect(screen.getByText(/remaining: \$20\.00/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled()

    fireEvent.change(amounts[1], { target: { value: '20.00' } })
    const catSelects = screen.getAllByLabelText(/category for part/)
    fireEvent.change(catSelects[1], { target: { value: '2' } })
    expect(screen.getByRole('button', { name: /save/i })).toBeEnabled()
  })

  it('applies the parent sign on save', async () => {
    const onSaved = vi.fn()
    vi.mocked(api.putSplits).mockResolvedValue({ transaction: txn })
    render(<SplitEditor txn={txn} cats={cats} onSaved={onSaved} onCancel={() => {}} />)

    const amounts = screen.getAllByLabelText(/amount for part/)
    fireEvent.change(amounts[0], { target: { value: '30.00' } })
    fireEvent.change(amounts[1], { target: { value: '20.00' } })
    const catSelects = screen.getAllByLabelText(/category for part/)
    fireEvent.change(catSelects[1], { target: { value: '2' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    expect(api.putSplits).toHaveBeenCalledWith(7, [
      { category_id: 1, amount: '-30.00', note: '' },
      { category_id: 2, amount: '-20.00', note: '' },
    ])
  })

  it('pre-fills draft rows from an already-split txn and enables Save immediately', () => {
    const splitTxn: Txn = {
      ...txn,
      splits: [
        { id: 1, category_id: 1, category: 'Dining', amount: '-30.00', note: '' },
        { id: 2, category_id: 2, category: 'Groceries', amount: '-20.00', note: 'tickets' },
      ],
    }
    render(<SplitEditor txn={splitTxn} cats={cats} onSaved={() => {}} onCancel={() => {}} />)

    const catSelects = screen.getAllByLabelText(/category for part/) as HTMLSelectElement[]
    const amounts = screen.getAllByLabelText(/amount for part/) as HTMLInputElement[]
    const notes = screen.getAllByLabelText(/note for part/) as HTMLInputElement[]

    expect(catSelects[0].value).toBe('1')
    expect(amounts[0].value).toBe('30.00')
    expect(notes[0].value).toBe('')

    expect(catSelects[1].value).toBe('2')
    expect(amounts[1].value).toBe('20.00')
    expect(notes[1].value).toBe('tickets')

    // Sum already matches the parent amount and every part has a category,
    // so Save should be enabled without any further edits.
    expect(screen.getByText(/remaining: \$0\.00/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /save/i })).toBeEnabled()
    expect(screen.queryByText(/assign a category to every part/i)).not.toBeInTheDocument()
  })

  it('shows a hint instead of the good color when remainder is zero but a category is missing', () => {
    render(<SplitEditor txn={txn} cats={cats} onSaved={() => {}} onCancel={() => {}} />)
    const amounts = screen.getAllByLabelText(/amount for part/)
    // Fill both amounts so remainder hits zero, but leave row 2's category unset.
    fireEvent.change(amounts[0], { target: { value: '30.00' } })
    fireEvent.change(amounts[1], { target: { value: '20.00' } })

    expect(screen.getByText(/remaining: \$0\.00/i)).toBeInTheDocument()
    expect(screen.getByText(/assign a category to every part/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled()
  })
})
