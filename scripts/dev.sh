#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_HUD="${BUILD_HUD:-0}"
LAUNCH_SETTINGS="${LAUNCH_SETTINGS:-1}"

printf '== RBot dev runner ==\n'
printf 'root: %s\n' "$ROOT_DIR"
printf 'BUILD_HUD=%s\n' "$BUILD_HUD"
printf 'LAUNCH_SETTINGS=%s\n' "$LAUNCH_SETTINGS"

printf '\n[1/3] Running tests...\n'
go test ./...

printf '\n[2/3] Preparing dev environment...\n'
"$ROOT_DIR/scripts/setup_dev.sh"

if [ ! -x "$ROOT_DIR/bin/rbotd" ]; then
	echo "bin/rbotd not found after setup"
	exit 1
fi

have_graphics=0
if [ -n "${WAYLAND_DISPLAY:-}" ] || [ -n "${DISPLAY:-}" ]; then
	have_graphics=1
fi

pids=()
cleanup() {
	for pid in "${pids[@]:-}"; do
		kill "$pid" >/dev/null 2>&1 || true
	done
	wait >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

printf '\n[3/3] Starting runtime components...\n'
mkdir -p logs

"$ROOT_DIR/bin/rbotd" >"$ROOT_DIR/logs/rbotd-dev.log" 2>&1 &
pids+=("$!")
DAEMON_PID="${pids[0]}"
printf '  daemon: pid=%s log=%s\n' "$DAEMON_PID" "$ROOT_DIR/logs/rbotd-dev.log"

sleep 1

if [ "$have_graphics" -eq 1 ]; then
	if [ "$BUILD_HUD" = "1" ] && [ -x "$ROOT_DIR/bin/rbot-hud" ]; then
		"$ROOT_DIR/bin/rbot-hud" >"$ROOT_DIR/logs/rbot-hud-dev.log" 2>&1 &
		HUD_PID="$!"
		pids+=("$HUD_PID")
		printf '  hud: pid=%s log=%s\n' "$HUD_PID" "$ROOT_DIR/logs/rbot-hud-dev.log"
	else
		printf '  hud: skipped (BUILD_HUD=1 required to launch the native HUD)\n'
	fi

	if [ "$LAUNCH_SETTINGS" = "1" ] && [ -x "$ROOT_DIR/bin/rbot-settings-gio" ]; then
		"$ROOT_DIR/bin/rbot-settings-gio" >"$ROOT_DIR/logs/rbot-settings-gio-dev.log" 2>&1 &
		SETTINGS_PID="$!"
		pids+=("$SETTINGS_PID")
		printf '  settings: pid=%s log=%s\n' "$SETTINGS_PID" "$ROOT_DIR/logs/rbot-settings-gio-dev.log"
	else
		printf '  settings: skipped (set LAUNCH_SETTINGS=1 and build bin/rbot-settings-gio)\n'
	fi
else
	printf '  gui: skipped (no graphical session detected)\n'
fi

printf '\nDev runtime is up. Waiting on daemon PID %s...\n' "$DAEMON_PID"
wait "$DAEMON_PID"
