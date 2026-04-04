//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"strings"
	"testing"
	"time"

	"vision-chat/chat"
	"vision-chat/llama"
	"vision-chat/vision"
)

const testServerURL = "http://127.0.0.1:8090"

// ============================================================
// HELPERS - Create realistic test images
// ============================================================

// createDesktopScreenshot simulates a desktop screenshot with windows, taskbar, icons
func createDesktopScreenshot() string {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))

	// Desktop background - blue gradient
	for y := range 1080 {
		blue := uint8(100 + float64(y)/1080*100)
		for x := range 1920 {
			img.Set(x, y, color.RGBA{R: 30, G: 50, B: blue, A: 255})
		}
	}

	// Taskbar at bottom - dark gray
	drawRect(img, 0, 1040, 1920, 1080, color.RGBA{R: 40, G: 40, B: 45, A: 255})

	// Window 1 - code editor (left side)
	drawRect(img, 50, 30, 900, 700, color.RGBA{R: 30, G: 30, B: 30, A: 255})   // bg
	drawRect(img, 50, 30, 900, 60, color.RGBA{R: 60, G: 60, B: 65, A: 255})    // title bar
	drawRect(img, 60, 70, 200, 700, color.RGBA{R: 35, G: 35, B: 40, A: 255})   // sidebar
	drawRect(img, 210, 80, 890, 82, color.RGBA{R: 100, G: 180, B: 100, A: 255}) // code line green
	drawRect(img, 210, 100, 600, 102, color.RGBA{R: 200, G: 150, B: 80, A: 255}) // code line yellow

	// Window 2 - browser (right side)
	drawRect(img, 920, 80, 1870, 800, color.RGBA{R: 250, G: 250, B: 250, A: 255}) // white page
	drawRect(img, 920, 80, 1870, 120, color.RGBA{R: 230, G: 230, B: 235, A: 255}) // toolbar
	drawRect(img, 950, 140, 1840, 300, color.RGBA{R: 200, G: 220, B: 255, A: 255}) // hero section

	// Terminal window at bottom
	drawRect(img, 50, 720, 900, 1020, color.RGBA{R: 10, G: 10, B: 15, A: 255})
	drawRect(img, 50, 720, 900, 745, color.RGBA{R: 50, G: 50, B: 55, A: 255}) // title bar
	drawRect(img, 60, 760, 400, 762, color.RGBA{R: 0, G: 255, B: 0, A: 255})   // green text

	return encodeAsJPEG(img)
}

// createPersonAtDesk simulates a webcam view of a person at a desk
func createPersonAtDesk() string {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	// Room background - beige wall
	for y := range 480 {
		for x := range 640 {
			img.Set(x, y, color.RGBA{R: 210, G: 195, B: 170, A: 255})
		}
	}

	// Desk - brown
	drawRect(img, 0, 350, 640, 480, color.RGBA{R: 120, G: 80, B: 50, A: 255})

	// Monitor on desk
	drawRect(img, 200, 150, 450, 340, color.RGBA{R: 20, G: 20, B: 25, A: 255})  // screen
	drawRect(img, 300, 340, 350, 360, color.RGBA{R: 80, G: 80, B: 80, A: 255})  // stand
	drawRect(img, 220, 170, 430, 320, color.RGBA{R: 50, G: 100, B: 200, A: 255}) // screen content

	// Keyboard
	drawRect(img, 220, 380, 430, 420, color.RGBA{R: 60, G: 60, B: 65, A: 255})

	// Coffee mug
	drawRect(img, 500, 370, 540, 420, color.RGBA{R: 200, G: 200, B: 200, A: 255})

	// Person silhouette (head + shoulders)
	drawCircle(img, 320, 100, 50, color.RGBA{R: 180, G: 140, B: 110, A: 255}) // head
	drawRect(img, 250, 150, 390, 300, color.RGBA{R: 50, G: 50, B: 120, A: 255}) // shirt

	return encodeAsJPEG(img)
}

