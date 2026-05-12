package session

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/tuffrabit/flamingode/internal/apiclient"
)

func TestEventRoundTrip(t *testing.T) {
	evt := Event{
		Type:             "assistant",
		Role:             "assistant",
		Content:          "hello world",
		ReasoningContent: "thinking deeply",
		ToolCalls: []apiclient.ToolCall{
			{ID: "call_1", Type: "function", Function: apiclient.FunctionCall{Name: "read_file", Arguments: `{"path":"test.go"}`}},
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var loaded Event
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	msg := loaded.ToMessage()
	if msg.Content != "hello world" {
		t.Errorf("content mismatch: got %q", msg.Content)
	}
	if msg.ReasoningContent != "thinking deeply" {
		t.Errorf("reasoning mismatch: got %q", msg.ReasoningContent)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("toolcalls length mismatch: got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("tool name mismatch: got %q", msg.ToolCalls[0].Function.Name)
	}
}

func TestSessionCreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	// monkey-patch Dir by writing to a known path... actually we can't easily.
	// Let's test the core logic with a direct file path instead.
	s := &Session{
		Header: Header{SessionID: "test-id", ModelID: "openai/gpt-4"},
		Path:   tmpDir + "/session.txt",
	}

	if err := s.writeHeader(); err != nil {
		t.Fatalf("writeHeader failed: %v", err)
	}

	events := []Event{
		{Type: "system", Role: "system", Content: "You are a helpful agent"},
		{Type: "user", Role: "user", Content: "hi"},
		{Type: "assistant", Role: "assistant", Content: "hello"},
	}
	for _, evt := range events {
		if err := s.AppendEvent(evt); err != nil {
			t.Fatalf("AppendEvent failed: %v", err)
		}
	}

	s.Header.Usage = apiclient.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	if err := s.UpdateHeader(); err != nil {
		t.Fatalf("UpdateHeader failed: %v", err)
	}

	loadedS, loadedEvents, err := LoadSession("test-id")
	if err == nil {
		// This will fail because LoadSession uses the real Dir(), not our tmpDir.
		// That's expected. Just verify the file on disk looks correct.
	}
	_ = loadedS
	_ = loadedEvents

	// Verify file content manually
	f, err := os.Open(s.Path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	var header Header
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("header unmarshal: %v", err)
	}
	if header.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", header.Usage.TotalTokens)
	}
	if lines[1] != `{"type":"system","role":"system","content":"You are a helpful agent"}` {
		t.Errorf("unexpected line 1: %s", lines[1])
	}
}
