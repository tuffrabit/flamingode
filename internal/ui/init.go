package ui

import (
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/config"
	"github.com/tuffrabit/flamingode/internal/tools"
)

func resolveModel(cfg config.Config) (*apiclient.Client, string, int, string) {
	if cfg.DefaultModel == "" {
		return nil, "", 0, "no model selected"
	}

	parts := strings.SplitN(cfg.DefaultModel, "/", 2)
	if len(parts) != 2 {
		return nil, "", 0, "no model selected"
	}

	providerName, modelID := parts[0], parts[1]
	provider, ok := cfg.Providers[providerName]
	if !ok {
		return nil, "", 0, "no model selected"
	}

	var contextWindow int
	found := false
	for _, m := range provider.Models {
		if m.ID == modelID {
			found = true
			contextWindow = m.ContextWindow
			break
		}
	}
	if !found {
		return nil, "", 0, "no model selected"
	}

	client := apiclient.NewWithBaseURL(provider.APIKey, provider.BaseURL)
	if cfg.Debug {
		_ = client.SetDebug(true, cfg.DebugLogPath)
	}
	return client, modelID, contextWindow, cfg.DefaultModel
}

func InitialMainViewModel(cfg config.Config) MainViewModel {
	ti := textarea.New()
	ti.Placeholder = "Type a message..."
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.CharLimit = 4096
	ti.ShowLineNumbers = false
	ti.Prompt = "> "
	ti.DynamicHeight = true
	ti.MaxHeight = 10
	ti.KeyMap.InsertNewline.SetEnabled(false)
	ti.SetWidth(50)

	client, modelID, contextWindow, status := resolveModel(cfg)

	wd, _ := os.Getwd()

	messages := []apiclient.ChatCompletionMessage{
		apiclient.NewTextMessage("system", "You are a helpful agent"),
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

	r := tools.NewRegistry()
	r.Register(&tools.ListDirectory{WorkingDir: wd})
	r.Register(&tools.ReadFile{WorkingDir: wd, MaxSize: cfg.Tools.ReadFile.MaxSize})
	r.Register(&tools.WriteFile{WorkingDir: wd})
	r.Register(&tools.ReplaceText{WorkingDir: wd})
	r.Register(&tools.ExecCommand{WorkingDir: wd, Timeout: time.Duration(cfg.Tools.ExecCommand.TimeoutSeconds) * time.Second})

	return MainViewModel{
		textInput:     ti,
		client:        client,
		modelID:       modelID,
		contextWindow: contextWindow,
		status:        status,
		workingDir:    wd,
		messages:      messages,
		spinner:       s,
		toolRegistry:  r,
	}
}