// createChangedDesktop creates a slightly different desktop (simulating screen change)
func createChangedDesktop() string {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))

	// Same background
	for y := range 1080 {
		blue := uint8(100 + float64(y)/1080*100)
		for x := range 1920 {
			img.Set(x, y, color.RGBA{R: 30, G: 50, B: blue, A: 255})
		}
	}

	// Taskbar
	drawRect(img, 0, 1040, 1920, 1080, color.RGBA{R: 40, G: 40, B: 45, A: 255})

	// Same window but with a NOTIFICATION POPUP (the change)
	drawRect(img, 50, 30, 900, 700, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	drawRect(img, 50, 30, 900, 60, color.RGBA{R: 60, G: 60, B: 65, A: 255})

	// Notification popup - bright orange (the key difference)
	drawRect(img, 1400, 50, 1850, 150, color.RGBA{R: 249, G: 115, B: 22, A: 255})
	drawRect(img, 1410, 60, 1840, 80, color.RGBA{R: 255, G: 255, B: 255, A: 255}) // text line

	return encodeAsJPEG(img)
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	draw.Draw(img, image.Rect(x1, y1, x2, y2), &image.Uniform{c}, image.Point{}, draw.Src)
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			if math.Sqrt(dx*dx+dy*dy) <= float64(r) {
				img.Set(x, y, c)
			}
		}
	}
}

func encodeAsJPEG(img *image.RGBA) string {
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// ============================================================
// TEST 1: Simula usuário ligando webcam e perguntando
// ============================================================
func TestE2E_WebcamConversation(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("=== Simulando: Usuário liga webcam e pergunta o que vê ===")

	// Capture "webcam" frame
	webcamFrame := createPersonAtDesk()
	processed, err := vision.ProcessFrame(webcamFrame, 512)
	if err != nil {
		t.Fatalf("ProcessFrame error: %v", err)
	}

	t.Logf("Frame processado: original → 512px max dimension")

	// User asks: "What do you see?"
	dataURI := vision.FormatAsDataURI(processed)
	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are a vision assistant. Describe what you see concisely."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "What do you see in this webcam image?"},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	start := time.Now()
	resp, err := client.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}

	reply, _ := resp.Choices[0].Message.Content.(string)
	elapsed := time.Since(start)

	t.Logf("User: 'What do you see in this webcam image?'")
	t.Logf("AI: %s", reply)
	t.Logf("Latency: %v | Tokens: %d", elapsed, resp.Usage.TotalTokens)

	if reply == "" {
		t.Error("Empty response")
	}

	// Follow-up question without new image
	t.Log("\n=== Follow-up: Pergunta sobre detalhes ===")
	messages = append(messages,
		llama.ChatMessage{Role: "assistant", Content: reply},
		llama.ChatMessage{Role: "user", Content: "Is there a coffee mug visible? Where exactly?"},
	)

	start = time.Now()
	resp2, err := client.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("Follow-up error: %v", err)
	}

	reply2, _ := resp2.Choices[0].Message.Content.(string)
	t.Logf("User: 'Is there a coffee mug visible? Where exactly?'")
	t.Logf("AI: %s", reply2)
	t.Logf("Latency: %v | Tokens: %d", time.Since(start), resp2.Usage.TotalTokens)
}

// ============================================================
// TEST 2: Simula screen share - desktop com IDE e browser
// ============================================================
func TestE2E_ScreenShareAnalysis(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("=== Simulando: Usuário compartilha tela do desktop ===")

	desktopFrame := createDesktopScreenshot()
	processed, err := vision.ProcessFrame(desktopFrame, 512)
	if err != nil {
		t.Fatalf("ProcessFrame error: %v", err)
	}

	dataURI := vision.FormatAsDataURI(processed)
	messages := []llama.ChatMessage{
		{Role: "system", Content: "You are analyzing a screen share. Describe what applications and windows are visible."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "I'm sharing my screen. What applications can you see open on my desktop?"},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	start := time.Now()
	resp, err := client.ChatCompletion(ctx, messages)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}

	reply, _ := resp.Choices[0].Message.Content.(string)
	t.Logf("User: 'What applications can you see open on my desktop?'")
	t.Logf("AI: %s", reply)
	t.Logf("Latency: %v | Tokens: %d", time.Since(start), resp.Usage.TotalTokens)
}

