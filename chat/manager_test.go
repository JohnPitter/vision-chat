package chat

import (
	"testing"

	"vision-chat/llama"
)

func TestNewManager(t *testing.T) {
	m := NewManager("You are a vision assistant.")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 system message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	content, ok := msgs[0].Content.(string)
	if !ok || content != "You are a vision assistant." {
		t.Errorf("unexpected system prompt: %v", msgs[0].Content)
	}
}

func TestManager_AddUserTextMessage(t *testing.T) {
	m := NewManager("system prompt")
	m.AddUserMessage("Hello")

	msgs := m.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].Role != "user" {
		t.Errorf("expected user role, got %s", msgs[1].Role)
	}
	content, ok := msgs[1].Content.(string)
	if !ok || content != "Hello" {
		t.Errorf("unexpected content: %v", msgs[1].Content)
	}
}

func TestManager_AddUserVisionMessage(t *testing.T) {
	m := NewManager("system prompt")
	m.AddUserVisionMessage("What is this?", "data:image/jpeg;base64,abc123")

	msgs := m.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	parts, ok := msgs[1].Content.([]llama.ContentPart)
	if !ok {
		t.Fatalf("expected []ContentPart, got %T", msgs[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "What is this?" {
		t.Errorf("unexpected text part: %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
		t.Errorf("unexpected image part: %+v", parts[1])
	}
}

func TestManager_AddAssistantMessage(t *testing.T) {
	m := NewManager("system prompt")
	m.AddUserMessage("Hi")
	m.AddAssistantMessage("Hello! How can I help?")

	msgs := m.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[2].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", msgs[2].Role)
	}
}

func TestManager_ClearHistory(t *testing.T) {
	m := NewManager("system prompt")
	m.AddUserMessage("msg1")
	m.AddAssistantMessage("resp1")
	m.AddUserMessage("msg2")

	m.Clear()
	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("after clear, expected 1 (system) message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Error("system message should be preserved after clear")
	}
}

func TestManager_MaxHistory(t *testing.T) {
	m := NewManagerWithMaxHistory("system prompt", 3)
	for i := 0; i < 10; i++ {
		m.AddUserMessage("user msg")
		m.AddAssistantMessage("assistant resp")
	}

	msgs := m.Messages()
	// System (1) + last 3 pairs (6) = 7 max
	if len(msgs) > 7 {
		t.Errorf("expected at most 7 messages with max history 3, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Error("first message must be system")
	}
}

func TestManager_ConversationRoundTrip(t *testing.T) {
	m := NewManager("You are helpful.")
	m.AddUserMessage("Hi")
	m.AddAssistantMessage("Hello!")
	m.AddUserVisionMessage("What is this?", "data:image/jpeg;base64,abc")
	m.AddAssistantMessage("I see an image.")

	msgs := m.Messages()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	roles := []string{"system", "user", "assistant", "user", "assistant"}
	for i, msg := range msgs {
		if msg.Role != roles[i] {
			t.Errorf("msg[%d]: expected role %s, got %s", i, roles[i], msg.Role)
		}
	}
}
