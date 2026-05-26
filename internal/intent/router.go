package intent

import (
	"database/sql"
	"sort"

	"rbot/internal/skills"
)

type Router struct {
	db         *sql.DB
	ToolExists func(string) bool
	WakeWords  []string
}

func NewRouter(db *sql.DB) *Router {
	return &Router{
		db: db,
		WakeWords: []string{"oye ronaldo", "ey ronaldo", "go ronaldo", "hola ronaldo", "ronald", "rbot"},
	}
}

// SetToolExists permite configurar una función callback para verificar si una herramienta existe
// en el registro, evitando así dependencias circulares con el paquete executor.
func (r *Router) SetToolExists(fn func(string) bool) {
	r.ToolExists = fn
}

// SetWakeWords configura las wake words personalizadas.
func (r *Router) SetWakeWords(ww []string) {
	r.WakeWords = ww
}

// Match evalúa el input del usuario contra todas las habilidades habilitadas
// y retorna una lista de candidatos ordenados por confianza (score).
func (r *Router) Match(userInput string) []IntentCandidate {
	var candidates []IntentCandidate

	// 1. Normalizar el input (corregir errores fonéticos, wake words, etc.)
	normalized := Normalize(userInput, r.WakeWords)

	// 2. Obtener todas las habilidades habilitadas
	allSkills, err := skills.GetAllEnabledSkills(r.db)
	if err != nil || len(allSkills) == 0 {
		return candidates
	}

	for _, skill := range allSkills {
		// 3. Extraer slots
		extractedSlots := ExtractSlots(skill.Name, normalized)

		// 4. Obtener slots requeridos/obligatorios
		mandatory := skill.RequiredSlots[skill.Name]

		// 5. Calcular puntuación heurística
		score, reason := CalculateScore(normalized, skill, r.ToolExists, mandatory, extractedSlots)

		if score > 0 {
			candidates = append(candidates, IntentCandidate{
				Intent:     skill.Name,
				ToolName:   skill.Name,
				SkillName:  skill.Name,
				Confidence: score,
				Reason:     reason,
				Slots:      extractedSlots,
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
