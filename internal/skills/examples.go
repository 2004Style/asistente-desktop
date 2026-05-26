package skills

type ExamplePositive struct {
	Input  string                 `yaml:"input" json:"input"`
	Intent string                 `yaml:"intent" json:"intent"`
	Tool   string                 `yaml:"tool" json:"tool"`
	Args   map[string]interface{} `yaml:"args" json:"args"`
}

type ExampleNegative struct {
	Input  string `yaml:"input" json:"input"`
	Reason string `yaml:"reason" json:"reason"`
}
