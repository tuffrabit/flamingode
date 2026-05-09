package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Providers) != 0 {
		t.Fatalf("expected empty providers, got %d", len(cfg.Providers))
	}
	if cfg.Tools.ReadFile.MaxSize != 100000 {
		t.Fatalf("expected default read_file max_size of 100000, got %d", cfg.Tools.ReadFile.MaxSize)
	}

	path := filepath.Join(tmpDir, ".flamingode", "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected default config file to be created")
	}
}

func TestLoadReadsExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	path := filepath.Join(tmpDir, ".flamingode", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	want := Config{
		DefaultModel: "ollama/llama3.1:8b",
		Providers: map[string]Provider{
			"ollama": {
				BaseURL: "http://localhost:11434/v1",
				API:     "openai-completions",
				APIKey:  "ollama",
				Models: []Model{
					{ID: "llama3.1:8b"},
					{
						ID:            "qwen2.5-coder:7b",
						Name:          "Qwen 2.5 Coder 7B",
						Reasoning:     true,
						Input:         []string{"text"},
						ContextWindow: 128000,
						MaxTokens:     32000,
						Cost: &Cost{
							Input:      0,
							Output:     0,
							CacheRead:  0,
							CacheWrite: 0,
						},
					},
				},
			},
		},
	}

	data, _ := json.MarshalIndent(want, "", "  ")
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ollama, ok := cfg.Providers["ollama"]
	if !ok {
		t.Fatal("expected ollama provider")
	}
	if ollama.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("unexpected baseUrl: %s", ollama.BaseURL)
	}
	if len(ollama.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(ollama.Models))
	}
	if ollama.Models[0].ID != "llama3.1:8b" {
		t.Fatalf("unexpected model id: %s", ollama.Models[0].ID)
	}
	if ollama.Models[1].Cost == nil {
		t.Fatal("expected cost on second model")
	}
}
