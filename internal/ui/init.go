package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/tuffrabit/flamingode/internal/apiclient"
	"github.com/tuffrabit/flamingode/internal/config"
	"github.com/tuffrabit/flamingode/internal/history"
	"github.com/tuffrabit/flamingode/internal/session"
	"github.com/tuffrabit/flamingode/internal/tools"
)

func ResolveModelByID(cfg config.Config, modelID string) (*apiclient.Client, string, int, string) {
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
	return ResolveModelByID(cfg, cfg.DefaultModel)
}

const defaultSystemPrompt = "You are a helpful agent"

func ResolveSystemPrompt(workingDir string) string {
	projectPath := filepath.Join(workingDir, ".flamingode", "system_prompt.md")
	if data, err := os.ReadFile(projectPath); err == nil {
		return string(data)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".flamingode", "system_prompt.md")
		if data, err := os.ReadFile(globalPath); err == nil {
			return string(data)
		}
	}

	return defaultSystemPrompt
}

func InitialMainViewModel(cfg config.Config, sess *session.Session, events []session.Event, yolo bool) MainViewModel {
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

	client, resolvedModelID, contextWindow, status := ResolveModelByID(cfg, modelID)

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
		systemPrompt := ResolveSystemPrompt(wd)
		messages = []apiclient.ChatCompletionMessage{
			apiclient.NewTextMessage("system", systemPrompt),
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
	r.Register(&tools.Glob{WorkingDir: wd})

	hist, _ := history.Load()

	return MainViewModel{
		textInput:     ti,
		client:        client,
		modelID:       resolvedModelID,
		fullModelID:   modelID,
		contextWindow: contextWindow,
		status:        status,
		workingDir:    wd,
		systemPrompt:  ResolveSystemPrompt(wd),
		yolo:          yolo,
		messages:      messages,
		spinner:       s,
		toolRegistry:  r,
		session:       sess,
		sessionID:     sessionID,
		sessionUsage:  sessionUsage,
		history:       hist,
		historyIndex:  -1,
		historyMaxLen: cfg.HistoryLength,
	}
}
