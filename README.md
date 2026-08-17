# vollmint

> Self-hosted household budget tracker — automatic bank sync, transfer-aware categorization, and recurring-charge forecasting.

[![CI](https://github.com/vollminlab/vollmint/actions/workflows/ci.yml/badge.svg)](https://github.com/vollminlab/vollmint/actions/workflows/ci.yml)
[![Build and Publish](https://github.com/vollminlab/vollmint/actions/workflows/build.yml/badge.svg)](https://github.com/vollminlab/vollmint/actions/workflows/build.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)
![Postgres](https://img.shields.io/badge/Postgres-16-4169E1?logo=postgresql&logoColor=white)

**vollmint** answers four questions about a household's money: where it actually goes, which
recurring charges are worth cutting, how much is impulse spending, and whether paycheck allocation
can improve. It pulls transactions from real bank accounts twice a day via
[SimpleFIN Bridge](https://beta-bridge.simplefin.org/), ingests Venmo activity from CSV statements,
and reconciles the two so a card payment or a Venmo cash-out doesn't get double-counted as spending.

It is a single Go binary with the React SPA embedded, backed by one Postgres database. No Redis, no
queue, no second service. Authentication is handled entirely upstream by Authentik forward-auth —
the application itself contains no auth code and trusts the proxy.

---

## Architecture

```mermaid
flowchart TB
    subgraph ext["External"]
        SF["SimpleFIN Bridge<br/><i>read-only bank aggregation</i>"]
        CSV["Venmo CSV<br/><i>statement export</i>"]
    end

    subgraph app["vollmint · one image, two entrypoints"]
        direction TB
        CRON["<b>vollmint sync</b><br/>CronJob · 06:10 &amp; 18:10 UTC"]
        SERVE["<b>vollmint serve</b><br/>Deployment · :8080"]
        ENRICH["enrichment pipeline<br/>rules → transfer matching → split cleanup → pending sweep"]
        SPA["embedded React SPA<br/><i>go:embed</i>"]
    end

    PG[("PostgreSQL<br/><i>CNPG · 2 instances</i>")]
    USER["Browser"]
    AK["Authentik<br/>forward-auth"]

    SF -->|"HTTPS · 90-day window"| CRON
    CSV -->|"POST /api/imports/venmo"| SERVE
    CRON --> ENRICH
    ENRICH --> PG
    SERVE <--> PG
    SERVE --- SPA
    USER --> AK
    AK -->|authenticated| SERVE

    classDef store fill:#1f6feb,stroke:#388bfd,color:#fff
    class PG store
```

Only the CronJob ever holds the SimpleFIN credential — the `serve` process never sees
`SIMPLEFIN_ACCESS_URL`, so a compromise of the web-facing pod cannot reach the bank feed.

### Package layout

Everything lives under `internal/`, so nothing is importable by other modules.

| Package | Responsibility |
|---|---|
| `internal/simplefin` | SimpleFIN HTTP client. No database access. |
| `internal/venmo` | CSV → transaction structs. Pure parsing, no database access. |
| `internal/store` | The only Postgres access layer (pgx). Upserts, queries, splits, snapshots. |
| `internal/ingest` | Orchestration and enrichment — sync, rules, transfer matching, sweeps. |
| `internal/report` | Read-only aggregation. Never writes. |
| `internal/migrate` | goose migrations, embedded via `go:embed`. |
| `internal/api` | HTTP handlers, middleware, SPA catch-all. |
| `web` | The Vite build, embedded via `go:embed all:dist`. |

---

## How it works

### Ingestion is idempotent and self-healing

Every transaction carries a `UNIQUE (source, external_id)` key, so re-running a sync over an
overlapping window updates rather than duplicates. The sync window is derived from the last
successful run minus seven days; a first run backfills 85 days, just inside SimpleFIN's 90-day cap.
If an institution errors, the run is recorded as `partial` rather than failed, and the next run
re-covers the gap.

Ingestion never deletes, with one deliberate exception: pending rows untouched for 14 days are
swept, because they represent authorizations the bank silently dropped.

### Transfers are matched, not guessed

Moving money between your own accounts is not spending. Three passes run inside a single
transaction:

| Pass | Matches | Window |
|---|---|---|
| Venmo debit | Bank-side `VENMO` ACH debit ↔ Venmo CSV row of equal amount | ±3 days |
| Venmo cash-out | Bank-side credit ↔ Venmo `Standard Transfer` of equal magnitude | ±3 days |
| Card payment | Negative and positive legs across different accounts, gated on a payment-descriptor pattern | ±5 days |

For a Venmo debit only the *bank* leg becomes a transfer — the Venmo leg carries the real spend, so
the category detail survives. Categories you assigned by hand are sticky: matching may only
overwrite `NULL` or the seeded `Needs Venmo detail` placeholder.

Until a Venmo CSV pairs them, bank-side Venmo debits land in that `Needs Venmo detail` bucket rather
than being ignored, so totals never understate.

### Money is never a float

Amounts are decimal strings end to end. Aggregation happens in Postgres `numeric` and is cast to
text at the boundary; the frontend treats money fields as strings and coerces to number only for
chart geometry.

### Views are filters, not accounts

There is one login. `scott`, `nikki`, `joint`, and `household` are UI filters over the same data.
Ownership is a property of an account, with a per-transaction override — the effective owner is
`COALESCE(t.owner_override, a.owner)`.

---

## Features

- **Dashboard** — in/out/vices/budget rollup, spend by category, upcoming bills
- **Transactions** — filter by view, month, category, account, free text, or uncategorized; inline
  category and owner editing; split a transaction across multiple categories
- **Recurring** — payees charged in three or more distinct months, with first-charge-this-month flagged
- **Trends** — monthly in/out over a rolling window, zero-filled for a continuous axis
- **Forecast** — predicts this month's bills from median day-of-month and most recent amount; P2P
  payees excluded
- **Insights** — generated cards for budget breaches, category spikes, subscription totals, price
  increases, and overlapping subscriptions
- **Net worth** — daily carry-forward series from balance snapshots, plus manually tracked accounts
- **Budgets** — per-category monthly amounts, household-wide
- **Rules** — substring or regex auto-categorization; creating a rule immediately re-runs it over
  uncategorized history

Splits are validated in the database: at least two parts, summing exactly to the parent amount, all
non-zero and same sign. Transfers and pending transactions cannot be split. Net worth never
fabricates history — a date before an account's first snapshot yields no point at all.

---

## API

JSON over HTTP. Errors are `{"error":"message"}`. List endpoints wrap their array in a named key.
`view` defaults to `household`; `month` is `YYYY-MM` and required where noted.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Liveness and readiness. Plain text, no database access. |
| `GET` | `/metrics` | Prometheus metrics. |
| `GET` | `/api/transactions` | List. Params: `view`, `month`, `category`, `account`, `q`, `uncategorized`. |
| `PATCH` | `/api/transactions/{id}` | Set `category_id` and/or `owner_override`. |
| `PUT` | `/api/transactions/{id}/splits` | Replace the split set. |
| `DELETE` | `/api/transactions/{id}/splits` | Remove all splits. |
| `GET` | `/api/summary` | Dashboard rollup. Requires `month`. |
| `GET` | `/api/forecast` | Recurring-bill forecast. Requires `month`. |
| `GET` | `/api/insights` | Generated insight cards. Requires `month`. |
| `GET` | `/api/recurring` | Detected recurring charges. |
| `GET` | `/api/trends` | Monthly in/out series. `months` 1–60, default 12. |
| `GET` | `/api/categories` | List categories. |
| `POST` | `/api/categories` | Create. `name` required; `kind` defaults to `spend`. |
| `PATCH` | `/api/categories/{id}` | Partial update of `name`, `kind`, `is_vice`. |
| `GET` | `/api/rules` | List rules in priority order. |
| `POST` | `/api/rules` | Create; re-applies over uncategorized history, returns `recategorized`. |
| `DELETE` | `/api/rules/{id}` | Delete. Rules are not editable by design. |
| `GET` | `/api/budgets` | Budgets for `month`. |
| `PUT` | `/api/budgets` | Replace all budgets for `month`. An empty list clears it. |
| `GET` | `/api/networth` | Daily series and current accounts. `range` ∈ `1m,3m,6m,1y,all`. |
| `POST` | `/api/accounts/manual` | Create a manually tracked account. |
| `PUT` | `/api/accounts/{id}/balance` | Update a manual account's balance. |
| `POST` | `/api/imports/venmo` | Multipart CSV upload, field `file`, ≤10 MiB. |
| `GET` | `/api/sync/status` | Last 20 ingestion runs. |
| `GET` | `/` | Embedded SPA. Unknown paths serve `index.html`; unmatched `/api` paths return JSON 404. |

---

## Data model

goose migrations under `internal/migrate/migrations/`, applied automatically by every entrypoint
that touches the database. There is no separate migrate command.

| Table | Purpose |
|---|---|
| `accounts` | SimpleFIN accounts plus the synthetic `venmo` account and manual accounts. Owner is `scott`, `nikki`, or `joint`. |
| `categories` | Seeded set of 15, extensible. `kind` ∈ `spend`, `income`, `transfer`, `savings`; `is_vice` flag. |
| `transactions` | The ledger. `UNIQUE (source, external_id)` is the idempotency key. Negative amounts are outflows. |
| `transaction_splits` | One transaction divided across categories, with invariants enforced in SQL. |
| `category_rules` | Substring or regex auto-categorization, first match wins by priority. |
| `budgets` | Per-category monthly amounts, keyed on the first of the month. |
| `account_balance_snapshots` | Daily balances keyed on the account's own `balance_date`, powering net worth. |
| `sync_runs` | Audit log of every ingestion run, and the source of truth for the next sync window. |

---

## Getting started

Requires Go 1.26+, Node 20+, and a Postgres 16 instance.

```bash
# 1. Postgres
docker run -d --name vollmint-pg -e POSTGRES_PASSWORD=dev -p 5433:5432 postgres:16
export DATABASE_URL='postgres://postgres:dev@localhost:5433/postgres?sslmode=disable'

# 2. Backend — migrates, then serves on :8080
go run ./cmd/vollmint serve

# 3. Frontend — Vite on :5173, proxies /api, /healthz, /metrics to :8080
cd web && npm install && npm run dev
```

Production build — compiles the SPA, then embeds it in the binary:

```bash
./scripts/build.sh    # → bin/vollmint
```

See [`docs/development.md`](docs/development.md) for the full dev loop.

### CLI

```
usage: vollmint <claim|sync|import-venmo|serve> [args]
```

| Command | Requires | Notes |
|---|---|---|
| `vollmint claim <setup-token>` | — | One-time SimpleFIN claim. Prints the permanent Access URL. |
| `vollmint sync` | `DATABASE_URL`, `SIMPLEFIN_ACCESS_URL` | Pull, enrich, record the run. |
| `vollmint import-venmo <file.csv>` | `DATABASE_URL` | Import a Venmo statement. The CSV is never persisted. |
| `vollmint serve` | `DATABASE_URL` | Serve API and SPA. |

`vollmint claim` spends the setup token permanently and prints an Access URL of the form
`https://user:pass@host/path` — the embedded credential is the *only* secret. Store it in 1Password
immediately; never write it to disk.

### Configuration

The complete set of environment variables. There is no config file.

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — | Postgres connection string. Required by every command except `claim`. |
| `SIMPLEFIN_ACCESS_URL` | — | SimpleFIN Access URL. Required by `sync` only. |
| `LISTEN_ADDR` | `:8080` | Bind address for `serve`. |
| `TEST_DATABASE_URL` | — | Used by the test suite. |

---

## Testing

```bash
export TEST_DATABASE_URL='postgres://postgres:dev@localhost:5433/postgres?sslmode=disable'
go test ./... -count=1     # ~200 Go tests, most are database-backed
cd web && npm test         # Vitest
```

Database-backed tests fail rather than skip when `TEST_DATABASE_URL` is unset — a silent skip would
hide real breakage. Because `go test ./...` runs packages concurrently against one database, the
suite serializes itself with a Postgres advisory lock.

---

## Deployment

Runs on the [vollminlab Kubernetes cluster](https://github.com/vollminlab/k8s-vollminlab-cluster) as
a Flux `HelmRelease`. Tagging `vX.Y.Z` builds and pushes both the image and the chart to Harbor;
Flux picks up the pinned chart version from the cluster repo.

The chart renders three objects — a Deployment, the sync CronJob, and a ClusterIP Service. The
Ingress, ExternalSecrets, NetworkPolicies, and the CNPG Postgres cluster live in the cluster repo,
not here.

| Concern | Where |
|---|---|
| Image and chart | `harbor.vollminlab.com/vollminlab/{vollmint,charts/vollmint}` |
| Chart source | [`charts/vollmint/`](charts/vollmint) |
| Cluster manifests | `clusters/vollminlab-cluster/vollmint/` in the cluster repo |
| Database | CNPG `vollmint-db`, 2 instances, barman backups to MinIO |
| Secrets | External Secrets Operator, sourced from 1Password |

The container is distroless, runs as UID 65532 with a read-only root filesystem, and drops all
capabilities.

---

## Design notes

The full decision record — build-vs-adopt analysis, why SimpleFIN, and the verified constraints
behind the ingestion design — lives in the cluster repo under
`docs/superpowers/specs/vollmint-design.md`, alongside the per-feature specs for forecasting,
splits, insights, and net-worth snapshots.

A few things that are deliberate rather than unfinished:

- Owner values are constrained to `scott`, `nikki`, and `joint` at the database level. This is not a
  multi-tenant application and is not intended to become one.
- Rules can be created and deleted but not edited — an edited rule's past effects are ambiguous.
- `POST /api/imports/venmo` and `GET /api/sync/status` have typed clients but no UI yet.
