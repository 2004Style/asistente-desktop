package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"rbot/internal/executor"
)

type ClipboardCopyTool struct{}

func NewClipboardCopyTool() *ClipboardCopyTool {
	return &ClipboardCopyTool{}
}

func (t *ClipboardCopyTool) Name() string { return "system.clipboard_copy" }
func (t *ClipboardCopyTool) Description() string {
	return "Copia un texto especificado al portapapeles del sistema."
}
func (t *ClipboardCopyTool) Category() string  { return "system" }
func (t *ClipboardCopyTool) RiskLevel() string { return "low" }
func (t *ClipboardCopyTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Texto a copiar al portapapeles.",
			},
		},
		"required": []string{"text"},
	}
}

func (t *ClipboardCopyTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	text, _ := args["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("el argumento 'text' es obligatorio")
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd = exec.CommandContext(ctx, "wl-copy")
	} else if _, err := exec.LookPath("xclip"); err == nil {
		cmd = exec.CommandContext(ctx, "xclip", "-selection", "clipboard")
	} else {
		return nil, fmt.Errorf("no se encontró ningún comando de portapapeles ('wl-copy' o 'xclip')")
	}

	cmd.Stdin = strings.NewReader(text)
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error al copiar al portapapeles: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       "Texto copiado al portapapeles correctamente.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
