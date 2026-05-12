package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tuffrabit/flamingode/internal/apiclient"
)

// Dir returns the sessions directory path (~/.flamingode/sessions).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return filepath.Join(home, ".flamingode", "sessions"), nil
}

// EnsureDir creates the sessions directory if it does not exist.
func EnsureDir() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

// Header is stored on the first line of every session file.
// It holds the latest usage snapshot so it can be parsed quickly without
// scanning the entire file.
type Header struct {
	SessionID string          `json:"session_id"`
	ModelID   string          `json:"model_id"`
	CreatedAt time.Time       `json:"created_at"`
	Usage     apiclient.Usage `json:"usage"`
}

// Event represents a single chat event stored in a session file.
type Event struct {
	Type             string               `json:"type"` // "system", "user", "assistant", "tool", "error"
	Role             string               `json:"role,omitempty"`
	Content          string               `json:"content,omitempty"`
	ReasoningContent string               `json:"reasoning_content,omitempty"`
	Name             string               `json:"name,omitempty"`
	ToolCalls        []apiclient.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string               `json:"tool_call_id,omitempty"`
	Error            string               `json:"error,omitempty"`
}

// ToMessage converts an Event back to an apiclient.ChatCompletionMessage.
func (e Event) ToMessage() apiclient.ChatCompletionMessage {
	return apiclient.ChatCompletionMessage{
		Role:             e.Role,
		Content:          e.Content,
		ReasoningContent: e.ReasoningContent,
		Name:             e.Name,
		ToolCalls:        e.ToolCalls,
		ToolCallID:       e.ToolCallID,
	}
}

// EventFromMessage creates an Event from an apiclient.ChatCompletionMessage.
func EventFromMessage(msg apiclient.ChatCompletionMessage) Event {
	return Event{
		Type:             msg.Role,
		Role:             msg.Role,
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		Name:             msg.Name,
		ToolCalls:        msg.ToolCalls,
		ToolCallID:       msg.ToolCallID,
	}
}

// Session manages a single chat session file.
type Session struct {
	Header Header
	Path   string
}

// NewSession creates a new session file with the given model ID.
func NewSession(modelID string) (*Session, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	path := filepath.Join(dir, id)
	s := &Session{
		Header: Header{
			SessionID: id,
			ModelID:   modelID,
			CreatedAt: time.Now(),
		},
		Path: path,
	}
	if err := s.writeHeader(); err != nil {
		return nil, err
	}
	return s, nil
}

// LoadSession reads a session file and returns its header plus all events.
func LoadSession(sessionID string) (*Session, []Event, error) {
	dir, err := Dir()
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, sessionID)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open session file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("empty session file")
	}

	var header Header
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		return nil, nil, fmt.Errorf("invalid session header: %w", err)
	}

	s := &Session{
		Header: header,
		Path:   path,
	}

	var events []Event
	for scanner.Scan() {
		var evt Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			// Skip malformed lines.
			continue
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading session file: %w", err)
	}

	return s, events, nil
}

func (s *Session) writeHeader() error {
	data, err := json.Marshal(s.Header)
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, append(data, '\n'), 0644)
}

// UpdateHeader rewrites the first line of the session file with the current
// header while preserving all existing events.
func (s *Session) UpdateHeader() error {
	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return fmt.Errorf("empty session file")
	}

	var rest [][]byte
	for scanner.Scan() {
		rest = append(rest, append([]byte(nil), scanner.Bytes()...))
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	headerData, err := json.Marshal(s.Header)
	if err != nil {
		return err
	}

	tmpPath := s.Path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	write := func(b []byte) error {
		_, err := out.Write(b)
		return err
	}

	if err := write(headerData); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := write([]byte{'\n'}); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	for _, line := range rest {
		if err := write(line); err != nil {
			out.Close()
			os.Remove(tmpPath)
			return err
		}
		if err := write([]byte{'\n'}); err != nil {
			out.Close()
			os.Remove(tmpPath)
			return err
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, s.Path)
}

// AppendEvent appends an event as a new line to the session file.
func (s *Session) AppendEvent(evt Event) error {
	f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}
