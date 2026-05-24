package apps

import (
	"bufio"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type AppInfo struct {
	Name        string
	DisplayName string
	Executable  string
	Command     string
	Categories  string
	Icon        string
	DesktopFile string
}

// ScanApplications escanea las carpetas de aplicaciones del sistema (.desktop) y las guarda en la base de datos.
func ScanApplications(db *sql.DB) error {
	dirs := []string{
		"/usr/share/applications",
	}
	
	home, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs, filepath.Join(home, ".local/share/applications"))
	}

	execRegex := regexp.MustCompile(`%[fFuUidDnNksv]`)

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue // Directorio no existe o no accesible
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".desktop") {
				continue
			}

			filePath := filepath.Join(dir, file.Name())
			app, err := parseDesktopFile(filePath)
			if err != nil {
				continue
			}

			// Limpiar el comando Exec de variables %u, %F, etc.
			cleanCommand := execRegex.ReplaceAllString(app.Command, "")
			cleanCommand = strings.TrimSpace(cleanCommand)
			
			// Obtener el ejecutable limpio (el primer token del comando)
			parts := strings.Fields(cleanCommand)
			var cleanExec string
			if len(parts) > 0 {
				cleanExec = parts[0]
			} else {
				cleanExec = cleanCommand
			}

			if app.Name == "" || cleanCommand == "" {
				continue
			}

			query := `
			INSERT INTO app_launchers (name, display_name, executable, desktop_file, command, categories, icon, source, is_available, last_verified_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'system', 1, datetime('now'))
			ON CONFLICT(name, executable) DO UPDATE SET
				display_name = excluded.display_name,
				command = excluded.command,
				categories = excluded.categories,
				icon = excluded.icon,
				last_verified_at = datetime('now'),
				is_available = 1;
			`

			_, err = db.Exec(query, strings.ToLower(app.Name), app.DisplayName, cleanExec, filePath, cleanCommand, app.Categories, app.Icon)
			if err == nil {
				// Agregar al search_index FTS5
				var entityID int64
				row := db.QueryRow("SELECT id FROM app_launchers WHERE name = ? AND executable = ?", strings.ToLower(app.Name), cleanExec)
				if err := row.Scan(&entityID); err == nil {
					_, _ = db.Exec("DELETE FROM search_index WHERE entity_type = 'app' AND entity_id = ?", entityID)
					_, _ = db.Exec("INSERT INTO search_index (entity_type, entity_id, title, body, path) VALUES ('app', ?, ?, ?, ?)",
						entityID, app.DisplayName, app.Categories, cleanExec)
				}
			}
		}
	}

	return nil
}

func parseDesktopFile(path string) (*AppInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	app := &AppInfo{
		DesktopFile: path,
	}

	inDesktopEntry := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "[Desktop Entry]" {
			inDesktopEntry = true
			continue
		} else if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inDesktopEntry = false
			continue
		}

		if !inDesktopEntry {
			continue
		}

		if strings.HasPrefix(line, "Name=") {
			app.DisplayName = strings.TrimPrefix(line, "Name=")
			if app.Name == "" {
				app.Name = app.DisplayName
			}
		} else if strings.HasPrefix(line, "Exec=") {
			app.Command = strings.TrimPrefix(line, "Exec=")
		} else if strings.HasPrefix(line, "Icon=") {
			app.Icon = strings.TrimPrefix(line, "Icon=")
		} else if strings.HasPrefix(line, "Categories=") {
			app.Categories = strings.TrimPrefix(line, "Categories=")
		}
	}

	return app, scanner.Err()
}

// VerifyApplications comprueba si las aplicaciones registradas siguen estando instaladas.
func VerifyApplications(db *sql.DB) {
	rows, err := db.Query("SELECT id, executable FROM app_launchers")
	if err != nil {
		return
	}
	defer rows.Close()

	type appCheck struct {
		id   int64
		exec string
	}
	var checks []appCheck

	for rows.Next() {
		var c appCheck
		if err := rows.Scan(&c.id, &c.exec); err == nil {
			checks = append(checks, c)
		}
	}

	for _, c := range checks {
		isAvailable := 1
		// Si es una ruta absoluta, comprobar existencia
		if filepath.IsAbs(c.exec) {
			if _, err := os.Stat(c.exec); err != nil {
				isAvailable = 0
			}
		} else {
			// Si no, buscar en PATH
			_, err := execLookPath(c.exec)
			if err != nil {
				isAvailable = 0
			}
		}

		_, _ = db.Exec("UPDATE app_launchers SET is_available = ?, last_verified_at = datetime('now') WHERE id = ?", isAvailable, c.id)
	}
}

// Envuelto para poder mockear o controlar
var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}
