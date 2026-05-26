package runtime

import (
	"fmt"
	"net"
	"os"
)

// CheckInstanceLock verifica si ya hay una instancia activa escuchando en socketPath.
// Si el socket existe pero no responde, se considera huérfano (stale) y se remueve de forma segura.
func CheckInstanceLock(socketPath string) error {
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		// El archivo del socket no existe, podemos continuar sin problema
		return nil
	}

	// El archivo existe, intentamos conectar para verificar actividad
	conn, err := net.Dial("unix", socketPath)
	if err == nil {
		// Conexión exitosa -> hay una instancia de rbotd corriendo activamente
		conn.Close()
		return fmt.Errorf("el daemon RBotd ya se encuentra activo y escuchando en: %s", socketPath)
	}

	// Conexión fallida -> socket stale/huérfano, se procede a su limpieza
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("se detectó un socket stale pero falló su eliminación: %w", err)
	}

	return nil
}
