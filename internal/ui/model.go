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
	"github.com/tuffrabit/flamingode/internal/session"
	"github.com/tuffrabit/flamingode/internal/tools"
)

type MainViewModel struct {
	ready               bool
	viewport            viewport.Model
	textInput           textarea.Model
	windowWidth         int
	windowHeight        int
	client              *apiclient.Client
	modelID             string
	fullModelID         string
	contextWindow       int
	status              string
	workingDir          string
	messages            []apiclient.ChatCompletionMessage
	pending             string
	pendingThinking     string
	streaming           bool
	spinner             spinner.Model
	toolRegistry        *tools.Registry
	pendingToolCalls    []apiclient.ToolCall
	sessionUsage        apiclient.Usage
	streamUsageRecorded bool
	session             *session.Session
	sessionID           string

	// Permission prompt state
	permissionPrompt *PermissionPrompt
	queuedToolCalls  []apiclient.ToolCall
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

func (m *MainViewModel) persistMessage(msg apiclient.ChatCompletionMessage) {
	if m.session == nil {
		return
	}
	_ = m.session.AppendEvent(session.EventFromMessage(msg))
}

func (m *MainViewModel) persistError(err error) {
	if m.session == nil {
		return
	}
	evt := session.Event{
		Type:    "error",
		Role:    "assistant",
		Content: fmt.Sprintf("[error: %v]", err),
	}
	_ = m.session.AppendEvent(evt)
}

func (m *MainViewModel) updateSessionUsage() {
	if m.session == nil {
		return
	}
	m.session.Header.Usage = m.sessionUsage
	_ = m.session.UpdateHeader()
}

func (m *MainViewModel) handleSlashCommand(input string) bool {
	switch input {
	case "/clear":
		newSess, err := session.NewSession(m.fullModelID)
		if err != nil {
			m.messages = append(m.messages, apiclient.NewTextMessage("assistant", fmt.Sprintf("[error: failed to create new session: %v]", err)))
			return true
		}
		m.session = newSess
		m.sessionID = newSess.Header.SessionID
		m.sessionUsage = apiclient.Usage{}
		m.streamUsageRecorded = false
		m.messages = []apiclient.ChatCompletionMessage{
			apiclient.NewTextMessage("system", "You are a helpful agent"),
		}
		_ = m.session.AppendEvent(session.EventFromMessage(m.messages[0]))
		m.pending = ""
		m.pendingThinking = ""
		m.pendingToolCalls = nil
		m.queuedToolCalls = nil
		return true
	}
	return false
}

func (m MainViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Permission prompt takes precedence over normal UI.
	if m.permissionPrompt != nil {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "ctrl+d" {
			return m, tea.Quit
		}

		var pCmd tea.Cmd
		m.permissionPrompt, pCmd = m.permissionPrompt.Update(msg)
		cmds = append(cmds, pCmd)

		if m.permissionPrompt != nil && m.permissionPrompt.done {
			choice := m.permissionPrompt.approved
			tc := m.permissionPrompt.toolCall
			m.permissionPrompt = nil
			cmds = append(cmds, m.handlePermissionChoice(choice, tc))
		}

		// Still allow viewport scrolling while the prompt is shown.
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)
	}

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
			if m.handleSlashCommand(strings.TrimSpace(input)) {
				m.viewport.SetContent(m.renderChat())
				m.viewport.GotoBottom()
				break
			}
			userMsg := apiclient.NewTextMessage("user", input)
			m.messages = append(m.messages, userMsg)
			m.persistMessage(userMsg)
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
		var bottomHeight int
		if m.permissionPrompt != nil {
			bottomHeight = lipgloss.Height(m.permissionPrompt.View())
		} else {
			bottomHeight = lipgloss.Height(m.textInput.View())
		}
		verticalMarginHeight := headerHeight + bottomHeight

		if !m.ready {
			m.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(msg.Height-verticalMarginHeight),
			)
			m.viewport.YPosition = headerHeight
			m.viewport.KeyMap = viewport.KeyMap{}
			m.ready = true
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
		} else {
			wasAtBottom := m.viewport.AtBottom()
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(msg.Height - verticalMarginHeight)
			m.viewport.SetContent(m.renderChat())
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
		}

	case streamMsg:
		if msg.err != nil {
			m.streaming = false
			m.messages = append(m.messages, apiclient.NewTextMessage("assistant", fmt.Sprintf("[error: %v]", msg.err)))
			m.persistError(msg.err)
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
			m.updateSessionUsage()
		}
		if msg.done {
			if len(m.pendingToolCalls) > 0 {
				assistantMsg := apiclient.ChatCompletionMessage{
					Role:             "assistant",
					Content:          m.pending,
					ReasoningContent: m.pendingThinking,
					ToolCalls:        m.pendingToolCalls,
				}
				m.messages = append(m.messages, assistantMsg)
				m.persistMessage(assistantMsg)

				m.queuedToolCalls = m.pendingToolCalls
				m.pendingToolCalls = nil
				m.pending = ""
				m.pendingThinking = ""
				m.streaming = false

				cmd := m.processToolCallQueue()
				cmds = append(cmds, cmd)
				m.viewport.SetContent(m.renderChat())
				m.viewport.GotoBottom()
				break
			}
			m.streaming = false
			assistantMsg := apiclient.ChatCompletionMessage{
				Role:             "assistant",
				Content:          m.pending,
				ReasoningContent: m.pendingThinking,
			}
			m.messages = append(m.messages, assistantMsg)
			m.persistMessage(assistantMsg)
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
		var bottomHeight int
		if m.permissionPrompt != nil {
			bottomHeight = lipgloss.Height(m.permissionPrompt.View())
		} else {
			bottomHeight = lipgloss.Height(m.textInput.View())
		}
		verticalMarginHeight := headerHeight + bottomHeight
		newViewportHeight := m.windowHeight - verticalMarginHeight
		if newViewportHeight > 0 && newViewportHeight != m.viewport.Height() {
			wasAtBottom := m.viewport.AtBottom()
			m.viewport.SetHeight(newViewportHeight)
			m.viewport.SetContent(m.renderChat())
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
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

func (m *MainViewModel) executeToolCall(tc apiclient.ToolCall) string {
	remainingTokens := 0
	if m.contextWindow > 0 {
		// Cap at 75% of context window to leave headroom for the model's
		// response and estimation error. Also subtract rough tool schema overhead.
		toolOverhead := len(m.toolRegistry.List()) * 200
		remainingTokens = int(float64(m.contextWindow)*0.75) - estimateTokens(m.messages) - toolOverhead
	}

	if m.contextWindow > 0 && remainingTokens <= 0 {
		return "[Result omitted: context window exhausted. Consider requesting a smaller range or summarizing.]"
	}

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

	result, err := m.toolRegistry.ExecuteToolCall(context.Background(), tc)
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

	return result
}

func (m *MainViewModel) processToolCallQueue() tea.Cmd {
	for len(m.queuedToolCalls) > 0 {
		tc := m.queuedToolCalls[0]
		m.queuedToolCalls = m.queuedToolCalls[1:]

		tool, ok := m.toolRegistry.Get(tc.Function.Name)
		if !ok {
			result := fmt.Sprintf("error: tool %q not found", tc.Function.Name)
			m.messages = append(m.messages, tools.NewToolResultMessage(tc.ID, result))
			m.persistMessage(tools.NewToolResultMessage(tc.ID, result))
			continue
		}

		if tool.GetPermissionRequired() {
			m.permissionPrompt = NewPermissionPrompt(tc)
			return nil
		}

		result := m.executeToolCall(tc)
		m.messages = append(m.messages, tools.NewToolResultMessage(tc.ID, result))
		m.persistMessage(tools.NewToolResultMessage(tc.ID, result))
	}

	m.streaming = true
	m.streamUsageRecorded = false
	return tea.Batch(m.startStream(), m.spinner.Tick)
}

func (m *MainViewModel) handlePermissionChoice(approved bool, tc apiclient.ToolCall) tea.Cmd {
	m.permissionPrompt = nil

	var result string
	if approved {
		result = m.executeToolCall(tc)
	} else {
		result = "Permission denied. The user declined this tool call. Please find another way to accomplish the task."
	}

	m.messages = append(m.messages, tools.NewToolResultMessage(tc.ID, result))
	m.persistMessage(tools.NewToolResultMessage(tc.ID, result))
	return m.processToolCallQueue()
}

func (m MainViewModel) View() tea.View {
	var c *tea.Cursor
	if !m.textInput.VirtualCursor() && m.permissionPrompt == nil {
		c = m.textInput.Cursor()
		c.Y += lipgloss.Height(m.headerView()) + m.viewport.Height()
	}

	var content string
	if !m.ready {
		content = m.headerView() + "\n\n Initializing..."
	} else {
		var bottomSection string
		if m.permissionPrompt != nil {
			bottomSection = m.permissionPrompt.View()
		} else {
			bottomSection = m.textInput.View()
		}
		content = lipgloss.JoinVertical(
			lipgloss.Top,
			m.headerView(),
			m.viewport.View(),
			bottomSection,
		)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = c
	return v
}
