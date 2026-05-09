package tools

import "context"

// ToolAction is the signature for the function that performs a tool's work.
//
// The arguments parameter is a JSON string containing the key/value pairs
// chosen by the model. The returned string is sent back to the model as the
// content of the tool result message.
type ToolAction func(ctx context.Context, arguments string) (string, error)

// Tool is the interface that every callable tool must implement.
//
// Implementing this interface is all that is required to register a new tool
// with the Registry and expose it to the LLM.
type Tool interface {
	// GetName returns the unique name of the tool.
	// It is used as the function name in OpenAI tool definitions and must match
	// the name the model sends back in a tool call.
	GetName() string

	// GetDescription returns a human-readable description of what the tool does.
	// This text is included in the tool definition sent to the model.
	GetDescription() string

	// GetParameters returns the JSON Schema object describing the tool's
	// parameters. It should marshal to a valid JSON Schema object such as:
	//
	//     {"type": "object", "properties": {...}, "required": [...]}
	//
	// Return nil or an empty map if the tool takes no arguments.
	GetParameters() map[string]interface{}

	// GetAction returns the function that actually performs the tool's work.
	GetAction() ToolAction
}
