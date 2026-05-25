package intent

import (
	"database/sql"
	"sort"
	"strings"

	"rbot/internal/skills"
)

type IntentCandidate struct {
	Intent     string
	SkillName  string
	Confidence float64
	Reason     string
	Slots      map[string]string
	RiskLevel  string
}

type Router struct {
	db *sql.DB
}

func NewRouter(db *sql.DB) *Router {
	return &Router{
		db: db,
	}
}

// Match evalúa el input del usuario contra todas las habilidades habilitadas
// y retorna una lista de candidatos ordenados por confianza (score).
func (r *Router) Match(userInput string) []IntentCandidate {
	var candidates []IntentCandidate

	allSkills, err := skills.GetAllEnabledSkills(r.db)
	if err != nil || len(allSkills) == 0 {
		return candidates
	}

	userInputLower := strings.ToLower(strings.TrimSpace(userInput))

	for _, skill := range allSkills {
		score := 0.0
		reason := "No match"

		// 1. Evaluar Voice Triggers (Trigger exacto o parcial fuerte)
		for _, trigger := range skill.VoiceTriggers {
			triggerLower := strings.ToLower(trigger)
			if triggerLower != "" && strings.Contains(userInputLower, triggerLower) {
				score += 0.60
				reason = "Match en voice trigger: " + trigger
				break // Solo sumar una vez por voice triggers
			}
		}

		// 2. Evaluar Negative Triggers (Penalización severa)
		for _, negTrigger := range skill.NegativeTriggers {
			negLower := strings.ToLower(negTrigger)
			if negLower != "" && strings.Contains(userInputLower, negLower) {
				score -= 0.50
				reason += " | Penalizado por negative trigger: " + negTrigger
			}
		}

		// 3. Evaluar verbos principales o palabras clave generales en el nombre/descripción
		words := strings.Fields(userInputLower)
		matchedWords := 0
		for _, word := range words {
			if len(word) > 3 {
				if strings.Contains(strings.ToLower(skill.Name), word) {
					matchedWords++
				}
				if strings.Contains(strings.ToLower(skill.Description), word) {
					matchedWords++
				}
			}
		}
		if matchedWords > 0 {
			score += float64(matchedWords) * 0.10
			if score > 1.0 { // Cap score addition for words
				score = 0.95
			}
		}

		if score > 0 {
			candidates = append(candidates, IntentCandidate{
				Intent:     skill.Name, // TODO: Usar el intent específico de la skill si está definido
				SkillName:  skill.Name,
				Confidence: score,
				Reason:     reason,
				Slots:      make(map[string]string),
				RiskLevel:  skill.RiskLevel,
			})
		}
	}

	// Ordenar candidatos por confianza de mayor a menor
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})

	return candidates
}

// TopN retorna los mejores N candidatos
func TopN(candidates []IntentCandidate, n int) []IntentCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	if len(candidates) < n {
		return candidates
	}
	return candidates[:n]
}

// Top retorna el mejor candidato, o uno vacío si no hay
func Top(candidates []IntentCandidate) IntentCandidate {
	if len(candidates) == 0 {
		return IntentCandidate{Confidence: 0.0}
	}
	return candidates[0]
}
