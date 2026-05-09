package ui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
)

type streamMsg struct {
	chunk         string
	thinkingChunk string
	done          bool
	err           error
	stream        *apiclient.ChatCompletionStream
	toolCalls     []apiclient.ToolCall
	finishReason  string
}

func (m MainViewModel) startStream() tea.Cmd {
	return func() tea.Msg {
		req := apiclient.ChatCompletionRequest{
			Model:    m.modelID,
			Messages: m.messages,
			Stream:   true,
		}
		if m.toolRegistry != nil {
			req.Tools = m.toolRegistry.ToOpenAITools()
		}
		stream, err := m.client.CreateChatCompletionStream(context.Background(), req)
		if err != nil {
			return streamMsg{err: err}
		}
		return m.readStream(stream)()
	}
}

func (m MainViewModel) readStream(stream *apiclient.ChatCompletionStream) tea.Cmd {
	return func() tea.Msg {
		chunk, err := stream.Recv()
		if err == io.EOF {
			_ = stream.Close()
			return streamMsg{done: true}
		}
		if err != nil {
			_ = stream.Close()
			return streamMsg{err: err}
		}
		var content, thinking, finishReason string
		var tcs []apiclient.ToolCall
		if len(chunk.Choices) > 0 {
			content = chunk.Choices[0].Delta.Content
			thinking = chunk.Choices[0].Delta.ReasoningContent
			tcs = chunk.Choices[0].Delta.ToolCalls
			finishReason = chunk.Choices[0].FinishReason
		}
		return streamMsg{chunk: content, thinkingChunk: thinking, stream: stream, toolCalls: tcs, finishReason: finishReason}
	}
}
