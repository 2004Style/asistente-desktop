# Compilation

## Default validation

The default developer and CI gate is:

```bash
go test ./...
```

The GTK HUD renderer is excluded from default builds because the native `gotk3/gdk` stack can fail in headless or incompatible environments. Default builds compile a no-GTK HUD stub so non-HUD packages stay testable.

For day-to-day development, use the script path:

```bash
BUILD_HUD=1 ./scripts/setup_dev.sh
BUILD_HUD=1 ./scripts/dev.sh
```

## HUD GTK build

To intentionally validate the GTK HUD renderer, use the `hud` build tag in an environment with GTK 3 development libraries available:

```bash
go test -tags hud ./internal/hud ./cmd/rbot-hud
go build -tags hud -o bin/rbot-hud ./cmd/rbot-hud
```

The release and install scripts can also include the native HUD binary when `BUILD_HUD=1` is set.
