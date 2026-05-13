package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func historyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return filepath.Join(home, ".flamingode", "history"), nil
}

// Load reads all history entries from disk.
func Load() ([]string, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to open history file: %w", err)
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var s string
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue
		}
		entries = append(entries, s)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("unable to read history file: %w", err)
	}
	return entries, nil
}

// Append adds a new entry to the history file. If the number of entries
// exceeds maxLength, the oldest entries are dropped.
func Append(input string, maxLength int) error {
	if maxLength <= 0 {
		maxLength = 50
	}

	entries, err := Load()
	if err != nil {
		return err
	}

	entries = append(entries, input)

	if len(entries) > maxLength {
		entries = entries[len(entries)-maxLength:]
	}

	path, err := historyPath()
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("unable to create history temp file: %w", err)
	}

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("unable to marshal history entry: %w", err)
		}
		if _, err := fmt.Fprintln(f, string(data)); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("unable to write history entry: %w", err)
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("unable to close history temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("unable to finalize history file: %w", err)
	}

	return nil
}
