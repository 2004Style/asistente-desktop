package input

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WaylandController usa wtype (teclado) y ydotool (ratón) en entornos Wayland.
type WaylandController struct {
	hasWtype      bool
	ydotoolUsable bool
}

func newWaylandController() InputController {
	_, errWtype := exec.LookPath("wtype")
	_, errYdotool := exec.LookPath("ydotool")

	ydotoolUsable := false
	if errYdotool == nil {
		// Verificar acceso a /dev/uinput
		f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
		if err == nil {
			f.Close()
			ydotoolUsable = true
		}
	}

	return &WaylandController{
		hasWtype:      errWtype == nil,
		ydotoolUsable: ydotoolUsable,
	}
}

func (c *WaylandController) Name() string { return "wayland-wtype-ydotool" }

func (c *WaylandController) TypeText(ctx context.Context, text string) error {
	if !c.hasWtype {
		return fmt.Errorf("simulación de teclado no disponible: wtype no encontrado")
	}
	cmd := exec.CommandContext(ctx, "wtype", text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wtype falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *WaylandController) PressKey(ctx context.Context, key string) error {
	if !c.hasWtype {
		return fmt.Errorf("simulación de teclado no disponible: wtype no encontrado")
	}
	// wtype -k para teclas especiales
	cmd := exec.CommandContext(ctx, "wtype", "-k", key)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wtype key falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *WaylandController) Hotkey(ctx context.Context, keys []string) error {
	if !c.hasWtype {
		return fmt.Errorf("simulación de teclado no disponible: wtype no encontrado")
	}
	if len(keys) == 0 {
		return fmt.Errorf("se requiere al menos una tecla")
	}

	// Construir args para wtype: -M mod -k key -m mod
	args := []string{}
	var mods []string
	var mainKeys []string

	for _, k := range keys {
		switch strings.ToLower(k) {
		case "ctrl", "control":
			mods = append(mods, "ctrl")
		case "alt":
			mods = append(mods, "alt")
		case "shift":
			mods = append(mods, "shift")
		case "super", "windows":
			mods = append(mods, "super")
		default:
			mainKeys = append(mainKeys, k)
		}
	}

	// Presionar modificadores
	for _, mod := range mods {
		args = append(args, "-M", mod)
	}
	// Presionar tecla principal
	for _, k := range mainKeys {
		args = append(args, "-k", k)
	}
	// Soltar modificadores
	for _, mod := range mods {
		args = append(args, "-m", mod)
	}

	cmd := exec.CommandContext(ctx, "wtype", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wtype hotkey falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *WaylandController) MoveMouse(ctx context.Context, x, y int) error {
	if !c.ydotoolUsable {
		return fmt.Errorf("movimiento de ratón no disponible: ydotool no usable (requiere acceso a /dev/uinput)")
	}
	cmd := exec.CommandContext(ctx, "ydotool", "mousemove", "--absolute",
		fmt.Sprintf("-x%d", x), fmt.Sprintf("-y%d", y))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ydotool mousemove falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *WaylandController) Click(ctx context.Context, button string) error {
	if !c.ydotoolUsable {
		return fmt.Errorf("clic de ratón no disponible: ydotool no usable (requiere acceso a /dev/uinput)")
	}
	var buttonNum string
	switch strings.ToLower(button) {
	case "left", "izquierda", "":
		buttonNum = "0x110"
	case "right", "derecha":
		buttonNum = "0x111"
	case "middle", "medio", "centro":
		buttonNum = "0x112"
	case "double", "doble":
		// Doble clic izquierdo
		for i := 0; i < 2; i++ {
			cmd := exec.CommandContext(ctx, "ydotool", "click", "0x110")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("ydotool doble clic falló: %v — %s", err, strings.TrimSpace(string(out)))
			}
		}
		return nil
	default:
		buttonNum = "0x110"
	}
	cmd := exec.CommandContext(ctx, "ydotool", "click", buttonNum)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ydotool click falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *WaylandController) Scroll(ctx context.Context, direction string, amount int) error {
	if !c.ydotoolUsable {
		return fmt.Errorf("scroll no disponible: ydotool no usable (requiere acceso a /dev/uinput)")
	}
	if amount <= 0 {
		amount = 3
	}
	axisVal := fmt.Sprintf("%d", amount*120)
	if strings.ToLower(direction) == "down" || strings.ToLower(direction) == "abajo" {
		axisVal = fmt.Sprintf("-%d", amount*120)
	}
	cmd := exec.CommandContext(ctx, "ydotool", "mousemove", "--wheel", axisVal)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ydotool scroll falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
