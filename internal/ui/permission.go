package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
)

type permissionItem string

func (i permissionItem) FilterValue() string { return "" }

type permissionDelegate struct{}

func (d permissionDelegate) Height() int                             { return 1 }
func (d permissionDelegate) Spacing() int                            { return 0 }
func (d permissionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d permissionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(permissionItem)
	if !ok {
		return
	}

	str := string(i)
	selected := index == m.Index()

	var style lipgloss.Style
	if selected {
		style = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
		str = "> " + str
	} else {
		style = lipgloss.NewStyle().PaddingLeft(4)
	}
	fmt.Fprint(w, style.Render(str))
}

// PermissionPrompt is a bubbletea component that asks the user to approve or
// deny a tool call.
type PermissionPrompt struct {
	list     list.Model
	toolCall apiclient.ToolCall
	approved bool
	done     bool
}

func formatArguments(args string) string {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(args), &obj); err != nil {
		return args
	}
	pretty, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return args
	}
	return string(pretty)
}

// NewPermissionPrompt creates a new permission prompt for the given tool call.
func NewPermissionPrompt(toolCall apiclient.ToolCall) *PermissionPrompt {
	items := []list.Item{
		permissionItem("Yes"),
		permissionItem("No"),
	}

	l := list.New(items, permissionDelegate{}, 40, 4)
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	p := &PermissionPrompt{
		list:     l,
		toolCall: toolCall,
	}
	return p
}

// Update handles bubbletea messages for the permission prompt.
func (p *PermissionPrompt) Update(msg tea.Msg) (*PermissionPrompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if i, ok := p.list.SelectedItem().(permissionItem); ok {
				p.approved = string(i) == "Yes"
				p.done = true
			}
			return p, nil
		case "y":
			p.approved = true
			p.done = true
			return p, nil
		case "n":
			p.approved = false
			p.done = true
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

// View renders the permission prompt.
func (p *PermissionPrompt) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).MarginLeft(2)
	b.WriteString(titleStyle.Render("Permission required"))
	b.WriteString("\n")

	toolStyle := lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#E67E22"))
	b.WriteString(toolStyle.Render(p.toolCall.Function.Name))
	b.WriteString("\n\n")

	argsStr := formatArguments(p.toolCall.Function.Arguments)
	argsStyle := lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#888"))
	b.WriteString(argsStyle.Render(argsStr))
	b.WriteString("\n\n")
	b.WriteString(p.list.View())

	return b.String()
}
