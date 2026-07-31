// Typed client for the vollmint API. Money fields are decimal strings — never
// coerce them to number except for chart geometry.

export type View = 'scott' | 'nikki' | 'joint' | 'household'

export interface Summary {
  in: string
  out: string
  vices: string
  budget_total: string
  month: string
  view: string
}

export interface CategorySpend {
  category_id: number
  category: string
  spent: string
  budget: string
  is_vice: boolean
}

export interface SummaryResponse {
  summary: Summary
  categories: CategorySpend[]
}

export interface Txn {
  id: number
  source: string
  account_id: string
  account_name: string
  posted: string
  amount: string
  description: string
  payee: string
  pending: boolean
  category_id: number | null
  category_name: string | null
  owner_override: string | null
  effective_owner: string
  transfer_peer_id: number | null
  splits: Split[]
}

export interface Split {
  id: number
  category_id: number
  category: string
  amount: string
  note: string
}

export interface SplitInput {
  category_id: number
  amount: string
  note: string
}

export interface ForecastBill {
  payee: string
  category_id: number | null
  category: string
  predicted_day: number
  expected_amount: string
  paid: boolean
  paid_date: string
  paid_amount: string
}

export interface Forecast {
  month: string
  view: string
  bills: ForecastBill[]
  remaining_expected: string
}

export interface Insight {
  type: string
  title: string
  body: string
  amount: string
}

export interface Category {
  id: number
  name: string
  parent_id: number | null
  kind: string
  is_vice: boolean
}

export interface Rule {
  id: number
  priority: number
  match_type: string
  pattern: string
  category_id: number
}

export interface Budget {
  category_id: number
  category_name: string
  amount: string
}

export interface Recurring {
  payee: string
  count: number
  months: number
  avg_amount: string
  last_seen: string
  first_seen: string
  is_new: boolean
}

export interface TrendPoint {
  month: string
  in: string
  out: string
}

export interface SyncRun {
  id: number
  kind: string
  started: string
  finished: string | null
  status: string
  rows_upserted: number
  detail: string
}

export type QueryParams = Record<string, string | number | boolean | undefined>

// buildQuery renders a query string, dropping undefined/empty values.
export function buildQuery(params: QueryParams): string {
  const parts: string[] = []
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || v === false) continue
    parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
  }
  return parts.length ? `?${parts.join('&')}` : ''
}

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, ...(init ? [init] : []))
  if (!res.ok) {
    let msg = `request failed: ${res.status}`
    try {
      const body = await res.json()
      if (body && body.error) msg = body.error
    } catch {
      // non-JSON error body; keep the status message
    }
    throw new Error(msg)
  }
  return res.json() as Promise<T>
}

function jsonInit(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }
}

export function getSummary(view: View, month: string): Promise<SummaryResponse> {
  return req<SummaryResponse>(`/api/summary${buildQuery({ view, month })}`)
}

export interface TxnFilter {
  view: View
  month?: string
  category?: number
  account?: string
  q?: string
  uncategorized?: boolean
}

export function getTransactions(f: TxnFilter): Promise<{ transactions: Txn[] }> {
  return req(`/api/transactions${buildQuery({ ...f })}`)
}

export function patchTransaction(
  id: number,
  patch: { category_id?: number; owner_override?: string },
): Promise<{ status: string }> {
  return req(`/api/transactions/${id}`, jsonInit('PATCH', patch))
}

export function getCategories(): Promise<{ categories: Category[] }> {
  return req('/api/categories')
}

export function getRules(): Promise<{ rules: Rule[] }> {
  return req('/api/rules')
}

export function createRule(rule: {
  priority: number
  match_type: string
  pattern: string
  category_id: number
}): Promise<{ id: number; recategorized: number }> {
  return req('/api/rules', jsonInit('POST', rule))
}

export function deleteRule(id: number): Promise<{ status: string }> {
  return req(`/api/rules/${id}`, { method: 'DELETE' })
}

export function getBudgets(month: string): Promise<{ budgets: Budget[] }> {
  return req(`/api/budgets${buildQuery({ month })}`)
}

export function putBudgets(
  month: string,
  budgets: { category_id: number; amount: string }[],
): Promise<{ status: string }> {
  return req(`/api/budgets${buildQuery({ month })}`, jsonInit('PUT', { budgets }))
}

export function getRecurring(view: View, month: string): Promise<{ recurring: Recurring[] }> {
  return req(`/api/recurring${buildQuery({ view, month })}`)
}

export function getTrends(view: View, month: string, months: number): Promise<{ trends: TrendPoint[] }> {
  return req(`/api/trends${buildQuery({ view, month, months })}`)
}

export function uploadVenmo(file: File): Promise<{ upserted: number; categorized: number; paired: number }> {
  const form = new FormData()
  form.append('file', file)
  return req('/api/imports/venmo', { method: 'POST', body: form })
}

export function getSyncStatus(): Promise<{ runs: SyncRun[] }> {
  return req('/api/sync/status')
}

export function putSplits(id: number, splits: SplitInput[]): Promise<{ transaction: Txn }> {
  return req(`/api/transactions/${id}/splits`, jsonInit('PUT', { splits }))
}

export function deleteSplits(id: number): Promise<{ status: string }> {
  return req(`/api/transactions/${id}/splits`, { method: 'DELETE' })
}

export function getForecast(view: View, month: string): Promise<{ forecast: Forecast }> {
  return req(`/api/forecast${buildQuery({ view, month })}`)
}

export function getInsights(view: View, month: string): Promise<{ insights: Insight[] }> {
  return req(`/api/insights${buildQuery({ view, month })}`)
}
