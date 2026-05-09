package ui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/config"
)

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

	client, modelID, status := resolveModel(cfg)

	messages := []apiclient.ChatCompletionMessage{
		apiclient.NewTextMessage("system", "You are a helpful agent"),
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

	return MainViewModel{
		textInput: ti,
		client:    client,
		modelID:   modelID,
		status:    status,
		messages:  messages,
		spinner:   s,
	}
}
