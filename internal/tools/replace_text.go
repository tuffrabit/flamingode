package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ReplaceText replaces occurrences of a text snippet in a file within the working directory.
type ReplaceText struct {
	WorkingDir string
}

func (r *ReplaceText) GetName() string {
	return "replace_text"
}

func (r *ReplaceText) GetDescription() string {
	return "Replace text in a file within the working directory. Returns the total number of bytes in the rewritten file. Rejects binary files. Replaces all occurrences if replace_all is true, otherwise replaces only the first occurrence."
}

func (r *ReplaceText) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to the file within the working directory.",
			},
			"old_text": map[string]interface{}{
				"type":        "string",
				"description": "The text to search for and replace. Must not be empty.",
			},
			"new_text": map[string]interface{}{
				"type":        "string",
				"description": "The text to replace old_text with. May be empty to remove text.",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, replace all occurrences of old_text. If false, replace only the first occurrence.",
			},
		},
		"required": []string{"path", "old_text", "new_text", "replace_all"},
	}
}

func (r *ReplaceText) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Path       string `json:"path"`
			OldText    string `json:"old_text"`
			NewText    string `json:"new_text"`
			ReplaceAll bool   `json:"replace_all"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}

		if args.OldText == "" {
			return "", fmt.Errorf("old_text must not be empty")
		}

		// Resolve to an absolute path within the working directory.
		absPath, err := filepath.Abs(filepath.Join(r.WorkingDir, args.Path))
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		// Resolve any symlinks in the path.
		resolvedPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("cannot access path: %w", err)
		}

		cleanPath := filepath.Clean(resolvedPath)
		cleanWorkingDir := filepath.Clean(r.WorkingDir)

		// Restrict access to the working directory tree.
		if cleanPath != cleanWorkingDir && !strings.HasPrefix(cleanPath, cleanWorkingDir+string(filepath.Separator)) {
			return "", fmt.Errorf("access denied: path is outside the working directory")
		}

		// Verify the path is a file, not a directory.
		info, err := os.Stat(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to stat file: %w", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("path is a directory, not a file")
		}

		// Read existing contents.
		b, err := os.ReadFile(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}

		// Reject binary or non-UTF-8 files.
		if !utf8.ValidString(string(b)) {
			return "", fmt.Errorf("file appears to be binary or contains invalid UTF-8")
		}

		content := string(b)
		if !strings.Contains(content, args.OldText) {
			return "", fmt.Errorf("old_text not found in file")
		}

		n := 1
		if args.ReplaceAll {
			n = -1
		}
		replaced := strings.Replace(content, args.OldText, args.NewText, n)

		data := []byte(replaced)
		if err := os.WriteFile(cleanPath, data, info.Mode()); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}

		return fmt.Sprintf("%d", len(data)), nil
	}
}
