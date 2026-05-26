package intent

// IntentCandidate representa una intención clasificada por el router.
type IntentCandidate struct {
	Intent     string                 `json:"intent"`
	ToolName   string                 `json:"tool_name"`
	SkillName  string                 `json:"skill_name,omitempty"`
	Confidence float64                `json:"confidence"`
	Reason     string                 `json:"reason,omitempty"`
	Slots      map[string]interface{} `json:"slots,omitempty"`
	RiskLevel  string                 `json:"risk_level,omitempty"`
}
