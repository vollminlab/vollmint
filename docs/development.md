# vollmint — local development

## Prerequisites

- Go on PATH: `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"`
- Node 20+ and npm
- A Postgres 16 dev instance at `localhost:5433` (user `postgres`, password `dev`)

## Backend

```bash
export DATABASE_URL='postgres://postgres:dev@localhost:5433/postgres?sslmode=disable'
go run ./cmd/vollmint serve            # migrates, then serves on :8080
```

Environment:
- `DATABASE_URL` (required) — Postgres DSN
- `LISTEN_ADDR` (optional, default `:8080`)

The `serve` process never touches SimpleFIN credentials — ingestion runs
separately via the `sync` / `import-venmo` subcommands (see the deploy plan for
the CronJob wiring).

## Frontend

```bash
cd web
npm install
npm run dev       # Vite dev server on :5173, proxies /api → :8080
```

Run the Go server (`serve`) in one terminal and the Vite dev server in another;
the proxy forwards API calls.

## Tests

```bash
# Go (needs the dev DB)
export TEST_DATABASE_URL='postgres://postgres:dev@localhost:5433/postgres?sslmode=disable'
go test ./... -count=1

# Frontend
cd web && npm test
```

## Production build

```bash
./scripts/build.sh     # builds the SPA, embeds it, produces ./bin/vollmint
```
