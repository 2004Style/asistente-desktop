package environment

// DesktopCapabilities stores the detected capabilities of the Linux desktop environment.
type DesktopCapabilities struct {
	// Session info
	SessionType string // "wayland" or "x11"
	Desktop     string // "Hyprland", "Sway", "GNOME", "KDE", "X11"

	// Compositor backends
	IsHyprland bool
	IsSway     bool
	IsX11      bool

	// Window management binaries
	HasHyprctl bool
	HasSwaymsg bool
	HasWmctrl  bool
	HasXdotool bool

	// Input emulation binaries
	HasWtype             bool
	HasYdotool           bool
	YdotoolUsable        bool   // true only if binary exists AND /dev/uinput is accessible or ydotoold responds
	YdotoolUnavailReason string

	// Audio/media binaries
	HasPlayerctl bool
	HasWpctl     bool
	HasPactl     bool
	HasAmixer    bool

	// Resolved backend names (for environment.capabilities tool output)
	WindowBackend string // "hyprland", "sway", "x11", "noop"
	InputBackend  string // "x11" (xdotool), "wayland" (wtype+ydotool), "noop"
	MediaBackend  string // "playerctl", "noop"
	VolumeBackend string // "wpctl", "pactl", "amixer", "noop"
}
