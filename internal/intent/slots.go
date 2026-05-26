package intent

import (
	"rbot/internal/intent/slots"
)

// ExtractSlots unifica los extractores por dominio y devuelve un mapa con los slots encontrados.
func ExtractSlots(intentName string, userInput string) map[string]interface{} {
	slotsMap := make(map[string]interface{})

	// 1. Aplicación
	if app := slots.ExtractApp(userInput); app != "" {
		slotsMap["app"] = app
	}

	// 2. Archivos y carpetas
	path, fileName, folderName := slots.ExtractFileSlots(userInput)
	if path != "" {
		slotsMap["path"] = path
	}
	if fileName != "" {
		slotsMap["file_name"] = fileName
	}
	if folderName != "" {
		slotsMap["folder_name"] = folderName
	}

	// 3. Navegador
	url, browserQuery := slots.ExtractBrowserSlots(userInput)
	if url != "" {
		slotsMap["url"] = url
	}
	if browserQuery != "" {
		slotsMap["query"] = browserQuery
	}

	// 4. Medios (música/canciones)
	if mediaQuery := slots.ExtractMediaSlots(userInput); mediaQuery != "" {
		// Damos prioridad a mediaQuery si el intent es de reproducción
		if intentName == "play_music" || intentName == "media" || slotsMap["query"] == nil {
			slotsMap["query"] = mediaQuery
		}
	}

	// 5. Fecha/Hora y Duración
	datetime, duration := slots.ExtractDateTimeSlots(userInput)
	if datetime != "" {
		slotsMap["datetime"] = datetime
	}
	if duration != "" {
		slotsMap["duration"] = duration
	}

	// 6. Entradas simuladas (Teclado/Mouse)
	key, keys, text, button := slots.ExtractInputSlots(userInput)
	if key != "" {
		slotsMap["key"] = key
	}
	if keys != "" {
		slotsMap["keys"] = keys
	}
	if text != "" {
		slotsMap["text"] = text
	}
	if button != "" {
		slotsMap["button"] = button
	}

	// 7. Sistema y Memoria
	command, keyMem, valMem, workspace := slots.ExtractSystemSlots(userInput)
	if command != "" {
		slotsMap["command"] = command
	}
	if keyMem != "" {
		// Evitar colisionar con key de teclado a menos que sea de memoria
		if intentName == "remember" || intentName == "memory" {
			slotsMap["key"] = keyMem
		}
	}
	if valMem != "" {
		slotsMap["value"] = valMem
	}
	if workspace != "" {
		slotsMap["workspace"] = workspace
	}

	return slotsMap
}
