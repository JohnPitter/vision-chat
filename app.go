package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"vision-chat/chat"
	"vision-chat/llama"
	"vision-chat/vision"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the frontend via Wails.
type App struct {
	ctx     context.Context
	server  *llama.ServerManager
	client  *llama.Client
	chatMgr *chat.Manager
	cache   *vision.FrameCache

	// Auto-describe mode
	autoDescribe   bool
	autoDescMu     sync.Mutex
	autoDescCancel context.CancelFunc
}

// NewApp creates a new App application struct.
func NewApp() *App {
	llamaCppDir := `C:\Users\joaop\.cache\models\llama-cpp`
	cfg := llama.ServerConfig{
		ExecutablePath: llamaCppDir + `\llama-server.exe`,
		HFRepo:         "ggml-org/gemma-3-4b-it-GGUF",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
		CtxSize:        4096,
		FlashAttn:      true,
	}
	return &App{
		server: llama.NewServerManager(cfg),
		chatMgr: chat.NewManagerWithMaxHistory(
			"You are a helpful vision assistant. Describe what you see in images and answer questions about them. Be concise and direct. When in auto-describe mode, focus on changes and movement.",
			20,
		),
		cache: vision.NewFrameCache(vision.CacheConfig{
			ChangeThreshold:    0.05,
			ComparisonSize:     64,
			MinProcessInterval: 16,  // 60fps capture
			MaxProcessInterval: 500, // slow down when static
		}),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	go func() {
		// Check if server is already running (e.g. started externally)
		tempClient := llama.NewClient(a.server.URL())
		if ok, _ := tempClient.HealthCheck(ctx); ok {
			log.Printf("llama-server already running at %s", a.server.URL())
			a.client = tempClient
			wailsRuntime.EventsEmit(ctx, "server:ready", true)
			return
		}

		if err := a.server.Start(ctx); err != nil {
			log.Printf("WARNING: failed to start llama-server: %v", err)
			wailsRuntime.EventsEmit(ctx, "server:error", err.Error())
			return
		}

		readyCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		if err := a.server.WaitForReady(readyCtx); err != nil {
			log.Printf("WARNING: llama-server not ready: %v", err)
			wailsRuntime.EventsEmit(ctx, "server:error", err.Error())
			return
		}

		a.client = llama.NewClient(a.server.URL())
		wailsRuntime.EventsEmit(ctx, "server:ready", true)
	}()
}

// shutdown is called when the app exits.
func (a *App) shutdown(ctx context.Context) {
	a.StopAutoDescribe()
	if a.server != nil {
		a.server.Stop()
	}
}

// SendMessage receives user text + optional base64 frame, streams AI response.
func (a *App) SendMessage(text string, frameBase64 string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("server not ready")
	}

	if frameBase64 != "" {
		processed, err := vision.ProcessFrame(frameBase64, 512)
		if err != nil {
			return "", fmt.Errorf("frame processing failed: %w", err)
		}
		dataURI := vision.FormatAsDataURI(processed)
		a.chatMgr.AddUserVisionMessage(text, dataURI)
	} else {
		a.chatMgr.AddUserMessage(text)
	}

	// Try streaming first
	var fullResponse strings.Builder
	err := a.client.StreamChatCompletion(a.ctx, a.chatMgr.Messages(), func(chunk llama.StreamChunk) {
		fullResponse.WriteString(chunk.Content)
		wailsRuntime.EventsEmit(a.ctx, "chat:stream", chunk.Content)
	})

	if err != nil {
		// Fallback to non-streaming
		resp, fallbackErr := a.client.ChatCompletion(a.ctx, a.chatMgr.Messages())
		if fallbackErr != nil {
			return "", fmt.Errorf("AI request failed: %w", fallbackErr)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response from model")
		}
		reply, _ := resp.Choices[0].Message.Content.(string)
		a.chatMgr.AddAssistantMessage(reply)
		return reply, nil
	}

	reply := fullResponse.String()
	a.chatMgr.AddAssistantMessage(reply)
	wailsRuntime.EventsEmit(a.ctx, "chat:stream:done", reply)
	return reply, nil
}