// ============================================================
// TEST 3: Streaming - resposta aparece token a token
// ============================================================
func TestE2E_StreamingResponse(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("=== Simulando: Streaming response (token a token) ===")

	frame := createPersonAtDesk()
	processed, _ := vision.ProcessFrame(frame, 512)
	dataURI := vision.FormatAsDataURI(processed)

	messages := []llama.ChatMessage{
		{Role: "system", Content: "Describe what you see in detail. Be thorough."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "Describe everything you see in this image."},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	var chunks []string
	var firstTokenLatency time.Duration
	start := time.Now()

	err := client.StreamChatCompletion(ctx, messages, func(chunk llama.StreamChunk) {
		if len(chunks) == 0 {
			firstTokenLatency = time.Since(start)
			t.Logf("⚡ First token at: %v", firstTokenLatency)
		}
		chunks = append(chunks, chunk.Content)
		// Print dots to show streaming progress
		if len(chunks)%10 == 0 {
			fmt.Printf(".")
		}
	})
	fmt.Println()

	totalTime := time.Since(start)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	fullResponse := strings.Join(chunks, "")
	tokensPerSec := float64(len(chunks)) / totalTime.Seconds()

	t.Logf("AI (streamed): %s", fullResponse)
	t.Logf("First token: %v", firstTokenLatency)
	t.Logf("Total time: %v", totalTime)
	t.Logf("Chunks: %d | Speed: %.1f tokens/sec", len(chunks), tokensPerSec)

	if firstTokenLatency > 5*time.Second {
		t.Errorf("First token too slow: %v (target: <5s)", firstTokenLatency)
	}
	if tokensPerSec < 10 {
		t.Errorf("Token speed too low: %.1f/sec (target: >10)", tokensPerSec)
	}
}

// ============================================================
// TEST 4: Smart Cache - simula 60fps com cenas estáticas e mudanças
// ============================================================
func TestE2E_SmartCacheRealScenario(t *testing.T) {
	t.Log("=== Simulando: 60fps com detecção de mudança ===")

	cache := vision.NewFrameCache(vision.CacheConfig{
		ChangeThreshold:    0.05,
		ComparisonSize:     64,
		MinProcessInterval: 16,
		MaxProcessInterval: 500,
	})

	desktop1 := createDesktopScreenshot()
	desktop2 := createChangedDesktop() // has notification popup

	// Phase 1: 5 seconds of static desktop (300 frames at 60fps)
	t.Log("Phase 1: Static desktop (300 frames)...")
	staticNew := 0
	for range 300 {
		result := cache.Analyze(desktop1)
		if result.IsNew {
			staticNew++
		}
	}
	stats1 := cache.Stats()
	t.Logf("  New frames: %d/300 (expected: 1)", staticNew)
	t.Logf("  Cache hit rate: %.1f%%", float64(stats1.CachedFrames)/float64(stats1.TotalFrames)*100)
	t.Logf("  Adaptive interval: %dms", cache.CurrentInterval())

	if staticNew > 2 {
		t.Errorf("Static scene should have at most 2 new frames, got %d", staticNew)
	}

	// Phase 2: Screen changes (notification popup appears)
	t.Log("Phase 2: Notification popup appears...")
	result := cache.Analyze(desktop2)
	t.Logf("  Change detected: %v (changePercent: %.2f%%)", result.IsNew, result.ChangePercent*100)
	t.Logf("  Interval reset to: %dms", cache.CurrentInterval())

	if !result.IsNew {
		t.Error("Notification popup should be detected as change")
	}
	if cache.CurrentInterval() != 16 {
		t.Errorf("Interval should reset to 16ms after change, got %d", cache.CurrentInterval())
	}

	// Phase 3: Same changed desktop for 100 frames (stabilize)
	t.Log("Phase 3: Stable after change (100 frames)...")
	for range 100 {
		cache.Analyze(desktop2)
	}
	t.Logf("  Interval adapted to: %dms", cache.CurrentInterval())

	finalStats := cache.Stats()
	savedPercent := float64(finalStats.CachedFrames) / float64(finalStats.TotalFrames) * 100
	t.Logf("\n=== Final Stats ===")
	t.Logf("  Total frames: %d", finalStats.TotalFrames)
	t.Logf("  Processed: %d", finalStats.ProcessedFrames)
	t.Logf("  Cached: %d", finalStats.CachedFrames)
	t.Logf("  Savings: %.1f%% of frames skipped", savedPercent)

	if savedPercent < 95 {
		t.Errorf("Expected >95%% cache savings, got %.1f%%", savedPercent)
	}
}

// ============================================================
// TEST 5: Auto-Describe mode - descreve mudanças automaticamente
// ============================================================
func TestE2E_AutoDescribeMode(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("=== Simulando: Auto-describe detecta mudança e descreve ===")

	cache := vision.NewFrameCache(vision.DefaultCacheConfig())

	// Frame 1: static desktop
	desktop := createDesktopScreenshot()
	cache.Analyze(desktop)
	cache.CacheResponse("Desktop with code editor and browser.")

	// Frame 2: notification appears - cache invalidated
	changed := createChangedDesktop()
	result := cache.Analyze(changed)

	if !result.IsNew {
		t.Fatal("Change not detected")
	}

	// Auto-describe triggers: send to model
	processed, _ := vision.ProcessFrame(changed, 512)
	dataURI := vision.FormatAsDataURI(processed)

	messages := []llama.ChatMessage{
		{Role: "system", Content: "Describe briefly what changed. Focus on new elements. Max 2 sentences."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "What do you see right now? Focus on anything that stands out."},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	var desc strings.Builder
	err := client.StreamChatCompletion(ctx, messages, func(chunk llama.StreamChunk) {
		desc.WriteString(chunk.Content)
	})
	if err != nil {
		t.Fatalf("Auto-describe error: %v", err)
	}

	description := desc.String()
	cache.CacheResponse(description)

	t.Logf("Auto-describe: %s", description)

	// Verify cache stores the response
	cached, ok := cache.GetCachedResponse()
	if !ok {
		t.Fatal("Response should be cached")
	}
	t.Logf("Cached response available: %v (len: %d)", ok, len(cached))
}

// ============================================================
// TEST 6: Conversation Manager - multi-turn com histórico
// ============================================================
func TestE2E_ConversationManager(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	t.Log("=== Simulando: Conversa multi-turn completa ===")

	mgr := chat.NewManagerWithMaxHistory("You are a vision assistant. Answer concisely.", 20)

	// Turn 1: Vision question
	frame := createDesktopScreenshot()
	processed, _ := vision.ProcessFrame(frame, 512)
	dataURI := vision.FormatAsDataURI(processed)
	mgr.AddUserVisionMessage("What's on my screen?", dataURI)

	t.Log("Turn 1: 'What's on my screen?' [with frame]")
	start := time.Now()
	resp1, err := client.ChatCompletion(ctx, mgr.Messages())
	if err != nil {
		t.Fatalf("Turn 1 error: %v", err)
	}
	reply1, _ := resp1.Choices[0].Message.Content.(string)
	mgr.AddAssistantMessage(reply1)
	t.Logf("  AI: %s (%v)", reply1, time.Since(start))

	// Turn 2: Follow-up (no image)
	mgr.AddUserMessage("What color is the background?")
	t.Log("Turn 2: 'What color is the background?'")
	start = time.Now()
	resp2, err := client.ChatCompletion(ctx, mgr.Messages())
	if err != nil {
		t.Fatalf("Turn 2 error: %v", err)
	}
	reply2, _ := resp2.Choices[0].Message.Content.(string)
	mgr.AddAssistantMessage(reply2)
	t.Logf("  AI: %s (%v)", reply2, time.Since(start))

	// Turn 3: New image (screen changed)
	frame2 := createChangedDesktop()
	processed2, _ := vision.ProcessFrame(frame2, 512)
	dataURI2 := vision.FormatAsDataURI(processed2)
	mgr.AddUserVisionMessage("Something changed! What's different now?", dataURI2)

	t.Log("Turn 3: 'Something changed! What's different now?' [with new frame]")
	start = time.Now()
	resp3, err := client.ChatCompletion(ctx, mgr.Messages())
	if err != nil {
		t.Fatalf("Turn 3 error: %v", err)
	}
	reply3, _ := resp3.Choices[0].Message.Content.(string)
	mgr.AddAssistantMessage(reply3)
	t.Logf("  AI: %s (%v)", reply3, time.Since(start))

	// Verify conversation coherence
	msgs := mgr.Messages()
	t.Logf("\nTotal messages in context: %d", len(msgs))
	t.Logf("Total tokens used: Turn1=%d, Turn2=%d, Turn3=%d",
		resp1.Usage.TotalTokens, resp2.Usage.TotalTokens, resp3.Usage.TotalTokens)
}

// ============================================================
// TEST 7: Performance benchmark
// ============================================================
func TestE2E_PerformanceBenchmark(t *testing.T) {
	client := llama.NewClient(testServerURL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("=== Performance Benchmark ===")

	// Text-only latency
	t.Log("\n1. Text-only latency:")
	textStart := time.Now()
	textResp, _ := client.ChatCompletion(ctx, []llama.ChatMessage{
		{Role: "user", Content: "Say 'hello' and nothing else."},
	})
	textLatency := time.Since(textStart)
	textReply, _ := textResp.Choices[0].Message.Content.(string)
	t.Logf("   Response: %s | Latency: %v", textReply, textLatency)

	// Vision latency
	t.Log("2. Vision latency:")
	frame := createPersonAtDesk()
	processed, _ := vision.ProcessFrame(frame, 512)
	dataURI := vision.FormatAsDataURI(processed)

	visionStart := time.Now()
	visionResp, _ := client.ChatCompletion(ctx, []llama.ChatMessage{
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "One word: what's the main object?"},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	})
	visionLatency := time.Since(visionStart)
	visionReply, _ := visionResp.Choices[0].Message.Content.(string)
	t.Logf("   Response: %s | Latency: %v", visionReply, visionLatency)

	// Streaming first-token latency
	t.Log("3. Streaming first-token latency:")
	var firstToken time.Duration
	streamStart := time.Now()
	client.StreamChatCompletion(ctx, []llama.ChatMessage{
		{Role: "user", Content: "Count to 5."},
	}, func(chunk llama.StreamChunk) {
		if firstToken == 0 {
			firstToken = time.Since(streamStart)
		}
	})
	t.Logf("   First token: %v", firstToken)

	// Frame processing throughput
	t.Log("4. Frame processing throughput:")
	bigFrame := createDesktopScreenshot() // 1920x1080
	procStart := time.Now()
	for range 100 {
		vision.ProcessFrame(bigFrame, 512)
	}
	procTime := time.Since(procStart)
	t.Logf("   100 frames (1920x1080→512): %v | %.0f frames/sec", procTime, 100/procTime.Seconds())

	// Cache analysis throughput
	t.Log("5. Cache analysis throughput:")
	cache := vision.NewFrameCache(vision.DefaultCacheConfig())
	cacheStart := time.Now()
	for range 1000 {
		cache.Analyze(bigFrame)
	}
	cacheTime := time.Since(cacheStart)
	t.Logf("   1000 frames analyzed: %v | %.0f frames/sec", cacheTime, 1000/cacheTime.Seconds())

	t.Log("\n=== Summary ===")
	t.Logf("Text latency:        %v", textLatency)
	t.Logf("Vision latency:      %v", visionLatency)
	t.Logf("Stream first token:  %v", firstToken)
	t.Logf("Frame process rate:  %.0f fps", 100/procTime.Seconds())
	t.Logf("Cache analysis rate: %.0f fps", 1000/cacheTime.Seconds())
}
