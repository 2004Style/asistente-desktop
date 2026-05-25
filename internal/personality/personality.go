package personality

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type AssistantState string

const (
	StateIdle       AssistantState = "idle"
	StateObserving  AssistantState = "observing"
	StatePlanning   AssistantState = "planning"
	StateExecuting  AssistantState = "executing"
	StateVerifying  AssistantState = "verifying"
	StateConfirming AssistantState = "confirming"
	StateError      AssistantState = "error"
	StateDone       AssistantState = "done"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type ResponseContext struct {
	State     AssistantState
	Risk      RiskLevel
	ToolName  string
	Target    string
	Ambiguous bool
	Error     error
	AgentName string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randomChoice devuelve un string aleatorio de la lista dada
func randomChoice(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[rand.Intn(len(options))]
}

// ComposeResponse genera respuestas habladas/escritas dinámicas según la personalidad elegida y el estado
func ComposeResponse(ctx ResponseContext) string {
	if ctx.Risk == RiskHigh && ctx.State == StateConfirming {
		return "No continuaré sin confirmación. Esta acción puede modificar elementos importantes."
	}

	if ctx.Ambiguous {
		return "Encontré varias posibilidades. Te mostraré candidatos antes de actuar."
	}

	switch ctx.State {
	case StateIdle:
		return "Todo está listo. Te escucho."
	case StateObserving:
		return "Estoy revisando el entorno."
	case StatePlanning:
		return "Analizando los pasos necesarios."
	case StateExecuting:
		return "Entendido. Procediendo."
	case StateVerifying:
		return "Verificando el resultado de la acción."
	case StateConfirming:
		return "Necesito confirmación para continuar."
	case StateError:
		if ctx.Error != nil {
			return fmt.Sprintf("He encontrado un inconveniente: %v", ctx.Error)
		}
		return "He encontrado un inconveniente. Tengo una posible causa."
	case StateDone:
		return handleStateDone(ctx)
	default:
		return "Todo está en orden. Te escucho."
	}
}

// handleStateDone provee variedad para la resolución de herramientas de riesgo bajo
func handleStateDone(ctx ResponseContext) string {
	switch ctx.ToolName {
	case "desktop.open_app":
		return fmt.Sprintf("Listo. Aplicación %s lanzada.", formatTarget(ctx.Target))
	case "desktop.close_app":
		return fmt.Sprintf("Listo. La aplicación %s fue cerrada.", formatTarget(ctx.Target))
	case "browser.open_url":
		return "Hecho. He abierto la página en el navegador."
	default:
		options := []string{
			"Hecho.",
			"Todo quedó en orden.",
			"Operación completada.",
			"Listo.",
		}
		return randomChoice(options)
	}
}

func formatTarget(target string) string {
	if target == "" {
		return "solicitada"
	}
	
	// Limpieza sencilla para respuestas habladas:
	if strings.Contains(target, "code") || strings.Contains(target, "vscode") {
		return "VS Code"
	}
	return target
}
