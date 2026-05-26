package environment

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Detect probes the current Linux desktop environment and returns capabilities.
// Results are persisted to the database before returning.
func Detect(db *sql.DB) (*DesktopCapabilities, error) {
	caps := &DesktopCapabilities{}

	// --- Session / compositor detection ---
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	hyprSig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	swaySocket := os.Getenv("SWAYSOCK")

	caps.SessionType = sessionType

	switch {
	case hyprSig != "":
		caps.IsHyprland = true
		caps.Desktop = "Hyprland"
	case swaySocket != "":
		caps.IsSway = true
		caps.Desktop = "Sway"
	case sessionType == "wayland":
		caps.Desktop = "Wayland"
	case sessionType == "x11":
		caps.IsX11 = true
		caps.Desktop = "X11"
	default:
		xdg := os.Getenv("XDG_CURRENT_DESKTOP")
		if xdg != "" {
			caps.Desktop = xdg
		} else {
			caps.Desktop = "unknown"
		}
	}

	if sessionType == "x11" {
		caps.IsX11 = true
	}

	// --- Binary probing ---
	caps.HasHyprctl = hasBinary("hyprctl")
	caps.HasSwaymsg = hasBinary("swaymsg")
	caps.HasWmctrl = hasBinary("wmctrl")
	caps.HasXdotool = hasBinary("xdotool")
	caps.HasWtype = hasBinary("wtype")
	caps.HasYdotool = hasBinary("ydotool")
	caps.HasPlayerctl = hasBinary("playerctl")
	caps.HasWpctl = hasBinary("wpctl")
	caps.HasPactl = hasBinary("pactl")
	caps.HasAmixer = hasBinary("amixer")

	// --- ydotool usability check ---
	if caps.HasYdotool {
		caps.YdotoolUsable, caps.YdotoolUnavailReason = checkYdotool()
	} else {
		caps.YdotoolUnavailReason = "ydotool binary not found"
	}

	// --- Resolve backends ---
	caps.WindowBackend = resolveWindowBackend(caps)
	caps.InputBackend = resolveInputBackend(caps)
	caps.MediaBackend = resolveMediaBackend(caps)
	caps.VolumeBackend = resolveVolumeBackend(caps)

	// --- Persist to DB ---
	if db != nil {
		if err := SaveCapabilities(db, caps); err != nil {
			// Non-fatal: log-worthy but we still return the caps.
			_ = fmt.Errorf("environment.Detect: %w", err)
		}
	}

	return caps, nil
}

// hasBinary returns true if the binary is found on PATH.
func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// checkYdotool verifies whether ydotool is actually usable:
// it tries to open /dev/uinput (write) and also tries running ydotool --help.
func checkYdotool() (usable bool, reason string) {
	// Check /dev/uinput accessibility
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err == nil {
		f.Close()
		return true, ""
	}
	uinputErr := err.Error()

	// Fallback: try running ydotool --help with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ydotool", "--help")
	if runErr := cmd.Run(); runErr == nil {
		return true, ""
	}

	return false, fmt.Sprintf("/dev/uinput not accessible (%s) and ydotool --help failed", uinputErr)
}

func resolveWindowBackend(caps *DesktopCapabilities) string {
	switch {
	case caps.IsHyprland && caps.HasHyprctl:
		return "hyprland"
	case caps.IsSway && caps.HasSwaymsg:
		return "sway"
	case caps.IsX11 && caps.HasWmctrl:
		return "x11"
	case caps.IsX11 && caps.HasXdotool:
		return "x11"
	default:
		return "noop"
	}
}

func resolveInputBackend(caps *DesktopCapabilities) string {
	switch {
	case caps.IsX11 && caps.HasXdotool:
		return "x11"
	case (caps.IsHyprland || caps.IsSway) && caps.HasWtype && caps.YdotoolUsable:
		return "wayland"
	case caps.HasWtype:
		return "wayland"
	default:
		return "noop"
	}
}

func resolveMediaBackend(caps *DesktopCapabilities) string {
	if caps.HasPlayerctl {
		return "playerctl"
	}
	return "noop"
}

func resolveVolumeBackend(caps *DesktopCapabilities) string {
	switch {
	case caps.HasWpctl:
		return "wpctl"
	case caps.HasPactl:
		return "pactl"
	case caps.HasAmixer:
		return "amixer"
	default:
		return "noop"
	}
}
