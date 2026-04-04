package chat

import (
	"sync"

	"vision-chat/llama"
)

// Manager manages conversation history with a system prompt.
type Manager struct {
	mu           sync.Mutex
	systemPrompt string
	history      []llama.ChatMessage
	maxPairs     int // 0 = unlimited
}

// NewManager creates a new conversation manager with a system prompt.
func NewManager(systemPrompt string) *Manager {
	return &Manager{
		systemPrompt: systemPrompt,
		history:      []llama.ChatMessage{},
	}
}

// NewManagerWithMaxHistory creates a manager that keeps at most maxPairs user/assistant pairs.
func NewManagerWithMaxHistory(systemPrompt string, maxPairs int) *Manager {
	return &Manager{
		systemPrompt: systemPrompt,
		history:      []llama.ChatMessage{},
		maxPairs:     maxPairs,
	}
}

// AddUserMessage adds a text-only user message.
func (m *Manager) AddUserMessage(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, llama.ChatMessage{
		Role:    "user",
		Content: text,
	})
	m.trimHistory()
}

// AddUserVisionMessage adds a multimodal user message with text and image.
func (m *Manager) AddUserVisionMessage(text string, imageDataURI string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	parts := []llama.ContentPart{
		{Type: "text", Text: text},
		{Type: "image_url", ImageURL: &llama.ImageURL{URL: imageDataURI}},
	}
	m.history = append(m.history, llama.ChatMessage{
		Role:    "user",
		Content: parts,
	})
	m.trimHistory()
}

// AddAssistantMessage adds an assistant response.
func (m *Manager) AddAssistantMessage(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, llama.ChatMessage{
		Role:    "assistant",
		Content: text,
	})
	m.trimHistory()
}

// Messages returns the full message list including the system prompt.
func (m *Manager) Messages() []llama.ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := make([]llama.ChatMessage, 0, len(m.history)+1)
	msgs = append(msgs, llama.ChatMessage{
		Role:    "system",
		Content: m.systemPrompt,
	})
	msgs = append(msgs, m.history...)
	return msgs
}

// Clear resets conversation history, preserving the system prompt.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = []llama.ChatMessage{}
}

// trimHistory enforces maxPairs limit. Must be called with lock held.
func (m *Manager) trimHistory() {
	if m.maxPairs <= 0 {
		return
	}
	maxMessages := m.maxPairs * 2
	if len(m.history) > maxMessages {
		m.history = m.history[len(m.history)-maxMessages:]
	}
}
