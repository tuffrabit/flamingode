package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/config"
)

var Version string

func getVersion() string {
	if Version != "" {
		return Version
	}

	return "dev"
}

type MainViewModel struct {
	ready     bool
	viewport  viewport.Model
	textInput textinput.Model
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
		}

	case tea.WindowSizeMsg:
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
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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

func initialMainViewModel() MainViewModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(50)

	return MainViewModel{
		textInput: ti,
	}
}

func main() {
	_, err := config.Load()
	if err != nil {
		log.Fatalln("Failed to load config, error:", err)
	}

	p := tea.NewProgram(initialMainViewModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
