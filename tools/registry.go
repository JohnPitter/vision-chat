package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolFunc is the function signature for tool implementations.
type ToolFunc func(args map[string]any) ToolResult

// getArg extracts a string argument from the args map, handling both string and numeric values.
func getArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

type registeredTool struct {
	Tool Tool
	Fn   ToolFunc
}

// Registry holds all available tools and executes them.
type Registry struct {
	tools map[string]registeredTool
}

// NewRegistry creates a registry with all default tools registered.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]registeredTool)}
	r.registerDefaults()
	return r
}

func (r *Registry) registerDefaults() {
	// Filesystem tools
	r.Register(Tool{Name: "list_files", Description: "List files and folders in a directory. Args: path", Dangerous: false}, toolListFiles)
	r.Register(Tool{Name: "read_file", Description: "Read the contents of a text file. Args: path", Dangerous: false}, toolReadFile)
	r.Register(Tool{Name: "create_file", Description: "Create or overwrite a file with content. Args: path, content", Dangerous: true}, toolCreateFile)
	r.Register(Tool{Name: "delete_file", Description: "Delete a file permanently. Args: path", Dangerous: true}, toolDeleteFile)
	r.Register(Tool{Name: "open_folder", Description: "Open a folder in the system file explorer. Args: path", Dangerous: false}, toolOpenFolder)
	r.Register(Tool{Name: "run_program", Description: "Run a program or open a file with the default application. Args: target", Dangerous: true}, toolRunProgram)

	// Screen automation tools (vision-based)
	r.Register(Tool{Name: "click", Description: "Left-click at screen coordinates. Args: x, y (pixels in image space)", Dangerous: false}, toolClick)
	r.Register(Tool{Name: "double_click", Description: "Double-click at screen coordinates. Args: x, y (pixels)", Dangerous: false}, toolDoubleClick)
	r.Register(Tool{Name: "type_text", Description: "Type text at the current cursor position. Args: text", Dangerous: false}, toolTypeText)
	r.Register(Tool{Name: "press_key", Description: "Press a key (enter, tab, escape, backspace, delete, up, down, left, right, f1-f12). Args: key", Dangerous: false}, toolPressKey)
	r.Register(Tool{Name: "hotkey", Description: "Press a key combination like ctrl+a, ctrl+l, alt+tab. Args: modifier, key", Dangerous: false}, toolHotKey)
	r.Register(Tool{Name: "move_mouse", Description: "Move mouse cursor to screen coordinates without clicking. Args: x, y (pixels)", Dangerous: false}, toolMoveMouse)
	r.Register(Tool{Name: "scroll", Description: "Scroll up or down. Args: direction (up/down), amount (lines, default 3)", Dangerous: false}, toolScroll)
	r.Register(Tool{Name: "screen_info", Description: "Get screen resolution and current cursor position. No args.", Dangerous: false}, toolGetScreenInfo)
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool, fn ToolFunc) {
	r.tools[tool.Name] = registeredTool{Tool: tool, Fn: fn}
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, rt := range r.tools {
		result = append(result, rt.Tool)
	}
	return result
}

// IsDangerous checks if a tool requires user confirmation.
// Unknown tools return false — they will just fail at execution.
func (r *Registry) IsDangerous(name string) bool {
	rt, ok := r.tools[name]
	if !ok {
		return false
	}
	return rt.Tool.Dangerous
}

// Execute runs a tool by name with the given arguments.
func (r *Registry) Execute(call ToolCall) ToolResult {
	rt, ok := r.tools[call.Name]
	if !ok {
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", call.Name)}
	}
	return rt.Fn(call.Args)
}

// BuildToolPrompt generates the system prompt section describing available tools.
func (r *Registry) BuildToolPrompt() string {
	var sb strings.Builder
	sb.WriteString("You have access to the following tools to interact with the user's computer:\n\n")

	for _, rt := range r.tools {
		danger := ""
		if rt.Tool.Dangerous {
			danger = " [REQUIRES CONFIRMATION]"
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s%s\n", rt.Tool.Name, rt.Tool.Description, danger))
	}

	sb.WriteString(`
When you need to use a tool, output it in this exact format:

<tool_call>
{"name": "tool_name", "args": {"arg1": "value1", "arg2": "value2"}}
</tool_call>

You can include text before the tool call to explain what you're doing.
After the tool executes, you will receive the result and can respond to the user.
Only use tools when the user explicitly asks you to perform an action.
Never use tools without being asked.
For dangerous actions (delete, create, run), always explain what you will do before calling the tool.
`)
	return sb.String()
}

// ParseToolCalls extracts tool calls from an AI response.
func ParseToolCalls(response string) []ToolCall {
	var calls []ToolCall
	remaining := response

	for {
		startIdx := strings.Index(remaining, "<tool_call>")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(remaining[startIdx:], "</tool_call>")
		if endIdx == -1 {
			break
		}

		jsonStr := strings.TrimSpace(remaining[startIdx+len("<tool_call>") : startIdx+endIdx])

		var call ToolCall
		if err := json.Unmarshal([]byte(jsonStr), &call); err == nil {
			if call.Name != "" {
				calls = append(calls, call)
			}
		}

		remaining = remaining[startIdx+endIdx+len("</tool_call>"):]
	}

	return calls
}

// ExtractTextBeforeToolCall returns the text portion before the first tool call.
func ExtractTextBeforeToolCall(response string) string {
	idx := strings.Index(response, "<tool_call>")
	if idx == -1 {
		return strings.TrimSpace(response)
	}
	return strings.TrimSpace(response[:idx])
}
