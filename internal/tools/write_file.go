package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile writes contents to a file within the working directory.
type WriteFile struct {
	WorkingDir string
}

func (w *WriteFile) GetName() string {
	return "write_file"
}

func (w *WriteFile) GetDescription() string {
	return "Write contents to a file within the working directory. Overwrites existing files. Creates missing parent directories. Returns the number of bytes written."
}

func (w *WriteFile) GetPermissionRequired() bool {
	return true
}

func (w *WriteFile) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to the file within the working directory.",
			},
			"contents": map[string]interface{}{
				"type":        "string",
				"description": "The contents to write to the file.",
			},
		},
		"required": []string{"path", "contents"},
	}
}

func (w *WriteFile) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}

		if filepath.IsAbs(args.Path) {
			return "", fmt.Errorf("absolute paths are not allowed")
		}

		cleanWorkingDir := filepath.Clean(w.WorkingDir)
		resolvedPath, err := resolveWritePath(cleanWorkingDir, args.Path)
		if err != nil {
			return "", err
		}

		// Ensure resolved path is still within the working directory tree.
		if resolvedPath != cleanWorkingDir && !strings.HasPrefix(resolvedPath, cleanWorkingDir+string(filepath.Separator)) {
			return "", fmt.Errorf("access denied: path is outside the working directory")
		}

		// Create parent directories if they don't exist.
		parentDir := filepath.Dir(resolvedPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directories: %w", err)
		}

		// Write the file.
		data := []byte(args.Contents)
		if err := os.WriteFile(resolvedPath, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}

		return fmt.Sprintf("%d", len(data)), nil
	}
}

// resolveWritePath resolves a relative path within a working directory,
// following symlinks in existing path components. It rejects paths that
// escape the working directory via ".." or symlink traversal.
func resolveWritePath(cleanWorkingDir, relPath string) (string, error) {
	relPath = filepath.Clean(relPath)
	components := strings.Split(relPath, string(filepath.Separator))

	current := cleanWorkingDir
	for _, comp := range components {
		if comp == "" || comp == "." {
			continue
		}

		next := filepath.Join(current, comp)

		info, err := os.Lstat(next)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", fmt.Errorf("cannot resolve symlink: %w", err)
			}
			next = resolved
		}

		current = next
	}

	return filepath.Clean(current), nil
}
