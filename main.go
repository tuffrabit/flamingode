package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/tuffrabit/flamingode/internal/config"
	"github.com/tuffrabit/flamingode/internal/session"
	"github.com/tuffrabit/flamingode/internal/ui"
)

func main() {
	debug := flag.Bool("d", false, "enable debug logging for LLM API requests and responses")
	resume := flag.String("resume", "", "resume an existing session by UUID")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalln("Failed to load config, error:", err)
	}

	if *debug {
		exe, err := os.Executable()
		if err != nil {
			log.Println("Unable to determine executable path, debug logging disabled:", err)
		} else {
			cfg.Debug = true
			cfg.DebugLogPath = filepath.Join(filepath.Dir(exe), "debug.log")
		}
	}

	if err := session.EnsureDir(); err != nil {
		log.Fatalln("Failed to create sessions directory, error:", err)
	}

	var sess *session.Session
	var events []session.Event

	if *resume != "" {
		sess, events, err = session.LoadSession(*resume)
		if err != nil {
			log.Fatalf("Failed to load session %s, error: %v\n", *resume, err)
		}
	} else {
		sess, err = session.NewSession(cfg.DefaultModel)
		if err != nil {
			log.Fatalln("Failed to create session, error:", err)
		}
	}

	p := tea.NewProgram(ui.InitialMainViewModel(cfg, sess, events))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
