package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
)

type MainViewModel struct {
	ready        bool
	viewport     viewport.Model
	textInput    textarea.Model
	windowWidth  int
	windowHeight int
	client       *apiclient.Client
	modelID      string
	status       string
	messages     []apiclient.ChatCompletionMessage
	pending      string
	streaming    bool
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
			m.streaming = true
			cmds = append(cmds, m.startStream())
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		m.textInput.SetWidth(msg.Width)

		headerHeight := lipgloss.Height(m.headerView())
		textInputHeight := lipgloss.Height(m.textInput.View())
		statusHeight := lipgloss.Height(m.statusView())
		verticalMarginHeight := headerHeight + textInputHeight + statusHeight

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
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			break
		}
		if msg.done {
			m.streaming = false
			m.messages = append(m.messages, apiclient.NewTextMessage("assistant", m.pending))
			m.pending = ""
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			break
		}
		m.pending += msg.chunk
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
		statusHeight := lipgloss.Height(m.statusView())
		verticalMarginHeight := headerHeight + textInputHeight + statusHeight
		newViewportHeight := m.windowHeight - verticalMarginHeight
		if newViewportHeight > 0 && newViewportHeight != m.viewport.Height() {
			m.viewport.SetHeight(newViewportHeight)
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
		}
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
			m.statusView(),
		)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = c
	return v
}
