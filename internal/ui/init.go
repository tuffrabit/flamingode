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
	"github.com/tuffrabit/flamingode/internal/session"
	"github.com/tuffrabit/flamingode/internal/tools"
)

func resolveModelByID(cfg config.Config, modelID string) (*apiclient.Client, string, int, string) {
	if modelID == "" {
		return nil, "", 0, "no model selected"
	}

	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) != 2 {
		return nil, "", 0, "no model selected"
	}

	providerName, id := parts[0], parts[1]
	provider, ok := cfg.Providers[providerName]
	if !ok {
		return nil, "", 0, "no model selected"
	}

	var contextWindow int
	found := false
	for _, m := range provider.Models {
		if m.ID == id {
			found = true
			contextWindow = m.ContextWindow
			break
		}
	}
	if !found {
		return nil, "", 0, "no model selected"
	}

	client := apiclient.NewWithBaseURL(provider.APIKey, provider.BaseURL)
	client.SetTimeout(time.Duration(cfg.APITimeoutSeconds) * time.Second)
	if cfg.Debug {
		_ = client.SetDebug(true, cfg.DebugLogPath)
	}
	return client, id, contextWindow, modelID
}

func resolveModel(cfg config.Config) (*apiclient.Client, string, int, string) {
	return resolveModelByID(cfg, cfg.DefaultModel)
}

func InitialMainViewModel(cfg config.Config, sess *session.Session, events []session.Event) MainViewModel {
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

	modelID := cfg.DefaultModel
	if sess != nil && sess.Header.ModelID != "" {
		modelID = sess.Header.ModelID
	}

	client, resolvedModelID, contextWindow, status := resolveModelByID(cfg, modelID)

	wd, _ := os.Getwd()

	var messages []apiclient.ChatCompletionMessage
	var sessionUsage apiclient.Usage
	var sessionID string

	if sess != nil {
		sessionID = sess.Header.SessionID
		sessionUsage = sess.Header.Usage
	}

	if len(events) > 0 {
		for _, evt := range events {
			messages = append(messages, evt.ToMessage())
		}
	} else {
		messages = []apiclient.ChatCompletionMessage{
			apiclient.NewTextMessage("system", "You are a helpful agent"),
		}
		if sess != nil {
			_ = sess.AppendEvent(session.EventFromMessage(messages[0]))
		}
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
	r.Register(&tools.Grep{WorkingDir: wd})

	return MainViewModel{
		textInput:     ti,
		client:        client,
		modelID:       resolvedModelID,
		contextWindow: contextWindow,
		status:        status,
		workingDir:    wd,
		messages:      messages,
		spinner:       s,
		toolRegistry:  r,
		session:       sess,
		sessionID:     sessionID,
		sessionUsage:  sessionUsage,
	}
}
