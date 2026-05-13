package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/tuffrabit/flamingode/internal/apiclient"
)

// mockTool is a minimal Tool implementation used only to exercise the framework.
type mockTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	action      ToolAction
}

func (m *mockTool) GetName() string                      { return m.name }
func (m *mockTool) GetDescription() string                { return m.description }
func (m *mockTool) GetParameters() map[string]interface{} { return m.parameters }
func (m *mockTool) GetAction() ToolAction                 { return m.action }
func (m *mockTool) GetPermissionRequired() bool           { return false }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "test_tool"}

	r.Register(tool)

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if got.GetName() != "test_tool" {
		t.Errorf("expected name test_tool, got %s", got.GetName())
	}

	_, ok = r.Get("missing")
	if ok {
		t.Error("expected missing tool to not be found")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "a"})
	r.Register(&mockTool{name: "b"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list))
	}
}

func TestRegistry_ToOpenAITools(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name:        "read_file",
		description: "Read a file.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path"},
		},
	})

	apiTools := r.ToOpenAITools()
	if len(apiTools) != 1 {
		t.Fatalf("expected 1 api tool, got %d", len(apiTools))
	}

	tool := apiTools[0]
	if tool.Type != "function" {
		t.Errorf("expected type function, got %s", tool.Type)
	}
	if tool.Function.Name != "read_file" {
		t.Errorf("expected name read_file, got %s", tool.Function.Name)
	}
	if tool.Function.Description != "Read a file." {
		t.Errorf("unexpected description: %s", tool.Function.Description)
	}
	if tool.Function.Parameters == nil {
		t.Error("expected parameters to be non-nil")
	}
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "echo",
		action: func(_ context.Context, arguments string) (string, error) {
			var args map[string]string
			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return "", err
			}
			return args["text"], nil
		},
	})

	result, err := r.Execute(context.Background(), "echo", `{"text":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected hello, got %s", result)
	}
}

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "unknown", `{}`)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_ExecuteToolCall(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "add",
		action: func(_ context.Context, arguments string) (string, error) {
			var args map[string]int
			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return "", err
			}
			sum := map[string]int{"sum": args["a"] + args["b"]}
			out, _ := json.Marshal(sum)
			return string(out), nil
		},
	})

	tc := apiclient.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: apiclient.FunctionCall{
			Name:      "add",
			Arguments: `{"a":2,"b":3}`,
		},
	}

	result, err := r.ExecuteToolCall(context.Background(), tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"sum":5}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRegistry_Execute_ActionError(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "fail",
		action: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("something went wrong")
		},
	})

	_, err := r.Execute(context.Background(), "fail", `{}`)
	if err == nil {
		t.Fatal("expected error from action")
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := NewToolResultMessage("call_abc", "ok")
	if msg.Role != "tool" {
		t.Errorf("expected role tool, got %s", msg.Role)
	}
	if msg.Content != "ok" {
		t.Errorf("expected content ok, got %s", msg.Content)
	}
	if msg.ToolCallID != "call_abc" {
		t.Errorf("expected tool_call_id call_abc, got %s", msg.ToolCallID)
	}
}
