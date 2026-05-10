package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceText_ReplaceFirst(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world hello"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":"hello","new_text":"hi","replace_all":false}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "14" {
		t.Fatalf("expected bytes written '14', got %q", result)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "file.txt"))
	if string(content) != "hi world hello" {
		t.Fatalf("expected 'hi world hello', got %q", string(content))
	}
}

func TestReplaceText_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world hello"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":"hello","new_text":"hi","replace_all":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "11" {
		t.Fatalf("expected bytes written '11', got %q", result)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "file.txt"))
	if string(content) != "hi world hi" {
		t.Fatalf("expected 'hi world hi', got %q", string(content))
	}
}

func TestReplaceText_RemoveText(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":" world","new_text":"","replace_all":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "5" {
		t.Fatalf("expected bytes written '5', got %q", result)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "file.txt"))
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(content))
	}
}

func TestReplaceText_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":"xyz","new_text":"abc","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error when old_text not found, got nil")
	}
}

func TestReplaceText_EmptyOldText(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":"","new_text":"abc","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error when old_text is empty, got nil")
	}
}

func TestReplaceText_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("secret"), 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"../secret.txt","old_text":"secret","new_text":"x","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestReplaceText_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"/etc/passwd","old_text":"root","new_text":"x","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestReplaceText_RejectsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"subdir","old_text":"x","new_text":"y","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error when path is a directory, got nil")
	}
}

func TestReplaceText_RejectsBinary(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "bin.dat"), []byte{0x00, 0x01, 0x02, 0xff}, 0644)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"bin.dat","old_text":"x","new_text":"y","replace_all":false}`)
	if err == nil {
		t.Fatal("expected error for binary file, got nil")
	}
}

func TestReplaceText_PreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(path, []byte("hello world"), 0755)

	tool := &ReplaceText{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"file.txt","old_text":" world","new_text":"","replace_all":false}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("expected permissions 0755, got %04o", info.Mode().Perm())
	}
}
