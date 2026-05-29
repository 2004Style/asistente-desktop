# Compilación y Construcción del Proyecto

Este documento describe cómo compilar las pruebas y construir los ejecutables de RBot de forma local.

---

## 1. Validación de Pruebas por Defecto

La suite de CI y las puertas de enlace ejecutan las pruebas básicas en todo el proyecto:

```bash
go test ./...
```

*Nota: La interfaz flotante del HUD (GTK) se encuentra excluida por defecto de `go test ./...` mediante exclusión de tags para evitar que las pruebas fallen en entornos headless.*

---

## 2. Compilación de Desarrollo Diario

Para compilar todos los binarios de desarrollo rápidamente y registrar los enlaces simbólicos de sistema en tu carpeta de usuario:

```bash
# Compilar todo sin el HUD de GTK
./scripts/setup_dev.sh

# Compilar todo incluyendo el HUD de GTK (si tienes librerías dev de GTK3)
BUILD_HUD=1 ./scripts/setup_dev.sh
```

---

## 3. Comandos de Compilación Manuales por Ejecutable

Puedes compilar cualquier ejecutable específico del proyecto de la siguiente forma:

### Panel de Ajustes (Gio GUI)
```bash
go build -o bin/rbot-settings-gio ./cmd/rbot-settings-gio
```

### Daemon Principal (`rbotd`)
```bash
go build -o bin/rbotd ./cmd/rbotd
```

### Controlador CLI (`rbotctl`)
```bash
go build -o bin/rbotctl ./cmd/rbotctl
```

### Cliente Principal (`rbot`)
```bash
go build -tags "gtk_3_18 hud" -o bin/rbot ./cmd/rbot
```

### HUD Flotante (GTK 3)
```bash
# Requiere tag hud explícito
go test -tags hud ./internal/hud ./cmd/rbot-hud
go build -tags hud -o bin/rbot-hud ./cmd/rbot-hud
```
