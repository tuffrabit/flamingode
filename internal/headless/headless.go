package headless

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/config"
	"github.com/tuffrabit/flamingode/internal/session"
	"github.com/tuffrabit/flamingode/internal/tools"
	"github.com/tuffrabit/flamingode/internal/ui"
)

func estimateTokens(msgs []apiclient.ChatCompletionMessage) int {
	tokens := 0
	for _, msg := range msgs {
		tokens += len(msg.Content) / 3
		tokens += len(msg.ReasoningContent) / 3
		for _, tc := range msg.ToolCalls {
			tokens += len(tc.Function.Name) / 3
			tokens += len(tc.Function.Arguments) / 3
		}
		if msg.ToolCallID != "" {
			tokens += len(msg.ToolCallID) / 3
		}
	}
	tokens += len(msgs) * 4
	return tokens
}

func executeToolCall(r *tools.Registry, messages []apiclient.ChatCompletionMessage, contextWindow int, tc apiclient.ToolCall) string {
	remainingTokens := 0
	if contextWindow > 0 {
		toolOverhead := len(r.List()) * 200
		remainingTokens = int(float64(contextWindow)*0.75) - estimateTokens(messages) - toolOverhead
	}

	if contextWindow > 0 && remainingTokens <= 0 {
		return "[Result omitted: context window exhausted. Consider requesting a smaller range or summarizing.]"
	}

	var origMaxSize int64
	if t, ok := r.Get("read_file"); ok {
		if rf, ok := t.(*tools.ReadFile); ok && contextWindow > 0 {
			origMaxSize = rf.MaxSize
			safeBytes := int64(remainingTokens * 4)
			if safeBytes > 0 && (safeBytes < rf.MaxSize || rf.MaxSize == 0) {
				rf.MaxSize = safeBytes
			}
		}
	}

	result, err := r.ExecuteToolCall(context.Background(), tc)
	if err != nil {
		result = fmt.Sprintf("error: %v", err)
	}

	if t, ok := r.Get("read_file"); ok {
		if rf, ok := t.(*tools.ReadFile); ok {
			rf.MaxSize = origMaxSize
		}
	}

	if contextWindow > 0 {
		resultTokens := len(result) / 4
		if resultTokens > remainingTokens {
			maxBytes := remainingTokens * 4
			if maxBytes > 0 && maxBytes < len(result) {
				result = result[:maxBytes] + fmt.Sprintf("\n[Result truncated: exceeded available context. Remaining: ~%d tokens]", remainingTokens)
			}
		}
	}

	return result
}

