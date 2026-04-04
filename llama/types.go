package llama

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentPart
}

// ContentPart represents a multimodal content element.
type ContentPart struct {
	Type     string    `json:"type"`               // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL wraps the base64 data URI for vision.
type ImageURL struct {
	URL string `json:"url"` // "data:image/jpeg;base64,..."
}

// ChatRequest is the POST body for /v1/chat/completions.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

// ChatResponse is the response from /v1/chat/completions.
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage contains token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ServerConfig holds configuration for the llama-server subprocess.
type ServerConfig struct {
	ExecutablePath string // Path to llama-server executable
	ModelPath      string // Path to the GGUF model file (ignored if HFRepo is set)
	MMProjPath     string // Path to multimodal projector (auto-downloaded with HFRepo)
	HFRepo         string // Hugging Face repo (e.g. "bartowski/Llama-3.2-11B-Vision-Instruct-GGUF:Q4_K_M")
	Host           string // Default: "127.0.0.1"
	Port           int    // Default: 8090
	NGPULayers     int    // Default: 99 (offload all to GPU)
	CtxSize        int    // Context size (default: 4096)
	FlashAttn      bool   // Enable flash attention (default: true)
}
