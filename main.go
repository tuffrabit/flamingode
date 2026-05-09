package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/config"
)

var Version string

func getVersion() string {
	if Version != "" {
		return Version
	}
	return "dev"
}

type streamMsg struct {
	chunk  string
	done   bool
	err    error
	stream *apiclient.ChatCompletionStream
}

type MainViewModel struct {
	ready     bool
	viewport  viewport.Model
	textInput textinput.Model
	client    *apiclient.Client
	modelID   string
	status    string
	messages  []apiclient.ChatCompletionMessage
	pending   string
	streaming bool
}

func (m MainViewModel) Init() tea.Cmd {
	return textinput.Blink
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

	return m, tea.Batch(cmds...)
}

func (m MainViewModel) renderChat() string {
	var b strings.Builder
	for _, msg := range m.messages {
		prefix := "You: "
		if msg.Role == "assistant" {
			prefix = "Assistant: "
		} else if msg.Role == "system" {
			continue
		}
		b.WriteString(prefix)
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}
	if m.streaming || m.pending != "" {
		b.WriteString("Assistant: ")
		b.WriteString(m.pending)
		if m.streaming {
			b.WriteString("█")
		}
	}
	return b.String()
}

func (m MainViewModel) startStream() tea.Cmd {
	return func() tea.Msg {
		req := apiclient.ChatCompletionRequest{
			Model:    m.modelID,
			Messages: m.messages,
			Stream:   true,
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
		var content string
		if len(chunk.Choices) > 0 {
			content = chunk.Choices[0].Delta.Content
		}
		return streamMsg{chunk: content, stream: stream}
	}
}

func renderPixelArt(rows []string) string {
	pink := lipgloss.NewStyle().Foreground(lipgloss.Color("#E84393"))
	orange := lipgloss.NewStyle().Foreground(lipgloss.Color("#E67E22"))
	black := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000"))

	styles := map[rune]lipgloss.Style{
		' ': lipgloss.NewStyle(),
		'p': pink,
		'o': orange,
		'b': black,
	}

	chars := map[rune]string{
		' ': " ",
		'p': "█",
		'o': "█",
		'b': "█",
	}

	var renderedRows []string
	for _, row := range rows {
		var rendered string
		for _, c := range row {
			rendered += styles[c].Render(chars[c])
		}
		renderedRows = append(renderedRows, rendered)
	}
	return strings.Join(renderedRows, "\n")
}

func (m MainViewModel) headerView() string {
	flamingoRows := []string{
		"     pbp      ",
		"   bopppp     ",
		"   b   pp     ",
		"      pp      ",
		"     pp       ",
		"   ppp        ",
		"  ppppppp     ",
		"  pppppppp    ",
		"   pppppppp   ",
		"    ppppppp   ",
		"      oo      ",
		"      o o     ",
		"      o  o    ",
		"     oooo     ",
		"    o  o      ",
		"       o      ",
		"      oo      ",
	}

	flamingo := renderPixelArt(flamingoRows)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFF")).
		PaddingTop(2).
		Render("flamingode")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render(getVersion())

	info := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)

	return lipgloss.JoinHorizontal(lipgloss.Top, flamingo, "  ", info)
}

func (m MainViewModel) statusView() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	return style.Render(m.status)
}

func (m MainViewModel) View() tea.View {
	var c *tea.Cursor
	if !m.textInput.VirtualCursor() {
		c = m.textInput.Cursor()
		c.Y += lipgloss.Height(m.headerView()) + m.viewport.Height() + lipgloss.Height(m.statusView())
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

func resolveModel(cfg config.Config) (*apiclient.Client, string, string) {
	if cfg.DefaultModel == "" {
		return nil, "", "no model selected"
	}

	parts := strings.SplitN(cfg.DefaultModel, "/", 2)
	if len(parts) != 2 {
		return nil, "", "no model selected"
	}

	providerName, modelID := parts[0], parts[1]
	provider, ok := cfg.Providers[providerName]
	if !ok {
		return nil, "", "no model selected"
	}

	found := false
	for _, m := range provider.Models {
		if m.ID == modelID {
			found = true
			break
		}
	}
	if !found {
		return nil, "", "no model selected"
	}

	client := apiclient.NewWithBaseURL(provider.APIKey, provider.BaseURL)
	return client, modelID, cfg.DefaultModel
}

func initialMainViewModel(cfg config.Config) MainViewModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.CharLimit = 4096
	ti.SetWidth(50)

	client, modelID, status := resolveModel(cfg)

	messages := []apiclient.ChatCompletionMessage{
		apiclient.NewTextMessage("system", "You are a helpful agent"),
	}

	return MainViewModel{
		textInput: ti,
		client:    client,
		modelID:   modelID,
		status:    status,
		messages:  messages,
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalln("Failed to load config, error:", err)
	}

	p := tea.NewProgram(initialMainViewModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
