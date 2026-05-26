package planner

import (
	"fmt"
)

type FailureRecovery string

const (
	RecoveryAbort    FailureRecovery = "abort"
	RecoveryContinue FailureRecovery = "continue"
	RecoveryRetry    FailureRecovery = "retry"
)

// DetermineRecoveryAction evalúa la falla de un paso y decide si continuar, abortar o reintentar.
func DetermineRecoveryAction(step PlanStep, stepErr error) (FailureRecovery, string) {
	switch step.ToolName {
	case "browser-open", "media", "youtube-media-control":
		// Errores menores no críticos se pueden omitir para no arruinar el resto del plan
		return RecoveryContinue, fmt.Sprintf("Herramienta no crítica '%s' falló, continuando el plan. Detalle: %v", step.ToolName, stepErr)
	default:
		// Por defecto, abortar ante cualquier fallo
		return RecoveryAbort, fmt.Sprintf("Paso crítico '%s' (%s) falló. Abortando plan. Detalle: %v", step.ID, step.ToolName, stepErr)
	}
}
