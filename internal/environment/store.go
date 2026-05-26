package environment

import (
	"database/sql"
	"fmt"
)

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SaveCapabilities persists the detected desktop capabilities to the database.
func SaveCapabilities(db *sql.DB, caps *DesktopCapabilities) error {
	type row struct {
		key       string
		value     string
		available int
	}

	rows := []row{
		{"session_type", caps.SessionType, 1},
		{"desktop", caps.Desktop, 1},
		{"is_hyprland", "", boolInt(caps.IsHyprland)},
		{"is_sway", "", boolInt(caps.IsSway)},
		{"is_x11", "", boolInt(caps.IsX11)},
		{"has_hyprctl", "", boolInt(caps.HasHyprctl)},
		{"has_swaymsg", "", boolInt(caps.HasSwaymsg)},
		{"has_wmctrl", "", boolInt(caps.HasWmctrl)},
		{"has_xdotool", "", boolInt(caps.HasXdotool)},
		{"has_wtype", "", boolInt(caps.HasWtype)},
		{"has_ydotool", "", boolInt(caps.HasYdotool)},
		{"ydotool_usable", caps.YdotoolUnavailReason, boolInt(caps.YdotoolUsable)},
		{"has_playerctl", "", boolInt(caps.HasPlayerctl)},
		{"has_wpctl", "", boolInt(caps.HasWpctl)},
		{"has_pactl", "", boolInt(caps.HasPactl)},
		{"has_amixer", "", boolInt(caps.HasAmixer)},
		{"window_backend", caps.WindowBackend, 1},
		{"input_backend", caps.InputBackend, 1},
		{"media_backend", caps.MediaBackend, 1},
		{"volume_backend", caps.VolumeBackend, 1},
	}

	for _, r := range rows {
		_, err := db.Exec(
			`INSERT OR REPLACE INTO environment_capabilities (key, value, available, checked_at)
			 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			r.key, r.value, r.available,
		)
		if err != nil {
			return fmt.Errorf("environment/store: saving key %q: %w", r.key, err)
		}
	}
	return nil
}

// LoadCapabilities reads the previously stored desktop capabilities from the database.
func LoadCapabilities(db *sql.DB) (*DesktopCapabilities, error) {
	rows, err := db.Query(`SELECT key, value, available FROM environment_capabilities`)
	if err != nil {
		return nil, fmt.Errorf("environment/store: loading capabilities: %w", err)
	}
	defer rows.Close()

	caps := &DesktopCapabilities{}
	for rows.Next() {
		var key, value string
		var available int
		if err := rows.Scan(&key, &value, &available); err != nil {
			return nil, fmt.Errorf("environment/store: scanning row: %w", err)
		}
		toBool := available != 0
		switch key {
		case "session_type":
			caps.SessionType = value
		case "desktop":
			caps.Desktop = value
		case "is_hyprland":
			caps.IsHyprland = toBool
		case "is_sway":
			caps.IsSway = toBool
		case "is_x11":
			caps.IsX11 = toBool
		case "has_hyprctl":
			caps.HasHyprctl = toBool
		case "has_swaymsg":
			caps.HasSwaymsg = toBool
		case "has_wmctrl":
			caps.HasWmctrl = toBool
		case "has_xdotool":
			caps.HasXdotool = toBool
		case "has_wtype":
			caps.HasWtype = toBool
		case "has_ydotool":
			caps.HasYdotool = toBool
		case "ydotool_usable":
			caps.YdotoolUsable = toBool
			caps.YdotoolUnavailReason = value
		case "has_playerctl":
			caps.HasPlayerctl = toBool
		case "has_wpctl":
			caps.HasWpctl = toBool
		case "has_pactl":
			caps.HasPactl = toBool
		case "has_amixer":
			caps.HasAmixer = toBool
		case "window_backend":
			caps.WindowBackend = value
		case "input_backend":
			caps.InputBackend = value
		case "media_backend":
			caps.MediaBackend = value
		case "volume_backend":
			caps.VolumeBackend = value
		}
	}
	return caps, rows.Err()
}
