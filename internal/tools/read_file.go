package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// DefaultMaxFileSize is roughly 25k tokens (approximated at ~4 bytes per token).
const DefaultMaxFileSize = 100_000

// ReadFile reads the contents of a text file within the working directory.
type ReadFile struct {
	WorkingDir string
	MaxSize    int64
}

func (r *ReadFile) GetName() string {
	return "read_file"
}

func (r *ReadFile) GetDescription() string {
	maxSize := r.MaxSize
	if maxSize <= 0 {
		maxSize = DefaultMaxFileSize
	}
	return fmt.Sprintf("Read the contents of a text file within the working directory. Returns the file content as a string. Files larger than %d bytes are truncated. Rejects binary files. Supports reading a specific line range via line_offset (1-indexed) and limit.", maxSize)
}

func (r *ReadFile) GetPermissionRequired() bool {
	return false
}

func (r *ReadFile) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to the file within the working directory.",
			},
			"line_offset": map[string]interface{}{
				"type":        "integer",
				"description": "Line number to start reading from (1-indexed). Defaults to 1.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of lines to read. If omitted, reads all remaining lines from line_offset.",
			},
		},
		"required": []string{"path"},
	}
}

// ReadFileContent reads a text file within the working directory and returns its
// contents, a bool indicating whether the read was truncated, and any error.
func ReadFileContent(workingDir, relPath string, maxSize int64) (string, bool, error) {
	cleanPath, err := ResolveWorkingDirPath(workingDir, relPath)
	if err != nil {
		return "", false, err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("path is a directory, not a file")
	}

	if maxSize <= 0 {
		maxSize = DefaultMaxFileSize
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	wasTruncated := info.Size() > maxSize
	b, err := io.ReadAll(io.LimitReader(file, maxSize))
	if err != nil {
		return "", false, fmt.Errorf("failed to read file: %w", err)
	}

	if wasTruncated {
		b = fixTruncatedUTF8(b)
	}

	if !utf8.ValidString(string(b)) {
		return "", false, fmt.Errorf("file appears to be binary or contains invalid UTF-8")
	}

	return string(b), wasTruncated, nil
}

func (r *ReadFile) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Path       string `json:"path"`
			LineOffset int    `json:"line_offset"`
			Limit      int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}

		content, wasTruncated, err := ReadFileContent(r.WorkingDir, args.Path, r.MaxSize)
		if err != nil {
			return "", err
		}

		if args.LineOffset < 1 {
			args.LineOffset = 1
		}

		scanner := bufio.NewScanner(strings.NewReader(content))
		maxSize := r.MaxSize
		if maxSize <= 0 {
			maxSize = DefaultMaxFileSize
		}
		scanner.Buffer(make([]byte, 1024), int(maxSize)+1024)

		currentLine := 1
		var result strings.Builder

		for scanner.Scan() {
			if currentLine >= args.LineOffset {
				result.WriteString(scanner.Text())
				result.WriteByte('\n')
				if args.Limit > 0 && currentLine >= args.LineOffset+args.Limit-1 {
					break
				}
			}
			currentLine++
		}

		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to scan file: %w", err)
		}

		if wasTruncated {
			result.WriteString(fmt.Sprintf("[File truncated: exceeded max size of %d bytes]\n", maxSize))
		}

		return result.String(), nil
	}
}

// fixTruncatedUTF8 trims incomplete UTF-8 sequences from the end of a byte slice.
func fixTruncatedUTF8(b []byte) []byte {
	if utf8.Valid(b) {
		return b
	}
	for i := 1; i <= 3 && i < len(b); i++ {
		if utf8.Valid(b[:len(b)-i]) {
			return b[:len(b)-i]
		}
	}
	return b
}
