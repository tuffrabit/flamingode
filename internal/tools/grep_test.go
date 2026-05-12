package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrep_LiteralInFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world\nfoo bar\nhello again\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"hello","path":"test.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "test.txt:1 hello world") {
		t.Fatalf("expected match line 1, got: %q", result)
	}
	if !strings.Contains(result, "test.txt:3 hello again") {
		t.Fatalf("expected match line 3, got: %q", result)
	}
	if strings.Contains(result, "foo bar") {
		t.Fatalf("unexpected match for foo bar, got: %q", result)
	}
}

func TestGrep_RegexInFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "data.go"), []byte("func main() {}\nfunc helper() {}\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"^func ","path":"data.go","regex":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "data.go:1 func main() {}") {
		t.Fatalf("expected match line 1, got: %q", result)
	}
	if !strings.Contains(result, "data.go:2 func helper() {}") {
		t.Fatalf("expected match line 2, got: %q", result)
	}
}

func TestGrep_RecursiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("alpha\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "b.txt"), []byte("beta\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"a","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a.txt:1 alpha") {
		t.Fatalf("expected a.txt match, got: %q", result)
	}
	if !strings.Contains(result, "sub/b.txt:1 beta") {
		t.Fatalf("expected sub/b.txt match, got: %q", result)
	}
}

func TestGrep_NoLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "x.txt"), []byte("match\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"match","path":"x.txt","line_numbers":false}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "x.txt match") {
		t.Fatalf("expected match without line number, got: %q", result)
	}
	if strings.Contains(result, "x.txt:1") {
		t.Fatalf("unexpected line number, got: %q", result)
	}
}

func TestGrep_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "empty.txt"), []byte("nothing here\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"xyz","path":"empty.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No matches found." {
		t.Fatalf("expected no matches message, got: %q", result)
	}
}

func TestGrep_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &Grep{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"query":"x","path":"../"}`)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestGrep_SkipsBinary(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "binary.bin"), []byte{0x00, 0x01, 0x02, 0xFF}, 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"\\x00","path":"binary.bin","regex":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No matches found." {
		t.Fatalf("expected no matches for binary file, got: %q", result)
	}
}

func TestGrep_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"query":"[bad","path":"a.txt","regex":true}`)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestGrep_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &Grep{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"query":"","path":"."}`)
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

func TestGrep_DefaultsToWorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "root.txt:1 findme") {
		t.Fatalf("expected match, got: %q", result)
	}
}
