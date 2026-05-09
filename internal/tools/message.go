package tools

import "github.com/tuffrabit/flamingode/internal/apiclient"

// NewToolResultMessage creates a ChatCompletionMessage with role "tool"
// that carries the result of a single tool invocation.
//
// The content string is typically the raw output returned by ToolAction.
func NewToolResultMessage(toolCallID string, content string) apiclient.ChatCompletionMessage {
	return apiclient.ChatCompletionMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}
