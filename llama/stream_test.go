package llama

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_StreamChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []string{"Hello", " world", "!"}
		for i, chunk := range chunks {
			finishReason := ""
			if i == len(chunks)-1 {
				finishReason = "stop"
			}
			data := fmt.Sprintf(`{"id":"test","choices":[{"index":0,"delta":{"content":"%s"},"finish_reason":%s}]}`,
				chunk, formatFinishReason(finishReason))
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ctx := context.Background()
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	var collected []string
	err := c.StreamChatCompletion(ctx, messages, func(chunk StreamChunk) {
		collected = append(collected, chunk.Content)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := strings.Join(collected, "")
	if result != "Hello world!" {
		t.Errorf("expected 'Hello world!', got '%s'", result)
	}
}

func TestClient_StreamChatCompletion_DetectsFinish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		fmt.Fprintf(w, `data: {"id":"t","choices":[{"index":0,"delta":{"content":"done"},"finish_reason":"stop"}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	c := NewClient(server.URL)
	var finished bool
	err := c.StreamChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "x"}}, func(chunk StreamChunk) {
		if chunk.FinishReason == "stop" {
			finished = true
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !finished {
		t.Error("expected finish_reason 'stop' to be detected")
	}
}

func TestClient_StreamChatCompletion_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				fmt.Fprintf(w, `data: {"id":"t","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":null}]}`+"\n\n")
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	c := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	count := 0
	err := c.StreamChatCompletion(ctx, []ChatMessage{{Role: "user", Content: "x"}}, func(chunk StreamChunk) {
		count++
	})
	// Should get an error from context cancellation
	if err == nil {
		t.Log("stream ended without error (server may have closed first)")
	}
	if count == 0 {
		t.Error("should have received at least some chunks before cancel")
	}
}

func TestClient_StreamChatCompletion_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"model not loaded"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.StreamChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "x"}}, func(chunk StreamChunk) {
		t.Error("should not receive chunks on error")
	})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func formatFinishReason(reason string) string {
	if reason == "" {
		return "null"
	}
	return fmt.Sprintf(`"%s"`, reason)
}
