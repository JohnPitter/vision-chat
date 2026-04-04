package llama

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestServerConfig_DefaultValues(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Port != 8090 {
		t.Errorf("expected port 8090, got %d", cfg.Port)
	}
	if cfg.NGPULayers != 99 {
		t.Errorf("expected 99 GPU layers, got %d", cfg.NGPULayers)
	}
	if cfg.CtxSize != 4096 {
		t.Errorf("expected ctx size 4096, got %d", cfg.CtxSize)
	}
	if !cfg.FlashAttn {
		t.Error("expected flash attention enabled by default")
	}
}

func TestServerManager_BuildArgs_LocalModel(t *testing.T) {
	cfg := ServerConfig{
		ExecutablePath: "/path/to/llama-server",
		ModelPath:      "/path/to/model.gguf",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
	}
	mgr := NewServerManager(cfg)
	args := mgr.BuildArgs()

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--model /path/to/model.gguf") {
		t.Errorf("expected --model flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "-ngl 99") {
		t.Errorf("expected -ngl flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "--host 127.0.0.1") {
		t.Errorf("expected --host flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "--port 8090") {
		t.Errorf("expected --port flag, got: %v", args)
	}
}

func TestServerManager_BuildArgs_HFRepo(t *testing.T) {
	cfg := ServerConfig{
		ExecutablePath: "/path/to/llama-server",
		HFRepo:         "bartowski/Llama-3.2-11B-Vision-Instruct-GGUF:Q4_K_M",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
		CtxSize:        4096,
		FlashAttn:      true,
	}
	mgr := NewServerManager(cfg)
	args := mgr.BuildArgs()
	argsStr := strings.Join(args, " ")

	// Should use --hf-repo instead of --model
	if strings.Contains(argsStr, "--model") {
		t.Error("HFRepo mode should not use --model flag")
	}
	if !strings.Contains(argsStr, "--hf-repo bartowski/Llama-3.2-11B-Vision-Instruct-GGUF:Q4_K_M") {
		t.Errorf("expected --hf-repo flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "--ctx-size 4096") {
		t.Errorf("expected --ctx-size flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "--flash-attn on") {
		t.Errorf("expected --flash-attn flag, got: %v", args)
	}
}

func TestServerManager_BuildArgs_LocalWithMMProj(t *testing.T) {
	cfg := ServerConfig{
		ModelPath:  "/path/to/model.gguf",
		MMProjPath: "/path/to/mmproj.gguf",
		Host:       "127.0.0.1",
		Port:       8090,
		NGPULayers: 99,
	}
	mgr := NewServerManager(cfg)
	args := mgr.BuildArgs()
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--model /path/to/model.gguf") {
		t.Errorf("expected --model flag, got: %v", args)
	}
	if !strings.Contains(argsStr, "--mmproj /path/to/mmproj.gguf") {
		t.Errorf("expected --mmproj flag, got: %v", args)
	}
}

func TestServerManager_StateTransitions(t *testing.T) {
	cfg := ServerConfig{
		ExecutablePath: "",
		ModelPath:      "test.gguf",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
	}
	mgr := NewServerManager(cfg)

	if mgr.Status() != StatusStopped {
		t.Errorf("initial status should be Stopped, got %s", mgr.Status())
	}
}

func TestServerManager_StartWithInvalidPath(t *testing.T) {
	cfg := ServerConfig{
		ExecutablePath: "/nonexistent/llama-server",
		ModelPath:      "test.gguf",
		Host:           "127.0.0.1",
		Port:           8090,
		NGPULayers:     99,
	}
	mgr := NewServerManager(cfg)
	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatal("expected error starting with invalid path")
		mgr.Stop()
	}
	if mgr.Status() != StatusStopped {
		t.Errorf("status should remain Stopped after failed start, got %s", mgr.Status())
	}
}

func TestServerManager_StopWhenNotRunning(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := NewServerManager(cfg)
	err := mgr.Stop()
	if err != nil {
		t.Errorf("stopping a non-running server should not error: %v", err)
	}
}

func TestServerManager_URL(t *testing.T) {
	cfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 8090,
	}
	mgr := NewServerManager(cfg)
	url := mgr.URL()
	if url != "http://127.0.0.1:8090" {
		t.Errorf("expected http://127.0.0.1:8090, got %s", url)
	}
}

func TestServerManager_WaitForReady_Timeout(t *testing.T) {
	cfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 19999,
	}
	mgr := NewServerManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := mgr.WaitForReady(ctx)
	if err == nil {
		t.Fatal("expected timeout error waiting for unavailable server")
	}
}
