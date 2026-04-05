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
	"vision-chat/tools"
	"vision-chat/vision"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the frontend via Wails.
type App struct {
	ctx       context.Context
	server    *llama.ServerManager
	client    *llama.Client
	chatMgr   *chat.Manager
	cache     *vision.FrameCache
	toolReg   *tools.Registry

	// Auto-describe mode
	autoDescribe   bool
	autoDescMu     sync.Mutex
	autoDescCancel context.CancelFunc

	// Tool confirmation
	pendingTool   *tools.ToolCall
	pendingToolMu sync.Mutex
	autoApprove   bool
}

// NewApp creates a new App application struct.
func NewApp() *App {
	llamaCppDir := `C:\Users\joaop\.cache\models\llama-cpp`
	cfg := llama.ServerConfig{
		ExecutablePath: llamaCppDir + `\llama-server.exe`,
		HFRepo:         "ggml-org/gemma-4-E4B-it-GGUF:Q8_0",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
		CtxSize:        4096,
		FlashAttn:      true,
	}
	toolReg := tools.NewRegistry()
	screenW, screenH := tools.GetScreenSize()
	systemPrompt := fmt.Sprintf(`You are a computer-use agent. You SEE the user's screen and control their mouse and keyboard based on what you see.

EVERYTHING you do must be based on WHAT YOU SEE in the image. Look at the screen, find the element, click on it.

COORDINATE SYSTEM:
- The image you see is approximately 512 pixels wide and 288 pixels tall.
- When you use click(x, y), give the coordinates of the element IN THE IMAGE.
- The system automatically converts to real screen coordinates (%dx%d).
- x=0 is the left edge, x=512 is the right edge.
- y=0 is the top edge, y=288 is the bottom edge.

HOW TO ACT:
1. LOOK at the screenshot — identify what's on screen
2. FIND the element the user wants (search bar, button, link, text field)
3. ESTIMATE its x,y position in the image
4. click() on it, then type_text() if needed, then press_key("enter")
5. Put ALL tool_calls in ONE response

EXAMPLE — "search for cars on YouTube" (you see YouTube is open):
I can see the YouTube search bar at the top center of the page, around x=350, y=22.
<tool_call>{"name": "click", "args": {"x": 350, "y": 22}}</tool_call>
<tool_call>{"name": "type_text", "args": {"text": "carros"}}</tool_call>
<tool_call>{"name": "press_key", "args": {"key": "enter"}}</tool_call>

EXAMPLE — "click on the first video" (you see video thumbnails):
I can see the first video thumbnail at around x=200, y=150.
<tool_call>{"name": "click", "args": {"x": 200, "y": 150}}</tool_call>

NAVIGATION AND SEARCH — use keyboard shortcuts, they are 100%% reliable:

Go to a website (use Ctrl+L to focus the address bar):
<tool_call>{"name": "hotkey", "args": {"modifier": "ctrl", "key": "l"}}</tool_call>
<tool_call>{"name": "type_text", "args": {"text": "youtube.com"}}</tool_call>
<tool_call>{"name": "press_key", "args": {"key": "enter"}}</tool_call>

Search on YouTube (navigate via URL):
<tool_call>{"name": "hotkey", "args": {"modifier": "ctrl", "key": "l"}}</tool_call>
<tool_call>{"name": "type_text", "args": {"text": "youtube.com/results?search_query=carros"}}</tool_call>
<tool_call>{"name": "press_key", "args": {"key": "enter"}}</tool_call>

Search on Google:
<tool_call>{"name": "hotkey", "args": {"modifier": "ctrl", "key": "l"}}</tool_call>
<tool_call>{"name": "type_text", "args": {"text": "google.com/search?q=receita+de+bolo"}}</tool_call>
<tool_call>{"name": "press_key", "args": {"key": "enter"}}</tool_call>

Use click(x,y) ONLY for clicking on visual elements like buttons, links, videos, images.

RULES:
- For navigation and search: ALWAYS use hotkey(ctrl, l) + type_text + press_key(enter). This is 100%% reliable.
- For clicking on visual elements (videos, buttons, links): use click(x,y) with image coordinates.
- NEVER use ctrl+f. NEVER refuse to act. Be PROACTIVE.
- Put ALL tool_calls in ONE response.
- After tools execute, describe what you see.

`, screenW, screenH) + toolReg.BuildToolPrompt()

	return &App{
		server:  llama.NewServerManager(cfg),
		toolReg: toolReg,
		chatMgr: chat.NewManagerWithMaxHistory(systemPrompt, 20),
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
		// Wait for frontend to load, then capture VisionChat's window handle
		time.Sleep(2 * time.Second)
		tools.CaptureAppWindow()

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

	// Check for tool calls in the response
	toolCalls := tools.ParseToolCalls(reply)
	if len(toolCalls) > 0 {
		go a.handleToolCalls(toolCalls)
	}

	return reply, nil
}

// handleToolCalls processes tool calls from the AI response.
func (a *App) handleToolCalls(calls []tools.ToolCall) {
	var results []string

	// Focus target window ONCE before executing the entire batch
	tools.FocusTarget()

	for _, call := range calls {
		if a.toolReg.IsDangerous(call.Name) && !a.autoApprove {
			a.pendingToolMu.Lock()
			callCopy := call
			a.pendingTool = &callCopy
			a.pendingToolMu.Unlock()

			wailsRuntime.EventsEmit(a.ctx, "tool:confirm", map[string]interface{}{
				"name": call.Name,
				"args": call.Args,
			})
			return
		}

		// Execute tool — NO UI events during batch to prevent focus steal
		result := a.toolReg.Execute(call)

		var msg string
		if result.Success {
			msg = fmt.Sprintf("[%s] OK: %s", call.Name, result.Output)
		} else {
			msg = fmt.Sprintf("[%s] FAILED: %s", call.Name, result.Error)
		}
		results = append(results, msg)

		// Brief delay between tools
		time.Sleep(100 * time.Millisecond)
	}

	// Tools done — restore VisionChat immediately
	tools.RestoreApp()
	time.Sleep(300 * time.Millisecond)

	allResults := strings.Join(results, "\n")
	wailsRuntime.EventsEmit(a.ctx, "tool:batch-result", allResults)

	// Ask AI to verify and continue
	a.chatMgr.AddUserMessage("Tool results:\n" + allResults + "\n\nDescribe what you see now. If there are more steps to complete the user's request, do them.")

	var finalResponse strings.Builder
	a.client.StreamChatCompletion(a.ctx, a.chatMgr.Messages(), func(chunk llama.StreamChunk) {
		finalResponse.WriteString(chunk.Content)
		wailsRuntime.EventsEmit(a.ctx, "chat:stream", chunk.Content)
	})

	final := finalResponse.String()
	a.chatMgr.AddAssistantMessage(final)
	wailsRuntime.EventsEmit(a.ctx, "chat:stream:done", final)

	// Continue if AI wants more actions
	nextCalls := tools.ParseToolCalls(final)
	if len(nextCalls) > 0 {
		time.Sleep(300 * time.Millisecond)
		a.handleToolCalls(nextCalls)
	}
}

// ConfirmTool executes a pending dangerous tool after user confirmation.
func (a *App) ConfirmTool() {
	a.pendingToolMu.Lock()
	call := a.pendingTool
	a.pendingTool = nil
	a.pendingToolMu.Unlock()

	if call == nil {
		return
	}

	a.handleToolCalls([]tools.ToolCall{*call})
}

// SetAutoApprove enables or disables automatic approval of all tool actions.
func (a *App) SetAutoApprove(enabled bool) {
	a.autoApprove = enabled
}

// IsAutoApprove returns whether auto-approve is enabled.
func (a *App) IsAutoApprove() bool {
	return a.autoApprove
}

// ConfirmToolAndApproveAll executes the pending tool and enables auto-approve for future tools.
func (a *App) ConfirmToolAndApproveAll() {
	a.autoApprove = true
	a.ConfirmTool()
}

// DenyTool cancels a pending dangerous tool.
func (a *App) DenyTool() {
	a.pendingToolMu.Lock()
	call := a.pendingTool
	a.pendingTool = nil
	a.pendingToolMu.Unlock()

	if call == nil {
		return
	}

	a.chatMgr.AddUserMessage(fmt.Sprintf("User denied execution of tool '%s'. Do not retry.", call.Name))

	var response strings.Builder
	a.client.StreamChatCompletion(a.ctx, a.chatMgr.Messages(), func(chunk llama.StreamChunk) {
		response.WriteString(chunk.Content)
		wailsRuntime.EventsEmit(a.ctx, "chat:stream", chunk.Content)
	})

	reply := response.String()
	a.chatMgr.AddAssistantMessage(reply)
	wailsRuntime.EventsEmit(a.ctx, "chat:stream:done", reply)
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

// SetShareTarget tells the agent which window/screen is being shared.
// Called by the frontend when screen/window capture starts.
func (a *App) SetShareTarget(trackLabel string) {
	log.Printf("Screen share target: %q", trackLabel)

	// Try to find the window by the track label
	if tools.SetTargetByTitle(trackLabel) {
		log.Printf("Target window found for: %q", trackLabel)
		return
	}

	// Try common browsers — this is the most common use case
	browsers := []string{"chrome", "firefox", "edge", "opera", "brave", "vivaldi"}
	for _, browser := range browsers {
		if tools.SetTargetByTitle(browser) {
			log.Printf("Target window found via browser: %s", browser)
			return
		}
	}

	// Last resort: capture whatever was the foreground before VisionChat
	tools.CaptureTargetWindow()
	log.Printf("Using last active window as target")
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
