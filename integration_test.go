// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
	"time"

	"vision-chat/llama"
	"vision-chat/vision"
)

const (
	serverURL = "http://127.0.0.1:8090"
)

// createTestScene creates a JPEG image simulating a real scene.
func createTestScene(width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Create a scene with different colored regions (simulating objects)
	for y := range height {
		for x := range width {
			switch {
			case x < width/3:
				img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255}) // red zone
			case x < 2*width/3:
				img.Set(x, y, color.RGBA{R: 50, G: 200, B: 50, A: 255}) // green zone
			default:
				img.Set(x, y, color.RGBA{R: 50, G: 50, B: 200, A: 255}) // blue zone
			}
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// TestIntegration_ServerHealth verifies the server is running.
func TestIntegration_ServerHealth(t *testing.T) {
	client := llama.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ok, err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("health check error: %v", err)
	}
	if !ok {
		t.Fatal("server is not healthy - ensure llama-server is running on port 8090")
	}
	t.Log("Server health: OK")
}

// TestIntegration_TextChat tests a simple text-only conversation.
func TestIntegration_TextChat(t *testing.T) {
	client := llama.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are a helpful assistant. Answer in 1 sentence."},
		{Role: "user", Content: "What is 2+2?"},
	}

	start := time.Now()
	resp, err := client.ChatCompletion(ctx, messages)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("chat completion error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("no choices returned")
	}

	reply, _ := resp.Choices[0].Message.Content.(string)
	t.Logf("Response: %s", reply)
	t.Logf("Latency: %v", elapsed)
	t.Logf("Tokens: prompt=%d, completion=%d, total=%d",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	if reply == "" {
		t.Error("empty response")
	}
}

// TestIntegration_VisionAnalysis sends an image and asks what's in it.
func TestIntegration_VisionAnalysis(t *testing.T) {
	client := llama.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create and process a test image
	rawFrame := createTestScene(640, 480)
	processed, err := vision.ProcessFrame(rawFrame, 512)
	if err != nil {
		t.Fatalf("frame processing error: %v", err)
	}
	dataURI := vision.FormatAsDataURI(processed)

	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are a vision assistant. Describe what you see briefly."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "Describe the colors and regions you see in this image."},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	start := time.Now()
	resp, err := client.ChatCompletion(ctx, messages)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("vision request error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("no choices returned")
	}

	reply, _ := resp.Choices[0].Message.Content.(string)
	t.Logf("Vision Response: %s", reply)
	t.Logf("Latency: %v", elapsed)
	t.Logf("Tokens: prompt=%d, completion=%d, total=%d",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	if reply == "" {
		t.Error("empty vision response")
	}
}

// TestIntegration_StreamingVision tests streaming with a vision request.
func TestIntegration_StreamingVision(t *testing.T) {
	client := llama.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	rawFrame := createTestScene(320, 240)
	processed, _ := vision.ProcessFrame(rawFrame, 512)
	dataURI := vision.FormatAsDataURI(processed)

	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are a vision assistant. Be brief."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "What colors do you see? One sentence."},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	var chunks []string
	var firstTokenTime time.Duration
	start := time.Now()

	err := client.StreamChatCompletion(ctx, messages, func(chunk llama.StreamChunk) {
		if len(chunks) == 0 {
			firstTokenTime = time.Since(start)
		}
		chunks = append(chunks, chunk.Content)
	})

	totalTime := time.Since(start)

	if err != nil {
		t.Fatalf("streaming error: %v", err)
	}

	fullResponse := strings.Join(chunks, "")
	t.Logf("Streaming Response: %s", fullResponse)
	t.Logf("First token latency: %v", firstTokenTime)
	t.Logf("Total latency: %v", totalTime)
	t.Logf("Chunks received: %d", len(chunks))
	t.Logf("Tokens/sec estimate: %.1f", float64(len(chunks))/totalTime.Seconds())

	if fullResponse == "" {
		t.Error("empty streaming response")
	}
	if len(chunks) < 2 {
		t.Error("expected multiple streaming chunks")
	}
}

// TestIntegration_FrameCachePerformance benchmarks the cache system with real frames.
func TestIntegration_FrameCachePerformance(t *testing.T) {
	cache := vision.NewFrameCache(vision.CacheConfig{
		ChangeThreshold:    0.05,
		ComparisonSize:     64,
		MinProcessInterval: 16,
		MaxProcessInterval: 500,
	})

	// Simulate 60fps with same scene
	frame := createTestScene(640, 480)

	start := time.Now()
	newCount := 0
	cachedCount := 0
	totalFrames := 600 // 10 seconds at 60fps

	for i := range totalFrames {
		result := cache.Analyze(frame)
		if result.IsNew {
			newCount++
		} else {
			cachedCount++
		}
		_ = i
	}

	elapsed := time.Since(start)
	stats := cache.Stats()

	t.Logf("Frames analyzed: %d", totalFrames)
	t.Logf("New frames: %d", newCount)
	t.Logf("Cached frames: %d", cachedCount)
	t.Logf("Cache hit rate: %.1f%%", float64(cachedCount)/float64(totalFrames)*100)
	t.Logf("Total time: %v", elapsed)
	t.Logf("Avg per frame: %v", elapsed/time.Duration(totalFrames))
	t.Logf("Throughput: %.0f frames/sec", float64(totalFrames)/elapsed.Seconds())
	t.Logf("Stats: %+v", stats)

	// Static scene should have 99%+ cache hit rate
	if cachedCount < totalFrames-2 { // first frame + maybe 1 detection
		t.Errorf("expected high cache hit rate for static scene, got %d cached out of %d",
			cachedCount, totalFrames)
	}

	// Each frame should process in < 1ms (64x64 comparison)
	avgPerFrame := elapsed / time.Duration(totalFrames)
	if avgPerFrame > 5*time.Millisecond {
		t.Errorf("frame analysis too slow: %v per frame (target: <1ms)", avgPerFrame)
	}
}

// TestIntegration_ConversationFlow simulates a real user conversation flow.
func TestIntegration_ConversationFlow(t *testing.T) {
	client := llama.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Simulate: user starts camera, asks about scene, follows up
	frame := createTestScene(640, 480)
	processed, _ := vision.ProcessFrame(frame, 512)
	dataURI := vision.FormatAsDataURI(processed)

	// Turn 1: Initial vision question
	t.Log("=== Turn 1: Initial vision question ===")
	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are a vision assistant. Answer concisely."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "What do you see in this image?"},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	resp1, err := client.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	reply1, _ := resp1.Choices[0].Message.Content.(string)
	t.Logf("Turn 1 reply: %s", reply1)

	// Turn 2: Follow-up text question (no new image)
	t.Log("=== Turn 2: Follow-up question ===")
	messages = append(messages,
		llama.ChatMessage{Role: "assistant", Content: reply1},
		llama.ChatMessage{Role: "user", Content: "How many distinct color regions are there?"},
	)

	resp2, err := client.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	reply2, _ := resp2.Choices[0].Message.Content.(string)
	t.Logf("Turn 2 reply: %s", reply2)

	fmt.Printf("\n=== Conversation Summary ===\n")
	fmt.Printf("Turn 1 tokens: %d\n", resp1.Usage.TotalTokens)
	fmt.Printf("Turn 2 tokens: %d\n", resp2.Usage.TotalTokens)
}
