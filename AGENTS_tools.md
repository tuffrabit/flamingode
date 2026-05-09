# Tool Framework

## Philosophy

A tool in **flamingode** is anything that implements the `Tool` interface. There is no code generation, no reflection, and no central switch statement — you write a regular Go struct, satisfy four getters, and register it. That’s it.

The framework is intentionally thin. It only bridges the gap between the OpenAI Chat Completions API and your own Go code.

## Interface

```go
package tools

// ToolAction performs the actual work.
// arguments is the JSON string the model produced.
// The returned string becomes the content of the tool result message.
type ToolAction func(ctx context.Context, arguments string) (string, error)

// Tool is the interface every callable tool must implement.
type Tool interface {
    GetName() string
    GetDescription() string
    GetParameters() map[string]interface{}
    GetAction() ToolAction
}
```

| Method | Maps to OpenAI | Purpose |
|--------|---------------|---------|
| `GetName()` | `function.name` | Unique identifier. The model uses this to request the tool. |
| `GetDescription()` | `function.description` | Human-readable explanation shown to the model. |
| `GetParameters()` | `function.parameters` | JSON Schema object describing the arguments the model may supply. |
| `GetAction()` | — | Returns the function that actually runs when the model calls the tool. |

## Registry

The `Registry` holds tools and converts them into API types.

```go
r := tools.NewRegistry()
r.Register(&MyTool{})

// Attach to a chat-completion request.
req := apiclient.ChatCompletionRequest{
    Model:    "gpt-4o",
    Messages: msgs,
    Tools:    r.ToOpenAITools(),
}
```

Dispatching a tool call from the model:

```go
for _, tc := range assistantMsg.ToolCalls {
    result, err := r.ExecuteToolCall(ctx, tc)
    if err != nil {
        result = fmt.Sprintf("error: %v", err)
    }
    msgs = append(msgs, tools.NewToolResultMessage(tc.ID, result))
}
```

## Example Tool Implementation

```go
package mytools

import (
    "context"
    "encoding/json"
    "os"

    "github.com/tuffrabit/flamingode/internal/tools"
)

// ReadFile lets the model read the contents of a file.
type ReadFile struct{}

func (r *ReadFile) GetName() string {
    return "read_file"
}

func (r *ReadFile) GetDescription() string {
    return "Read a file's contents."
}

func (r *ReadFile) GetParameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "path": map[string]interface{}{
                "type":        "string",
                "description": "Absolute or relative path to the file.",
            },
        },
        "required": []string{"path"},
    }
}

func (r *ReadFile) GetAction() tools.ToolAction {
    return func(ctx context.Context, arguments string) (string, error) {
        var args struct {
            Path string `json:"path"`
        }
        if err := json.Unmarshal([]byte(arguments), &args); err != nil {
            return "", err
        }
        b, err := os.ReadFile(args.Path)
        return string(b), err
    }
}
```

Register it once at startup:

```go
r.Register(&mytools.ReadFile{})
```

## Tool Result Messages

After a tool runs you must append a message with `role: "tool"` back into the conversation. Use the helper so the fields are wired correctly:

```go
msg := tools.NewToolResultMessage(toolCallID, content)
```

This produces an `apiclient.ChatCompletionMessage` ready to be appended to the message slice and sent back to the model.
