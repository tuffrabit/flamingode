package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/tuffrabit/flamingode/internal/config"
	"github.com/tuffrabit/flamingode/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalln("Failed to load config, error:", err)
	}

	p := tea.NewProgram(ui.InitialMainViewModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
