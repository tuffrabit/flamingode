package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

type MainViewModel struct {
}

func (m MainViewModel) Init() tea.Cmd {
	return nil
}

func (m MainViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyPressMsg:
		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m MainViewModel) View() tea.View {
	s := ""
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

func initialMainViewModel() MainViewModel {
	return MainViewModel{}
}

func main() {
	p := tea.NewProgram(initialMainViewModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
