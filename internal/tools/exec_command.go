package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// DefaultExecTimeout is the default maximum duration for a command.
const DefaultExecTimeout = 60 * time.Second

// MaxExecOutputSize is the maximum combined stdout/stderr bytes to return.
const MaxExecOutputSize = 100_000

// ExecCommand runs a command with arguments in the working directory.
type ExecCommand struct {
	WorkingDir string
	Timeout    time.Duration
}

func (e *ExecCommand) GetName() string {
	return "exec_command"
}

func (e *ExecCommand) GetDescription() string {
	return "Run a command with arguments in the working directory. Returns stdout, stderr, and exit status. Commands are executed directly (no shell), so pipes, redirections, and variable expansion are not supported."
}

func (e *ExecCommand) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The command to run (e.g., 'grep', 'cat', 'go').",
			},
			"args": map[string]interface{}{
				"type":        "array",
				"description": "Arguments to pass to the command.",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"command"},
	}
}

func (e *ExecCommand) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Command == "" {
			return "", fmt.Errorf("command is required")
		}

		timeout := e.Timeout
		if timeout <= 0 {
			timeout = DefaultExecTimeout
		}

		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(execCtx, args.Command, args.Args...)
		cmd.Dir = e.WorkingDir

		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		runErr := cmd.Run()

		stdout, stderr := truncateOutputs(stdoutBuf.Bytes(), stderrBuf.Bytes(), MaxExecOutputSize)

		var result bytes.Buffer
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				result.WriteString(fmt.Sprintf("exit code: %d\n", exitErr.ExitCode()))
			}
			if execCtx.Err() == context.DeadlineExceeded {
				result.WriteString("error: command timed out\n")
			} else {
				result.WriteString(fmt.Sprintf("error: %v\n", runErr))
			}
		}

		if len(stdout) > 0 {
			result.WriteString(fmt.Sprintf("stdout:\n%s\n", string(stdout)))
		}
		if len(stderr) > 0 {
			result.WriteString(fmt.Sprintf("stderr:\n%s\n", string(stderr)))
		}

		return result.String(), nil
	}
}

func truncateOutputs(stdout, stderr []byte, maxTotal int) ([]byte, []byte) {
	total := len(stdout) + len(stderr)
	if total <= maxTotal {
		return stdout, stderr
	}

	stdoutMax := maxTotal
	stderrMax := maxTotal
	if len(stdout) > 0 && len(stderr) > 0 {
		stdoutMax = maxTotal / 2
		stderrMax = maxTotal / 2
	}

	if len(stdout) < stdoutMax {
		stderrMax = maxTotal - len(stdout)
	} else if len(stderr) < stderrMax {
		stdoutMax = maxTotal - len(stderr)
	}

	return truncateBytes(stdout, stdoutMax), truncateBytes(stderr, stderrMax)
}

func truncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	if maxLen <= 3 {
		return b[:maxLen]
	}
	truncated := b[:maxLen-3]
	truncated = fixTruncatedUTF8(truncated)
	return append(truncated, '.', '.', '.')
}
