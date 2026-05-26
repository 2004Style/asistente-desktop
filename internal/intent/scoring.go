package intent

import (
	"strings"

	"rbot/internal/skills"
)

// CalculateScore calcula la confianza de coincidencia de una entrada con una Skill/Herramienta.
func CalculateScore(userInput string, skill skills.SkillMetadata, toolExists func(string) bool, mandatorySlots []string, extractedSlots map[string]interface{}) (float64, string) {
	score := 0.0
	var reasons []string

	userInputLower := strings.ToLower(userInput)

	// 1. Coincidencia en Voice Trigger (+0.60)
	matchedTrigger := false
	for _, trigger := range skill.VoiceTriggers {
		triggerLower := strings.ToLower(trigger)
		if triggerLower != "" && (strings.Contains(userInputLower, triggerLower) || userInputLower == triggerLower) {
			score += 0.60
			matchedTrigger = true
			reasons = append(reasons, "trigger exacto: "+trigger)
			break
		}
	}

	// 2. Verbo de acción compatible (+0.30)
	verbosComunes := []string{"abre", "cierra", "busca", "lee", "crea", "elimina", "pon", "reproduce", "copia", "mueve", "ejecuta", "compila", "presiona", "escribe", "notifica", "recuérdame"}
	matchedVerb := false
	for _, v := range verbosComunes {
		if strings.Contains(userInputLower, v) {
			// Comprobar si la descripción de la skill o el nombre menciona el verbo o es afín
			if strings.Contains(strings.ToLower(skill.Description), v) || strings.Contains(strings.ToLower(skill.Name), v) {
				score += 0.30
				matchedVerb = true
				reasons = append(reasons, "verbo compatible: "+v)
				break
			}
		}
	}

	// 3. Entidad compatible (+0.25)
	if len(extractedSlots) > 0 {
		score += 0.25
		reasons = append(reasons, "entidad compatible")
	}

	// 4. Herramienta registrada disponible (+0.20)
	if toolExists != nil {
		if toolExists(skill.Name) {
			score += 0.20
			reasons = append(reasons, "tool registrada disponible")
		}
	}

	// 5. Penalización por Negative Trigger (-0.50)
	for _, neg := range skill.NegativeTriggers {
		negLower := strings.ToLower(neg)
		if negLower != "" && strings.Contains(userInputLower, negLower) {
			score -= 0.50
			reasons = append(reasons, "negative trigger: "+neg)
		}
	}

	// 6. Penalización por falta de Slot obligatorio (-0.30)
	hasAllMandatory := true
	for _, s := range mandatorySlots {
		if val, exists := extractedSlots[s]; !exists || val == nil || val == "" {
			hasAllMandatory = false
			break
		}
	}
	if !hasAllMandatory {
		score -= 0.30
		reasons = append(reasons, "falta slot obligatorio")
	}

	// Si no tiene coincidencia de trigger ni verbo compatible ni slots, la confianza es 0.0
	if !matchedTrigger && !matchedVerb && len(extractedSlots) == 0 {
		return 0.0, "sin coincidencias de trigger, verbo ni slots"
	}

	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score, strings.Join(reasons, ", ")
}

