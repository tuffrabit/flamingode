package ui

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/tuffrabit/flamingode/internal/tools"
)

// resolveAtMentions scans input for whitespace-delimited tokens starting with '@'.
// For each token, it attempts to resolve the remainder as a file or directory path
// relative to workingDir. Files are read with the same limits as the read_file tool.
// Directories are listed with the same logic as the list_directory tool.
// The result is returned inline wrapped in XML-style tags.
//
// If maxFileSize is negative, every @ mention is replaced with an inline error
// indicating the context window is exhausted.
func resolveAtMentions(input, workingDir string, maxFileSize int64) string {
	if !strings.Contains(input, "@") {
		return input
	}

	var result strings.Builder
	i := 0
	for i < len(input) {
		// Preserve whitespace
		for i < len(input) && unicode.IsSpace(rune(input[i])) {
			result.WriteByte(input[i])
			i++
		}
		if i >= len(input) {
			break
		}

		// Read token
		start := i
		for i < len(input) && !unicode.IsSpace(rune(input[i])) {
			i++
		}
		token := input[start:i]

		if strings.HasPrefix(token, "@") && len(token) > 1 {
			path := token[1:]
			if maxFileSize < 0 {
				result.WriteString(fmt.Sprintf("<error path=\"%s\">context window exhausted, unable to include file</error>", path))
			} else {
				replacement, err := resolveMention(path, workingDir, maxFileSize)
				if err != nil {
					result.WriteString(fmt.Sprintf("<error path=\"%s\">%s</error>", path, err))
				} else {
					result.WriteString(replacement)
				}
			}
		} else {
			result.WriteString(token)
		}
	}

	return result.String()
}

func resolveMention(path, workingDir string, maxFileSize int64) (string, error) {
	cleanPath, err := tools.ResolveWorkingDirPath(workingDir, path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if info.IsDir() {
		content, err := tools.ListDirectoryContent(workingDir, path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("<directory path=\"%s\">\n%s\n</directory>", path, content), nil
	}

	content, truncated, err := tools.ReadFileContent(workingDir, path, maxFileSize)
	if err != nil {
		return "", err
	}
	if truncated {
		content += fmt.Sprintf("\n[File truncated: exceeded max size of %d bytes]\n", maxFileSize)
	}
	return fmt.Sprintf("<file path=\"%s\">\n%s\n</file>", path, content), nil
}
