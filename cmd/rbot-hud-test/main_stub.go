//go:build !hud

package main

import "fmt"

func main() {
	fmt.Println("RBot HUD test demo is not compiled in this build; rebuild with -tags hud to enable the GTK demo.")
}
