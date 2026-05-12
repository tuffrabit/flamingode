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

func TestGrep_SkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "node_modules", "pkg.js"), []byte("findme\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "src.js"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "node_modules") {
		t.Fatalf("expected node_modules to be skipped, got: %q", result)
	}
	if !strings.Contains(result, "src.js") {
		t.Fatalf("expected src.js match, got: %q", result)
	}
}

func TestGrep_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	_ = os.WriteFile(realFile, []byte("findme\n"), 0644)
	_ = os.Symlink(realFile, filepath.Join(tmpDir, "link.txt"))

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Symlinks are skipped, so only real.txt should match.
	lines := strings.Count(result, "findme")
	if lines != 1 {
		t.Fatalf("expected 1 match, got %d: %q", lines, result)
	}
}

func TestGrep_SkipsHugeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file slightly larger than MaxGrepFileSize.
	huge := make([]byte, MaxGrepFileSize+1)
	copy(huge, []byte("findme\n"))
	_ = os.WriteFile(filepath.Join(tmpDir, "huge.log"), huge, 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"huge.log"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No matches found." {
		t.Fatalf("expected no matches for huge file, got: %q", result)
	}
}

func TestGrep_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("findme\n"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tool := &Grep{WorkingDir: tmpDir}
	_, err := tool.GetAction()(ctx, `{"query":"findme","path":"."}`)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestGrep_RespectsGitignore_File(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("secret.txt\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("findme\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "public.txt"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "secret.txt") {
		t.Fatalf("expected secret.txt to be skipped, got: %q", result)
	}
	if !strings.Contains(result, "public.txt") {
		t.Fatalf("expected public.txt match, got: %q", result)
	}
}

func TestGrep_RespectsGitignore_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("build/\n"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "build"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "build", "out.js"), []byte("findme\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "src.js"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "build") {
		t.Fatalf("expected build dir to be skipped, got: %q", result)
	}
	if !strings.Contains(result, "src.js") {
		t.Fatalf("expected src.js match, got: %q", result)
	}
}

func TestGrep_RespectsGitignore_Wildcard(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.log\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "debug.log"), []byte("findme\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "app.txt"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "debug.log") {
		t.Fatalf("expected debug.log to be skipped, got: %q", result)
	}
	if !strings.Contains(result, "app.txt") {
		t.Fatalf("expected app.txt match, got: %q", result)
	}
}

func TestGrep_RespectsGitignore_Anchored(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("/root.txt\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("findme\n"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "root.txt"), []byte("findme\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"findme","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(result, "\n")
	var hasRoot, hasSubRoot bool
	for _, line := range lines {
		if strings.HasPrefix(line, "root.txt:") {
			hasRoot = true
		}
		if strings.HasPrefix(line, "sub/root.txt:") {
			hasSubRoot = true
		}
	}
	if hasRoot {
		t.Fatalf("expected root.txt at root to be skipped, got: %q", result)
	}
	if !hasSubRoot {
		t.Fatalf("expected sub/root.txt match, got: %q", result)
	}
}

func TestGrep_CaseInsensitiveLiteral(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("Hello World\nhello again\nHELLO\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"hello","path":"data.txt","case_insensitive":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Hello World") {
		t.Fatalf("expected match for Hello World, got: %q", result)
	}
	if !strings.Contains(result, "hello again") {
		t.Fatalf("expected match for hello again, got: %q", result)
	}
	if !strings.Contains(result, "HELLO") {
		t.Fatalf("expected match for HELLO, got: %q", result)
	}
}

func TestGrep_CaseInsensitiveRegex(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("Hello World\nhello again\nHELLO\n"), 0644)

	tool := &Grep{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"query":"^hello","path":"data.txt","regex":true,"case_insensitive":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Hello World") {
		t.Fatalf("expected match for Hello World, got: %q", result)
	}
	if !strings.Contains(result, "hello again") {
		t.Fatalf("expected match for hello again, got: %q", result)
	}
	if !strings.Contains(result, "HELLO") {
		t.Fatalf("expected match for HELLO, got: %q", result)
	}
}
