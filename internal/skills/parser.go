package skills

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseSkillMd(path string) (*SkillMetadata, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var frontmatterLines []string
	var bodyLines []string
	dashCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			dashCount++
			continue
		}

		if dashCount == 1 {
			frontmatterLines = append(frontmatterLines, line)
		} else if dashCount >= 2 {
			bodyLines = append(bodyLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", err
	}

	yamlContent := strings.Join(frontmatterLines, "\n")
	bodyContent := strings.Join(bodyLines, "\n")

	meta := &SkillMetadata{
		Status:           "disabled",
		Permissions:      []string{},
		VoiceTriggers:    []string{},
		NegativeTriggers: []string{},
		Intents:          []string{},
		Tools:            []string{},
		RequiredSlots:    make(map[string][]string),
	}

	if len(frontmatterLines) > 0 {
		if err := yaml.Unmarshal([]byte(yamlContent), meta); err != nil {
			return nil, "", fmt.Errorf("error parseando YAML en %s: %w", path, err)
		}
	}

	// Limpieza de strings
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	if meta.RiskLevel == "" {
		meta.RiskLevel = "medium"
	}
	if meta.Status == "" {
		meta.Status = "disabled"
	}

	return meta, bodyContent, nil
}
