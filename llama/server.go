package llama

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// ServerStatus represents the current state of the llama-server subprocess.
type ServerStatus string

const (
	StatusStopped  ServerStatus = "stopped"
	StatusStarting ServerStatus = "starting"
	StatusRunning  ServerStatus = "running"
)

// ServerManager manages the llama-server subprocess lifecycle.
type ServerManager struct {
	mu     sync.Mutex
	cfg    ServerConfig
	cmd    *exec.Cmd
	status ServerStatus
}

// DefaultServerConfig returns sensible defaults for llama-server.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:       "127.0.0.1",
		Port:       8090,
		NGPULayers: 99,
		CtxSize:    4096,
		FlashAttn:  true,
	}
}

// NewServerManager creates a new server manager with the given config.
func NewServerManager(cfg ServerConfig) *ServerManager {
	return &ServerManager{
		cfg:    cfg,
		status: StatusStopped,
	}
}

// BuildArgs returns the command-line arguments for llama-server.
func (m *ServerManager) BuildArgs() []string {
	args := []string{}

	// Model source: HF repo takes priority over local path
	if m.cfg.HFRepo != "" {
		args = append(args, "--hf-repo", m.cfg.HFRepo)
	} else if m.cfg.ModelPath != "" {
		args = append(args, "--model", m.cfg.ModelPath)
		if m.cfg.MMProjPath != "" {
			args = append(args, "--mmproj", m.cfg.MMProjPath)
		}
	}

	args = append(args,
		"-ngl", strconv.Itoa(m.cfg.NGPULayers),
		"--host", m.cfg.Host,
		"--port", strconv.Itoa(m.cfg.Port),
	)

	if m.cfg.CtxSize > 0 {
		args = append(args, "--ctx-size", strconv.Itoa(m.cfg.CtxSize))
	}

	if m.cfg.FlashAttn {
		args = append(args, "--flash-attn", "on")
	}

	return args
}

// Start launches the llama-server subprocess.
func (m *ServerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusRunning || m.status == StatusStarting {
		return nil
	}

	m.status = StatusStarting
	m.cmd = exec.CommandContext(ctx, m.cfg.ExecutablePath, m.BuildArgs()...)

	if err := m.cmd.Start(); err != nil {
		m.status = StatusStopped
		m.cmd = nil
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	m.status = StatusRunning

	// Monitor process exit in background
	go func() {
		m.cmd.Wait()
		m.mu.Lock()
		m.status = StatusStopped
		m.mu.Unlock()
	}()

	return nil
}

// Stop terminates the llama-server subprocess.
func (m *ServerManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	if err := m.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill llama-server: %w", err)
	}

	m.status = StatusStopped
	m.cmd = nil
	return nil
}

// Status returns the current server status.
func (m *ServerManager) Status() ServerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// URL returns the base URL of the llama-server.
func (m *ServerManager) URL() string {
	return fmt.Sprintf("http://%s:%d", m.cfg.Host, m.cfg.Port)
}

// WaitForReady polls the health endpoint until the server is ready or context expires.
func (m *ServerManager) WaitForReady(ctx context.Context) error {
	client := NewClient(m.URL())
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for llama-server: %w", ctx.Err())
		case <-ticker.C:
			ok, _ := client.HealthCheck(ctx)
			if ok {
				return nil
			}
		}
	}
}
