package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/tools"
)

type MainViewModel struct {
	ready            bool
	viewport         viewport.Model
	textInput        textarea.Model
	windowWidth      int
	windowHeight     int
	client           *apiclient.Client
	modelID          string
	contextWindow    int
	status           string
	workingDir       string
	messages         []apiclient.ChatCompletionMessage
	pending          string
	pendingThinking  string
	streaming        bool
	spinner          spinner.Model
	toolRegistry     *tools.Registry
	pendingToolCalls    []apiclient.ToolCall
	sessionUsage        apiclient.Usage
	streamUsageRecorded bool
}

func estimateTokens(msgs []apiclient.ChatCompletionMessage) int {
	tokens := 0
	for _, msg := range msgs {
		// Use bytes/3 instead of bytes/4 for a more conservative estimate
		// (code and special characters often tokenize at ~3 bytes/token).
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
	// Rough overhead per message for role/formatting.
	tokens += len(msgs) * 4
	return tokens
}

func (m MainViewModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m MainViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit
		case "enter":
			if m.streaming || m.client == nil {
				break
			}
			input := m.textInput.Value()
			if strings.TrimSpace(input) == "" {
				break
			}
			m.textInput.SetValue("")
			m.messages = append(m.messages, apiclient.NewTextMessage("user", input))
			m.pending = ""
			m.pendingThinking = ""
			m.streaming = true
			m.streamUsageRecorded = false
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			cmds = append(cmds, m.startStream(), m.spinner.Tick)
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		m.textInput.SetWidth(msg.Width)

		headerHeight := lipgloss.Height(m.headerView())
		textInputHeight := lipgloss.Height(m.textInput.View())
		verticalMarginHeight := headerHeight + textInputHeight

		if !m.ready {
			m.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(msg.Height-verticalMarginHeight),
			)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(msg.Height - verticalMarginHeight)
		}
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()

	case streamMsg:
		if msg.err != nil {
			m.streaming = false
			m.messages = append(m.messages, apiclient.NewTextMessage("assistant", fmt.Sprintf("[error: %v]", msg.err)))
			m.pending = ""
			m.pendingThinking = ""
			m.pendingToolCalls = nil
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			break
		}
		// Accumulate usage only once per stream. Some OpenAI-compatible servers
		// (including certain llama.cpp versions) send usage in multiple chunks.
		if msg.usage != nil && !m.streamUsageRecorded {
			m.sessionUsage.PromptTokens += msg.usage.PromptTokens
			m.sessionUsage.CompletionTokens += msg.usage.CompletionTokens
			m.sessionUsage.TotalTokens += msg.usage.TotalTokens
			m.streamUsageRecorded = true
		}
		if msg.done {
			if len(m.pendingToolCalls) > 0 {
				m.messages = append(m.messages, apiclient.ChatCompletionMessage{
					Role:             "assistant",
					Content:          m.pending,
					ReasoningContent: m.pendingThinking,
					ToolCalls:        m.pendingToolCalls,
				})
				for _, tc := range m.pendingToolCalls {
					remainingTokens := 0
					if m.contextWindow > 0 {
						// Cap at 75% of context window to leave headroom for the model's
						// response and estimation error. Also subtract rough tool schema overhead.
						toolOverhead := len(m.toolRegistry.List()) * 200
						remainingTokens = int(float64(m.contextWindow)*0.75) - estimateTokens(m.messages) - toolOverhead
					}

					var result string
					if m.contextWindow > 0 && remainingTokens <= 0 {
						result = "[Result omitted: context window exhausted. Consider requesting a smaller range or summarizing.]"
					} else {
						var origMaxSize int64
						if t, ok := m.toolRegistry.Get("read_file"); ok {
							if rf, ok := t.(*tools.ReadFile); ok && m.contextWindow > 0 {
								origMaxSize = rf.MaxSize
								safeBytes := int64(remainingTokens * 4)
								if safeBytes > 0 && (safeBytes < rf.MaxSize || rf.MaxSize == 0) {
									rf.MaxSize = safeBytes
								}
							}
						}

						var err error
						result, err = m.toolRegistry.ExecuteToolCall(context.Background(), tc)
						if err != nil {
							result = fmt.Sprintf("error: %v", err)
						}

						if t, ok := m.toolRegistry.Get("read_file"); ok {
							if rf, ok := t.(*tools.ReadFile); ok {
								rf.MaxSize = origMaxSize
							}
						}

						if m.contextWindow > 0 {
							resultTokens := len(result) / 4
							if resultTokens > remainingTokens {
								maxBytes := remainingTokens * 4
								if maxBytes > 0 && maxBytes < len(result) {
									result = result[:maxBytes] + fmt.Sprintf("\n[Result truncated: exceeded available context. Remaining: ~%d tokens]", remainingTokens)
								}
							}
						}
					}

					m.messages = append(m.messages, tools.NewToolResultMessage(tc.ID, result))
				}
				m.pendingToolCalls = nil
				m.pending = ""
				m.pendingThinking = ""
				m.streaming = true
				m.streamUsageRecorded = false
				cmds = append(cmds, m.startStream(), m.spinner.Tick)
				m.viewport.SetContent(m.renderChat())
				m.viewport.GotoBottom()
				break
			}
			m.streaming = false
			m.messages = append(m.messages, apiclient.ChatCompletionMessage{
				Role:             "assistant",
				Content:          m.pending,
				ReasoningContent: m.pendingThinking,
			})
			m.pending = ""
			m.pendingThinking = ""
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			break
		}
		m.pending += msg.chunk
		m.pendingThinking += msg.thinkingChunk
		for _, tc := range msg.toolCalls {
			if tc.Index >= len(m.pendingToolCalls) {
				m.pendingToolCalls = append(m.pendingToolCalls, make([]apiclient.ToolCall, tc.Index-len(m.pendingToolCalls)+1)...)
			}
			if tc.ID != "" {
				m.pendingToolCalls[tc.Index].ID = tc.ID
			}
			if tc.Type != "" {
				m.pendingToolCalls[tc.Index].Type = tc.Type
			}
			if tc.Function.Name != "" {
				m.pendingToolCalls[tc.Index].Function.Name = tc.Function.Name
			}
			m.pendingToolCalls[tc.Index].Function.Arguments += tc.Function.Arguments
		}
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		cmds = append(cmds, m.readStream(msg.stream))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	if m.ready {
		headerHeight := lipgloss.Height(m.headerView())
		textInputHeight := lipgloss.Height(m.textInput.View())
		verticalMarginHeight := headerHeight + textInputHeight
		newViewportHeight := m.windowHeight - verticalMarginHeight
		if newViewportHeight > 0 && newViewportHeight != m.viewport.Height() {
			m.viewport.SetHeight(newViewportHeight)
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
		}
	}

	if m.streaming {
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinCmd)
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
	}

	return m, tea.Batch(cmds...)
}

func (m MainViewModel) View() tea.View {
	var c *tea.Cursor
	if !m.textInput.VirtualCursor() {
		c = m.textInput.Cursor()
		c.Y += lipgloss.Height(m.headerView()) + m.viewport.Height()
	}

	var content string
	if !m.ready {
		content = m.headerView() + "\n\n Initializing..."
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Top,
			m.headerView(),
			m.viewport.View(),
			m.textInput.View(),
		)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = c
	return v
}
