#!/usr/bin/env bash
set -euo pipefail

echo "Running RBot developer start script"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "1) Running unit tests..."
go test ./...

echo "2) Building binaries (rbot, rbotd, rbotctl)..."
mkdir -p bin

go build -o bin/rbot ./cmd/rbot
	go build -o bin/rbotd ./cmd/rbotd || true
	go build -o bin/rbotctl ./cmd/rbotctl || true

echo "3) Ensure config exists (dev)"
if [ ! -f config/rbot.yaml ]; then
	mkdir -p config
	./bin/rbot setup --yes || true
fi

echo "Done. You can run the daemon with: ./bin/rbotd &"
echo "Use ./bin/rbotctl providers list or ./bin/rbot chat 'hola'"
