package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAtMentions_NoMentions(t *testing.T) {
	input := "hello world"
	result := resolveAtMentions(input, "/tmp", 1000)
	if result != input {
		t.Fatalf("expected %q, got %q", input, result)
	}
}

func TestResolveAtMentions_FileMention(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello file\n"), 0644)

	input := "Please review @test.txt"
	result := resolveAtMentions(input, tmpDir, 1000)

	if !strings.Contains(result, "<file path=\"test.txt\">") {
		t.Fatalf("expected file tag, got: %q", result)
	}
	if !strings.Contains(result, "hello file") {
		t.Fatalf("expected file content, got: %q", result)
	}
	if !strings.Contains(result, "</file>") {
		t.Fatalf("expected closing file tag, got: %q", result)
	}
}

func TestResolveAtMentions_DirectoryMention(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "subdir", "inner.txt"), []byte("x"), 0644)

	input := "Check @subdir"
	result := resolveAtMentions(input, tmpDir, 1000)

	if !strings.Contains(result, "<directory path=\"subdir\">") {
		t.Fatalf("expected directory tag, got: %q", result)
	}
	if !strings.Contains(result, "inner.txt") {
		t.Fatalf("expected directory content, got: %q", result)
	}
	if !strings.Contains(result, "</directory>") {
		t.Fatalf("expected closing directory tag, got: %q", result)
	}
}

func TestResolveAtMentions_MultipleMentions(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("alpha\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("beta\n"), 0644)

	input := "Compare @a.txt and @b.txt"
	result := resolveAtMentions(input, tmpDir, 1000)

	if !strings.Contains(result, "alpha") || !strings.Contains(result, "beta") {
		t.Fatalf("expected both file contents, got: %q", result)
	}
	if strings.Contains(result, "@a.txt") || strings.Contains(result, "@b.txt") {
		t.Fatalf("expected mentions to be replaced, got: %q", result)
	}
}

func TestResolveAtMentions_NonExistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	input := "Look at @missing.txt"
	result := resolveAtMentions(input, tmpDir, 1000)

	if !strings.Contains(result, "<error path=\"missing.txt\">") {
		t.Fatalf("expected error tag, got: %q", result)
	}
}

func TestResolveAtMentions_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	input := "Look at @../secret.txt"
	result := resolveAtMentions(input, tmpDir, 1000)

	if !strings.Contains(result, "<error path=\"../secret.txt\">") {
		t.Fatalf("expected error tag, got: %q", result)
	}
}

func TestResolveAtMentions_ContextExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	input := "Read @test.txt"
	result := resolveAtMentions(input, tmpDir, -1)

	if !strings.Contains(result, "<error path=\"test.txt\">") {
		t.Fatalf("expected error tag for exhausted context, got: %q", result)
	}
}

func TestResolveAtMentions_LeavesEmailAlone(t *testing.T) {
	input := "Contact me at user@example.com"
	result := resolveAtMentions(input, "/tmp", 1000)
	if result != input {
		t.Fatalf("expected input unchanged, got %q", result)
	}
}

func TestResolveAtMentions_LeavesBareAt(t *testing.T) {
	input := "Look @ this"
	result := resolveAtMentions(input, "/tmp", 1000)
	if result != input {
		t.Fatalf("expected input unchanged, got %q", result)
	}
}

func TestResolveAtMentions_TruncatesOversizedFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "huge.txt"), []byte(strings.Repeat("x", 200)), 0644)

	input := "Read @huge.txt"
	result := resolveAtMentions(input, tmpDir, 100)

	if !strings.Contains(result, "[File truncated:") {
		t.Fatalf("expected truncation notice, got: %q", result)
	}
}