// AnalyzeFrame checks if a frame has changed enough to warrant AI processing.
// Called at 60fps from the frontend — returns whether the frame is new.
func (a *App) AnalyzeFrame(frameBase64 string) map[string]interface{} {
	result := a.cache.Analyze(frameBase64)
	stats := a.cache.Stats()
	return map[string]interface{}{
		"isNew":         result.IsNew,
		"changePercent": result.ChangePercent,
		"interval":      a.cache.CurrentInterval(),
		"totalFrames":   stats.TotalFrames,
		"cachedFrames":  stats.CachedFrames,
		"savedPercent":  float64(stats.CachedFrames) / max(float64(stats.TotalFrames), 1) * 100,
	}
}

// StartAutoDescribe enables auto-describe mode that periodically describes
// what the camera sees, focusing on changes and movement.
func (a *App) StartAutoDescribe(intervalMs int) {
	a.autoDescMu.Lock()
	defer a.autoDescMu.Unlock()

	if a.autoDescribe {
		return
	}

	a.autoDescribe = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.autoDescCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				wailsRuntime.EventsEmit(a.ctx, "auto:request-frame", true)
			}
		}
	}()
}

// StopAutoDescribe disables auto-describe mode.
func (a *App) StopAutoDescribe() {
	a.autoDescMu.Lock()
	defer a.autoDescMu.Unlock()

	if a.autoDescCancel != nil {
		a.autoDescCancel()
		a.autoDescCancel = nil
	}
	a.autoDescribe = false
}

// IsAutoDescribing returns whether auto-describe mode is active.
func (a *App) IsAutoDescribing() bool {
	a.autoDescMu.Lock()
	defer a.autoDescMu.Unlock()
	return a.autoDescribe
}

// AutoDescribeFrame is called by the frontend with a captured frame during auto-describe.
func (a *App) AutoDescribeFrame(frameBase64 string) {
	if a.client == nil {
		return
	}

	// Only process if frame actually changed
	result := a.cache.Analyze(frameBase64)
	if !result.IsNew {
		return
	}

	processed, err := vision.ProcessFrame(frameBase64, 512)
	if err != nil {
		return
	}

	dataURI := vision.FormatAsDataURI(processed)
	messages := []llama.ChatMessage{
		{Role: "system", Content: "Describe briefly what you see. Focus on changes, movement, and key objects. Max 2 sentences."},
		{Role: "user", Content: []llama.ContentPart{
			{Type: "text", Text: "What do you see right now?"},
			{Type: "image_url", ImageURL: &llama.ImageURL{URL: dataURI}},
		}},
	}

	var desc strings.Builder
	err = a.client.StreamChatCompletion(a.ctx, messages, func(chunk llama.StreamChunk) {
		desc.WriteString(chunk.Content)
		wailsRuntime.EventsEmit(a.ctx, "auto:stream", chunk.Content)
	})

	if err != nil {
		return
	}

	a.cache.CacheResponse(desc.String())
	wailsRuntime.EventsEmit(a.ctx, "auto:done", desc.String())
}

// ClearChat resets conversation history.
func (a *App) ClearChat() {
	a.chatMgr.Clear()
}

// GetServerStatus returns current server status.
func (a *App) GetServerStatus() string {
	return string(a.server.Status())
}

// GetCacheStats returns frame cache statistics.
func (a *App) GetCacheStats() map[string]interface{} {
	stats := a.cache.Stats()
	return map[string]interface{}{
		"totalFrames":     stats.TotalFrames,
		"cachedFrames":    stats.CachedFrames,
		"processedFrames": stats.ProcessedFrames,
		"savedPercent":    float64(stats.CachedFrames) / max(float64(stats.TotalFrames), 1) * 100,
		"currentInterval": a.cache.CurrentInterval(),
	}
}
