# vollmint

Household budget tracker. Go backend, React SPA (plan 2), deployed on the
vollminlab cluster (plan 3). Design spec lives in
k8s-vollminlab-cluster/docs/superpowers/specs/vollmint-design.md.

## Commands
- `vollmint claim <setup-token>` — one-time SimpleFIN claim; prints Access URL (save to 1Password, never to disk)
- `vollmint sync` — pull SimpleFIN accounts/transactions (env: DATABASE_URL, SIMPLEFIN_ACCESS_URL)
- `vollmint import-venmo <file.csv>` — import a Venmo CSV export (env: DATABASE_URL)
- `vollmint serve` — start the HTTP API server; env: DATABASE_URL (required), LISTEN_ADDR (optional, default :8080)

## Dev
docker run -d --name vollmint-pg -e POSTGRES_PASSWORD=dev -p 5433:5432 postgres:16
export TEST_DATABASE_URL='postgres://postgres:dev@localhost:5433/postgres?sslmode=disable'
go test ./...
