package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func toolListFiles(args map[string]any) ToolResult {
	path := getArg(args, "path")
	if path == "" {
		return ToolResult{Success: false, Error: "missing 'path' argument"}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot list directory: %v", err)}
	}

	var sb strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			fmt.Fprintf(&sb, "[DIR]  %s\n", entry.Name())
		} else if info != nil {
			fmt.Fprintf(&sb, "[FILE] %s (%d bytes)\n", entry.Name(), info.Size())
		} else {
			fmt.Fprintf(&sb, "[FILE] %s\n", entry.Name())
		}
	}

	if sb.Len() == 0 {
		return ToolResult{Success: true, Output: "(empty directory)"}
	}
	return ToolResult{Success: true, Output: strings.TrimSpace(sb.String())}
}

func toolReadFile(args map[string]any) ToolResult {
	path := getArg(args, "path")
	if path == "" {
		return ToolResult{Success: false, Error: "missing 'path' argument"}
	}

	info, err := os.Stat(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot stat file: %v", err)}
	}
	if info.Size() > 100*1024 {
		return ToolResult{Success: false, Error: fmt.Sprintf("file too large: %d bytes (max 100KB)", info.Size())}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot read file: %v", err)}
	}

	return ToolResult{Success: true, Output: string(data)}
}

func toolCreateFile(args map[string]any) ToolResult {
	path := getArg(args, "path")
	if path == "" {
		return ToolResult{Success: false, Error: "missing 'path' argument"}
	}
	content := getArg(args, "content")

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot create directory: %v", err)}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot write file: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("File created: %s (%d bytes)", path, len(content))}
}

func toolDeleteFile(args map[string]any) ToolResult {
	path := getArg(args, "path")
	if path == "" {
		return ToolResult{Success: false, Error: "missing 'path' argument"}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ToolResult{Success: false, Error: fmt.Sprintf("file does not exist: %s", path)}
	}

	if err := os.Remove(path); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot delete file: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("Deleted: %s", path)}
}

func toolOpenFolder(args map[string]any) ToolResult {
	path := getArg(args, "path")
	if path == "" {
		return ToolResult{Success: false, Error: "missing 'path' argument"}
	}

	info, err := os.Stat(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("path does not exist: %v", err)}
	}
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot open folder: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("Opened folder: %s", path)}
}

func toolRunProgram(args map[string]any) ToolResult {
	target := getArg(args, "target")
	if target == "" {
		return ToolResult{Success: false, Error: "missing 'target' argument"}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Start(); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("cannot run program: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("Opened: %s", target)}
}
