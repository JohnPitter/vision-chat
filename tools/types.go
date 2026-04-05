package tools

// Tool represents an executable tool the AI agent can use.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Dangerous   bool   `json:"dangerous"` // requires user confirmation
}

// ToolCall represents a parsed tool invocation from the AI response.
type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// ToolResult holds the outcome of a tool execution.
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
