package tools

import (
	"context"
	"fmt"

	"github.com/tuffrabit/flamingode/internal/apiclient"
)

// Registry holds registered tools and provides conversions to API types.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.GetName()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	list := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// ToOpenAITools converts all registered tools into the API representation
// used by the OpenAI Chat Completions request body.
func (r *Registry) ToOpenAITools() []apiclient.Tool {
	out := make([]apiclient.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, apiclient.Tool{
			Type: "function",
			Function: apiclient.FunctionDefinition{
				Name:        t.GetName(),
				Description: t.GetDescription(),
				Parameters:  t.GetParameters(),
			},
		})
	}
	return out
}

// Execute runs the tool identified by name with the supplied JSON arguments.
func (r *Registry) Execute(ctx context.Context, name string, arguments string) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}
	return t.GetAction()(ctx, arguments)
}

// ExecuteToolCall is a convenience wrapper that extracts the name and
// arguments from an API ToolCall and executes the matching tool.
func (r *Registry) ExecuteToolCall(ctx context.Context, tc apiclient.ToolCall) (string, error) {
	return r.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
}
