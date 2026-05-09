package ui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/tuffrabit/flamingode/internal/version"
)

func (m MainViewModel) renderChat() string {
	wrapWidth := m.viewport.Width()
	if wrapWidth <= 0 {
		wrapWidth = 80
	}
	var b strings.Builder
	for _, msg := range m.messages {
		prefix := "You: "
		if msg.Role == "assistant" {
			prefix = "Assistant: "
		} else if msg.Role == "system" {
			continue
		}
		line := prefix + msg.Content
		b.WriteString(ansi.Wordwrap(line, wrapWidth, ""))
		b.WriteString("\n\n")
	}
	if m.streaming || m.pending != "" {
		line := "Assistant: " + m.pending
		if m.streaming {
			line += "█"
		}
		b.WriteString(ansi.Wordwrap(line, wrapWidth, ""))
	}
	return b.String()
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
		Render(version.Get())

	info := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)

	return lipgloss.JoinHorizontal(lipgloss.Top, flamingo, "  ", info)
}

func (m MainViewModel) statusView() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	return style.Render(m.status)
}
