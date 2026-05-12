package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecCommand_Success(t *testing.T) {
	args := `{"command":"go","args":["version"]}`
	e := &ExecCommand{
		WorkingDir: t.TempDir(),
		Timeout:    5 * time.Second,
	}

	result, err := e.GetAction()(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "go version") {
		t.Fatalf("expected 'go version' in output, got: %s", result)
	}
}

func TestExecCommand_ErrorAndStderr(t *testing.T) {
	args := `{"command":"go","args":["notacommand"]}`
	e := &ExecCommand{
		WorkingDir: t.TempDir(),
		Timeout:    5 * time.Second,
	}

	result, err := e.GetAction()(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "error:") {
		t.Fatalf("expected error in output, got: %s", result)
	}
}

func TestExecCommand_Timeout(t *testing.T) {
	var cmd string
	var sleepArgs []string
	if runtime.GOOS == "windows" {
		cmd = "timeout"
		sleepArgs = []string{"/t", "10"}
	} else {
		cmd = "sleep"
		sleepArgs = []string{"10"}
	}

	argsMap := map[string]interface{}{
		"command": cmd,
		"args":    sleepArgs,
	}
	data, _ := json.Marshal(argsMap)

	e := &ExecCommand{
		WorkingDir: t.TempDir(),
		Timeout:    100 * time.Millisecond,
	}

	start := time.Now()
	result, err := e.GetAction()(context.Background(), string(data))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "timed out") {
		t.Fatalf("expected timeout message, got: %s", result)
	}

	if elapsed > 2*time.Second {
		t.Fatalf("expected quick timeout, took %v", elapsed)
	}
}

func TestExecCommand_EmptyCommand(t *testing.T) {
	args := `{"command":""}`
	e := &ExecCommand{
		WorkingDir: t.TempDir(),
		Timeout:    5 * time.Second,
	}

	_, err := e.GetAction()(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecCommand_Truncation(t *testing.T) {
	// Write a tiny program that prints a lot to stdout
	// We'll just test the truncate helper directly since cross-platform
	// compilation is heavy for a simple truncation test.
	stdout := make([]byte, MaxExecOutputSize+100)
	stderr := make([]byte, 50)
	for i := range stdout {
		stdout[i] = 'a'
	}
	for i := range stderr {
		stderr[i] = 'b'
	}

	out, err := truncateOutputs(stdout, stderr, MaxExecOutputSize)
	if len(out)+len(err) > MaxExecOutputSize+6 { // +6 for "..." on each
		t.Fatalf("expected truncated outputs to fit within limit, got %d+%d", len(out), len(err))
	}

	if !strings.HasSuffix(string(out), "...") {
		t.Fatal("expected stdout to be truncated with ellipsis")
	}
}