// Run executes a single headless session with the given prompt and prints the
// assistant's final response to stdout.
func Run(cfg config.Config, modelID string, prompt string, yolo bool) error {
	client, resolvedModelID, contextWindow, status := ui.ResolveModelByID(cfg, modelID)
	if status == "no model selected" {
		return fmt.Errorf("no model selected")
	}

	wd, _ := os.Getwd()

	sess, err := session.NewSession(modelID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	var messages []apiclient.ChatCompletionMessage
	var sessionUsage apiclient.Usage

	systemPrompt := ui.ResolveSystemPrompt(wd)
	messages = []apiclient.ChatCompletionMessage{
		apiclient.NewTextMessage("system", systemPrompt),
	}
	_ = sess.AppendEvent(session.EventFromMessage(messages[0]))

	resolvedPrompt := ui.ResolveAtMentions(prompt, wd, cfg.Tools.ReadFile.MaxSize)
	userMsg := apiclient.NewTextMessage("user", resolvedPrompt)
	messages = append(messages, userMsg)
	_ = sess.AppendEvent(session.EventFromMessage(userMsg))

	r := tools.NewRegistry()
	r.Register(&tools.ListDirectory{WorkingDir: wd})
	r.Register(&tools.ReadFile{WorkingDir: wd, MaxSize: cfg.Tools.ReadFile.MaxSize})
	r.Register(&tools.WriteFile{WorkingDir: wd})
	r.Register(&tools.ReplaceText{WorkingDir: wd})
	r.Register(&tools.ExecCommand{WorkingDir: wd, Timeout: time.Duration(cfg.Tools.ExecCommand.TimeoutSeconds) * time.Second})
	r.Register(&tools.Grep{WorkingDir: wd})
	r.Register(&tools.Glob{WorkingDir: wd})

	for {
		req := apiclient.ChatCompletionRequest{
			Model:         resolvedModelID,
			Messages:      messages,
			Stream:        true,
			StreamOptions: &apiclient.StreamOptions{IncludeUsage: true},
		}
		req.Tools = r.ToOpenAITools()

		stream, err := client.CreateChatCompletionStream(context.Background(), req)
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		var pending string
		var pendingThinking string
		var pendingToolCalls []apiclient.ToolCall
		var streamUsageRecorded bool

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				_ = stream.Close()
				break
			}
			if err != nil {
				_ = stream.Close()
				return fmt.Errorf("stream read error: %w", err)
			}

			if chunk.Usage != nil && !streamUsageRecorded {
				sessionUsage.PromptTokens += chunk.Usage.PromptTokens
				sessionUsage.CompletionTokens += chunk.Usage.CompletionTokens
				sessionUsage.TotalTokens += chunk.Usage.TotalTokens
				streamUsageRecorded = true
			}

			if len(chunk.Choices) > 0 {
				pending += chunk.Choices[0].Delta.Content
				pendingThinking += chunk.Choices[0].Delta.ReasoningContent
				for _, tc := range chunk.Choices[0].Delta.ToolCalls {
					if tc.Index >= len(pendingToolCalls) {
						pendingToolCalls = append(pendingToolCalls, make([]apiclient.ToolCall, tc.Index-len(pendingToolCalls)+1)...)
					}
					if tc.ID != "" {
						pendingToolCalls[tc.Index].ID = tc.ID
					}
					if tc.Type != "" {
						pendingToolCalls[tc.Index].Type = tc.Type
					}
					if tc.Function.Name != "" {
						pendingToolCalls[tc.Index].Function.Name = tc.Function.Name
					}
					pendingToolCalls[tc.Index].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		if len(pendingToolCalls) > 0 {
			assistantMsg := apiclient.ChatCompletionMessage{
				Role:             "assistant",
				Content:          pending,
				ReasoningContent: pendingThinking,
				ToolCalls:        pendingToolCalls,
			}
			messages = append(messages, assistantMsg)
			_ = sess.AppendEvent(session.EventFromMessage(assistantMsg))

			for _, tc := range pendingToolCalls {
				tool, ok := r.Get(tc.Function.Name)
				if !ok {
					result := fmt.Sprintf("error: tool %q not found", tc.Function.Name)
					messages = append(messages, tools.NewToolResultMessage(tc.ID, result))
					_ = sess.AppendEvent(session.EventFromMessage(tools.NewToolResultMessage(tc.ID, result)))
					continue
				}

				if tool.GetPermissionRequired() && !yolo {
					result := "Permission denied. The user declined this tool call. Please find another way to accomplish the task."
					messages = append(messages, tools.NewToolResultMessage(tc.ID, result))
					_ = sess.AppendEvent(session.EventFromMessage(tools.NewToolResultMessage(tc.ID, result)))
					continue
				}

				result := executeToolCall(r, messages, contextWindow, tc)
				messages = append(messages, tools.NewToolResultMessage(tc.ID, result))
				_ = sess.AppendEvent(session.EventFromMessage(tools.NewToolResultMessage(tc.ID, result)))
			}

			sess.Header.Usage = sessionUsage
			_ = sess.UpdateHeader()
			continue
		}

		assistantMsg := apiclient.ChatCompletionMessage{
			Role:             "assistant",
			Content:          pending,
			ReasoningContent: pendingThinking,
		}
		messages = append(messages, assistantMsg)
		_ = sess.AppendEvent(session.EventFromMessage(assistantMsg))

		sess.Header.Usage = sessionUsage
		_ = sess.UpdateHeader()

		fmt.Println(pending)
		return nil
	}
}
