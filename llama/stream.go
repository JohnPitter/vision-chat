package llama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamChunk represents a single token chunk from streaming response.
type StreamChunk struct {
	Content      string
	FinishReason string
}

// streamDelta is the delta object in streaming responses.
type streamDelta struct {
	Content string `json:"content"`
}

// streamChoice is a choice in streaming responses.
type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// streamResponse is the SSE data payload.
type streamResponse struct {
	ID      string         `json:"id"`
	Choices []streamChoice `json:"choices"`
}

// StreamChatCompletion sends a streaming chat completion request.
// The callback is invoked for each token chunk received.
func (c *Client) StreamChatCompletion(ctx context.Context, messages []ChatMessage, onChunk func(StreamChunk)) error {
	reqBody := ChatRequest{
		Model:    "local-model",
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for streaming
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var sr streamResponse
		if err := json.Unmarshal([]byte(data), &sr); err != nil {
			continue // skip malformed chunks
		}

		for _, choice := range sr.Choices {
			chunk := StreamChunk{
				Content: choice.Delta.Content,
			}
			if choice.FinishReason != nil {
				chunk.FinishReason = *choice.FinishReason
			}
			onChunk(chunk)
		}
	}

	if err := scanner.Err(); err != nil {
		// Check if it's a context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}
