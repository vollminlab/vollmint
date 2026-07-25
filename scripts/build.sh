#!/usr/bin/env bash
# Builds the vollmint SPA and the Go binary with the SPA embedded.
set -euo pipefail
cd "$(dirname "$0")/.."

export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"

echo "==> building web frontend"
(cd web && npm ci && npm run build)

echo "==> building go binary (embeds web/dist)"
go build -o bin/vollmint ./cmd/vollmint

echo "==> done: ./bin/vollmint"
