import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { Rules } from './Rules'

const rules = {
  rules: [
    { id: 1, priority: 500, match_type: 'substring', pattern: 'netflix', category_id: 6 },
    { id: 2, priority: 1000, match_type: 'substring', pattern: 'VENMO', category_id: 15 },
  ],
}
const cats = {
  categories: [
    { id: 6, name: 'Subscriptions', parent_id: null, kind: 'spend', is_vice: false },
    { id: 15, name: 'Needs Venmo detail', parent_id: null, kind: 'spend', is_vice: false },
  ],
}

function stubFetch() {
  return vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === 'POST') {
      return Promise.resolve({ ok: true, json: async () => ({ id: 3, recategorized: 4 }) })
    }
    if (init?.method === 'DELETE') {
      return Promise.resolve({ ok: true, json: async () => ({ status: 'deleted' }) })
    }
    const body = url.startsWith('/api/categories') ? cats : rules
    return Promise.resolve({ ok: true, json: async () => body })
  })
}

beforeEach(() => {
  vi.stubGlobal('fetch', stubFetch())
})

describe('Rules', () => {
  it('lists rules with resolved category names', async () => {
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('netflix')).toBeInTheDocument())
    expect(screen.getByText('VENMO')).toBeInTheDocument()
    const table = screen.getByRole('table')
    expect(within(table).getByText('Subscriptions')).toBeInTheDocument()
    expect(within(table).getByText('Needs Venmo detail')).toBeInTheDocument()
  })

  it('creates a rule and reports how many transactions were recategorized', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('netflix')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('new rule pattern'), { target: { value: 'spotify' } })
    fireEvent.change(screen.getByLabelText('new rule category'), { target: { value: '6' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    await waitFor(() =>
      expect(screen.getByText('Rule added — 4 transactions recategorized.')).toBeInTheDocument(),
    )
    const post = fetchMock.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === 'POST')!
    expect(post[0]).toBe('/api/rules')
    const body = JSON.parse((post[1] as RequestInit).body as string)
    expect(body).toEqual({ priority: 500, match_type: 'substring', pattern: 'spotify', category_id: 6 })
  })

  it('requires pattern and category before submitting', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('netflix')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))
    await waitFor(() => expect(screen.getByText(/Pattern and category are required/i)).toBeInTheDocument())
    expect(fetchMock.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === 'POST')).toBe(false)
  })

  it('rejects a cleared or non-positive priority', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('netflix')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('new rule pattern'), { target: { value: 'spotify' } })
    fireEvent.change(screen.getByLabelText('new rule category'), { target: { value: '6' } })
    fireEvent.change(screen.getByLabelText('new rule priority'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))
    await waitFor(() => expect(screen.getByText(/Priority must be a positive integer/i)).toBeInTheDocument())
    expect(fetchMock.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === 'POST')).toBe(false)
  })

  it('deletes a rule', async () => {
    const fetchMock = stubFetch()
    vi.stubGlobal('fetch', fetchMock)
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('netflix')).toBeInTheDocument())
    fireEvent.click(screen.getByLabelText('delete rule netflix'))
    await waitFor(() => {
      const del = fetchMock.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === 'DELETE')
      expect(del?.[0]).toBe('/api/rules/1')
    })
  })

  it('surfaces API errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) }),
    )
    render(<Rules />)
    await waitFor(() => expect(screen.getByText('Error: boom')).toBeInTheDocument())
  })
})
