package llama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8090")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.BaseURL() != "http://localhost:8090" {
		t.Errorf("expected base URL http://localhost:8090, got %s", c.BaseURL())
	}
}

func TestClient_SendTextMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected application/json, got %s", contentType)
		}

		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("expected role user, got %s", req.Messages[0].Role)
		}

		resp := ChatResponse{
			ID: "test-1",
			Choices: []Choice{
				{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hello!"}, FinishReason: "stop"},
			},
			Usage: Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ctx := context.Background()
	messages := []ChatMessage{
		{Role: "user", Content: "Hi"},
	}

	resp, err := c.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		t.Fatal("expected string content")
	}
	if content != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", content)
	}
}

func TestClient_SendVisionMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		json.Unmarshal(body, &req)

		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}

		// Content should be an array (multimodal)
		contentBytes, _ := json.Marshal(req.Messages[0].Content)
		var parts []ContentPart
		if err := json.Unmarshal(contentBytes, &parts); err != nil {
			t.Fatalf("expected array content for vision: %v", err)
		}
		if len(parts) != 2 {
			t.Fatalf("expected 2 content parts, got %d", len(parts))
		}
		if parts[0].Type != "text" {
			t.Errorf("expected first part type 'text', got '%s'", parts[0].Type)
		}
		if parts[1].Type != "image_url" {
			t.Errorf("expected second part type 'image_url', got '%s'", parts[1].Type)
		}
		if !strings.HasPrefix(parts[1].ImageURL.URL, "data:image/jpeg;base64,") {
			t.Error("expected base64 data URI")
		}

		resp := ChatResponse{
			ID: "test-vision-1",
			Choices: []Choice{
				{Index: 0, Message: ChatMessage{Role: "assistant", Content: "I see a cat."}, FinishReason: "stop"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ctx := context.Background()

	parts := []ContentPart{
		{Type: "text", Text: "What do you see?"},
		{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/jpeg;base64,/9j/4AAQ=="}},
	}
	messages := []ChatMessage{
		{Role: "user", Content: parts},
	}

	resp, err := c.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	if content != "I see a cat." {
		t.Errorf("expected 'I see a cat.', got '%s'", content)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.ChatCompletion(ctx, []ChatMessage{{Role: "user", Content: "Hi"}})
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "model loading failed"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	_, err := c.ChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestClient_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ok, err := c.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected health check to return true")
	}
}

func TestClient_HealthCheckDown(t *testing.T) {
	c := NewClient("http://127.0.0.1:19999")
	ok, _ := c.HealthCheck(context.Background())
	if ok {
		t.Error("expected health check to return false for down server")
	}
}
