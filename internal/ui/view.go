package ui

import (
	"encoding/json"
	"fmt"
	"sort"
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
	thinkingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	var b strings.Builder
	for _, msg := range m.messages {
		if msg.Role == "system" {
			continue
		}
		prefix := "You: "
		if msg.Role == "assistant" {
			if msg.ReasoningContent != "" {
				thinkingLine := "Thinking: " + msg.ReasoningContent
				b.WriteString(ansi.Wordwrap(thinkingStyle.Render(thinkingLine), wrapWidth, ""))
				b.WriteString("\n\n")
			}
			hasToolCalls := len(msg.ToolCalls) > 0
			if hasToolCalls {
				for _, tc := range msg.ToolCalls {
					toolLine := "Tool: " + tc.Function.Name
					if tc.Function.Arguments != "" {
						var args map[string]interface{}
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
							keys := make([]string, 0, len(args))
							for k := range args {
								keys = append(keys, k)
							}
							sort.Strings(keys)
							for _, k := range keys {
								toolLine += " " + k + ":" + fmt.Sprintf("%v", args[k])
							}
						}
					}
					b.WriteString(ansi.Wordwrap(toolLine, wrapWidth, ""))
					b.WriteString("\n\n")
				}
			}
			if msg.Content == "" && hasToolCalls {
				continue
			}
			prefix = "Assistant: "
		} else if msg.Role == "tool" {
			continue
		}
		line := prefix + msg.Content
		b.WriteString(ansi.Wordwrap(line, wrapWidth, ""))
		b.WriteString("\n\n")
	}
	if m.streaming || m.pending != "" || m.pendingThinking != "" {
		if m.pendingThinking != "" {
			thinkingLine := "Thinking: " + m.pendingThinking
			if m.streaming {
				thinkingLine += "█"
			}
			b.WriteString(ansi.Wordwrap(thinkingStyle.Render(thinkingLine), wrapWidth, ""))
			b.WriteString("\n\n")
		}
		if m.pending != "" || !m.streaming {
			line := "Assistant: " + m.pending
			if m.streaming && m.pending != "" {
				line += "█"
			}
			b.WriteString(ansi.Wordwrap(line, wrapWidth, ""))
		}
		if m.streaming {
			if m.pending != "" {
				b.WriteString("\n")
			}
			b.WriteString(m.spinner.View())
		}
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

	wdLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render(m.workingDir)

	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render(m.status)

	infoLines := []string{title, subtitle, wdLine, statusLine}
	if m.sessionUsage.TotalTokens > 0 {
		usageLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Render(fmt.Sprintf("Tokens: %d ↑ / %d ↓  (%d total)", m.sessionUsage.PromptTokens, m.sessionUsage.CompletionTokens, m.sessionUsage.TotalTokens))
		infoLines = append(infoLines, usageLine)
	}
	info := lipgloss.JoinVertical(lipgloss.Left, infoLines...)

	return lipgloss.JoinHorizontal(lipgloss.Top, flamingo, "  ", info)
}
