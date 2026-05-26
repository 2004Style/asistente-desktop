package planner

import (
	"fmt"
)

// ResolveDependencies ordena los pasos del plan según sus dependencias (orden topológico).
// Retorna error si se detecta una dependencia circular.
func ResolveDependencies(steps []PlanStep) ([]PlanStep, error) {
	stepsMap := make(map[string]PlanStep)
	for _, step := range steps {
		stepsMap[step.ID] = step
	}

	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, step := range steps {
		inDegree[step.ID] = 0
	}

	for _, step := range steps {
		for _, dep := range step.DependsOn {
			if _, exists := stepsMap[dep]; !exists {
				// Si la dependencia no existe en los pasos del plan, se considera resuelta/inexistente
				continue
			}
			adj[dep] = append(adj[dep], step.ID)
			inDegree[step.ID]++
		}
	}

	var queue []string
	// Mantener el orden original estable para pasos sin dependencias iniciales
	for _, step := range steps {
		if inDegree[step.ID] == 0 {
			queue = append(queue, step.ID)
		}
	}

	var ordered []PlanStep
	visitedCount := 0

	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]

		ordered = append(ordered, stepsMap[currID])
		visitedCount++

		for _, neighbor := range adj[currID] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visitedCount != len(steps) {
		return nil, fmt.Errorf("detectada dependencia circular o no resuelta en el plan")
	}

	return ordered, nil
}
