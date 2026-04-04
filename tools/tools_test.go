package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================
// Registry tests
// ============================================================

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
}

func TestRegistry_ListTools(t *testing.T) {
	r := NewRegistry()
	tools := r.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected at least one registered tool")
	}

	// Verify known tools exist
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{"list_files", "open_folder", "delete_file", "create_file", "read_file", "run_program"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "nonexistent", Args: map[string]string{}})
	if result.Success {
		t.Error("executing unknown tool should fail")
	}
	if !strings.Contains(result.Error, "unknown tool") {
		t.Errorf("expected 'unknown tool' error, got: %s", result.Error)
	}
}

func TestRegistry_IsDangerous(t *testing.T) {
	r := NewRegistry()

	if !r.IsDangerous("delete_file") {
		t.Error("delete_file should be dangerous")
	}
	if r.IsDangerous("list_files") {
		t.Error("list_files should not be dangerous")
	}
	if !r.IsDangerous("run_program") {
		t.Error("run_program should be dangerous")
	}
}

// ============================================================
// Tool call parser tests
// ============================================================

func TestParseToolCall_ValidJSON(t *testing.T) {
	input := `I'll list the files for you.
<tool_call>
{"name": "list_files", "args": {"path": "C:\\Users\\test\\Documents"}}
</tool_call>`

	calls := ParseToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "list_files" {
		t.Errorf("expected tool name 'list_files', got %q", calls[0].Name)
	}
	if calls[0].Args["path"] != `C:\Users\test\Documents` {
		t.Errorf("unexpected path arg: %q", calls[0].Args["path"])
	}
}

func TestParseToolCall_MultipleTools(t *testing.T) {
	input := `Let me do both.
<tool_call>
{"name": "list_files", "args": {"path": "/tmp"}}
</tool_call>
And then:
<tool_call>
{"name": "read_file", "args": {"path": "/tmp/test.txt"}}
</tool_call>`

	calls := ParseToolCalls(input)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Name != "list_files" {
		t.Errorf("first call should be list_files, got %s", calls[0].Name)
	}
	if calls[1].Name != "read_file" {
		t.Errorf("second call should be read_file, got %s", calls[1].Name)
	}
}

func TestParseToolCall_NoToolCall(t *testing.T) {
	input := "I can see a cat on your screen! It looks cute."
	calls := ParseToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestParseToolCall_InvalidJSON(t *testing.T) {
	input := `<tool_call>
this is not json
</tool_call>`

	calls := ParseToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("invalid JSON should produce 0 tool calls, got %d", len(calls))
	}
}

func TestExtractTextBeforeToolCall(t *testing.T) {
	input := `Sure, I'll list the files for you.
<tool_call>
{"name": "list_files", "args": {"path": "/tmp"}}
</tool_call>`

	text := ExtractTextBeforeToolCall(input)
	if text != "Sure, I'll list the files for you." {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestExtractTextBeforeToolCall_NoToolCall(t *testing.T) {
	input := "Just a normal message."
	text := ExtractTextBeforeToolCall(input)
	if text != "Just a normal message." {
		t.Errorf("unexpected text: %q", text)
	}
}

// ============================================================
// Filesystem tool tests
// ============================================================

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "list_files", Args: map[string]string{"path": dir}})

	if !result.Success {
		t.Fatalf("list_files failed: %s", result.Error)
	}
	if !strings.Contains(result.Output, "a.txt") {
		t.Error("output should contain a.txt")
	}
	if !strings.Contains(result.Output, "b.txt") {
		t.Error("output should contain b.txt")
	}
	if !strings.Contains(result.Output, "subdir") {
		t.Error("output should contain subdir")
	}
}

func TestListFiles_InvalidPath(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "list_files", Args: map[string]string{"path": "/nonexistent/dir"}})
	if result.Success {
		t.Error("should fail for nonexistent path")
	}
}

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "create_file", Args: map[string]string{
		"path":    path,
		"content": "hello world",
	}})

	if !result.Success {
		t.Fatalf("create_file failed: %s", result.Error)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("file content here"), 0644)

	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "read_file", Args: map[string]string{"path": path}})

	if !result.Success {
		t.Fatalf("read_file failed: %s", result.Error)
	}
	if result.Output != "file content here" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestDeleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "to_delete.txt")
	os.WriteFile(path, []byte("delete me"), 0644)

	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "delete_file", Args: map[string]string{"path": path}})

	if !result.Success {
		t.Fatalf("delete_file failed: %s", result.Error)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteFile_NonexistentFile(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "delete_file", Args: map[string]string{"path": "/nonexistent/file.txt"}})
	if result.Success {
		t.Error("deleting nonexistent file should fail")
	}
}

func TestDeleteFile_MissingPath(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(ToolCall{Name: "delete_file", Args: map[string]string{}})
	if result.Success {
		t.Error("should fail without path arg")
	}
}

// ============================================================
// System prompt tests
// ============================================================

func TestBuildSystemPrompt(t *testing.T) {
	r := NewRegistry()
	prompt := r.BuildToolPrompt()

	if !strings.Contains(prompt, "list_files") {
		t.Error("prompt should contain list_files")
	}
	if !strings.Contains(prompt, "tool_call") {
		t.Error("prompt should contain tool_call format")
	}
	if !strings.Contains(prompt, "delete_file") {
		t.Error("prompt should contain delete_file")
	}
}
