#!/usr/bin/env bash
set -euo pipefail

# Smoke script for GUI (manual/headful).
# Starts rbotd in background (if available) and launches the Fyne settings UI.
# Usage: ./scripts/smoke_gui.sh

RBOTD_BIN=./bin/rbotd
GUI_BIN=./bin/rbot-settings-ui

# Start daemon if available
if [ -x "$RBOTD_BIN" ]; then
  echo "Starting rbotd..."
  "$RBOTD_BIN" &
  RBOTD_PID=$!
  echo "rbotd PID=$RBOTD_PID"
  # give it a moment to initialize
  sleep 1
else
  echo "rbotd binary not found at $RBOTD_BIN. Ensure daemon is running for full smoke tests."
fi

# Prefer built binary, fallback to go run
if [ -x "$GUI_BIN" ]; then
  echo "Launching GUI: $GUI_BIN"
  "$GUI_BIN"
else
  echo "Launching GUI via 'go run cmd/rbot-settings-ui'"
  (cd "$(git rev-parse --show-toplevel)" && go run cmd/rbot-settings-ui)
fi

# Cleanup background daemon if we started it
if [ -n "${RBOTD_PID-}" ]; then
  echo "Stopping rbotd PID=$RBOTD_PID"
  kill "$RBOTD_PID" || true
fi
