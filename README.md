# Flamingode

A terminal-based AI coding harness. Chat with LLMs, let them read and edit your code, run commands, and search your project — all from a keyboard-driven TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles).

---

## Features

- **OpenAI-compatible API support** — Works with OpenAI, Ollama, llama.cpp, LM Studio, or any other OpenAI-compatible endpoint.
- **Streaming chat TUI** — Real-time responses with a clean, scrollable interface.
- **Built-in coding tools** — The model can read files, write files, search with grep, glob for files, list directories, and run shell commands.
- **Context window management** — Automatically estimates tokens and truncates tool results to stay within model limits.
- **Session persistence** — Every conversation is saved to `~/.flamingode/sessions/` and can be resumed later.
- **Inline file mentions** — Type `@path/to/file` (or `@directory/`) in your message to automatically include file contents inline.
- **Permission gating** — Destructive or risky actions (like `exec_command`) prompt for approval before running.
- **Command history** — Press `↑` / `↓` to recall previous messages.
- **Debug logging** — Optional request/response logging to debug API issues.

---

## Installation

### Prerequisites

- [Go](https://go.dev/) 1.26 or later

### Build from source

```bash
# Clone the repository
git clone https://github.com/tuffrabit/flamingode.git
cd flamingode

# Build for current platform
go build -o flamingode .

# Or run directly
go run .
```

### Cross-compile for all platforms

```bash
./build.sh [version]
```

This produces binaries in `bin/` for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

If a version is provided (e.g., `./build.sh 1.0.0`), it is embedded into the binary and used in the output filename.

---

## Configuration

On first run, Flamingode creates `~/.flamingode/config.json` with sensible defaults. Edit this file to add your providers and models.

### Minimal configuration (Ollama)

```json
{
  "providers": {
    "ollama": {
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "models": [
        { "id": "llama3.1:8b" },
        { "id": "qwen2.5-coder:7b" }
      ]
    }
  },
  "defaultModel": "ollama/llama3.1:8b"
}
```

### Full configuration example

```json
{
  "providers": {
    "openai": {
      "baseUrl": "https://api.openai.com/v1",
      "api": "openai-completions",
      "apiKey": "sk-...",
      "models": [
        {
          "id": "gpt-4o",
          "name": "GPT-4o",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 16000,
          "cost": { "input": 0.0025, "output": 0.01, "cacheRead": 0.00125, "cacheWrite": 0 }
        }
      ]
    },
    "ollama": {
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "models": [
        { "id": "llama3.1:8b", "contextWindow": 128000 }
      ]
    }
  },
  "defaultModel": "openai/gpt-4o",
  "apiTimeoutSeconds": 600,
  "historyLength": 50,
  "tools": {
    "read_file": {
      "max_size": 100000
    },
    "exec_command": {
      "timeout_seconds": 60
    }
  }
}
```

### Configuration options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `providers` | object | `{}` | Map of provider names to endpoint configurations. |
| `defaultModel` | string | `""` | The model to use on startup, in the form `provider/model-id`. |
| `apiTimeoutSeconds` | integer | `600` | HTTP timeout for API requests. |
| `historyLength` | integer | `50` | Number of previous messages to keep in input history. |
| `tools.read_file.max_size` | integer | `100000` | Maximum file size in bytes for the `read_file` tool. |
| `tools.exec_command.timeout_seconds` | integer | `0` | Timeout for shell command execution (`0` = no timeout). |

---

## Usage

### Start a new chat

```bash
flamingode
```

### Resume a previous session

```bash
flamingode -resume <session-uuid>
```

Session files are stored in `~/.flamingode/sessions/`. The session UUID is displayed in the header while you chat.

### Enable debug logging

```bash
flamingode -d
```

Logs all API requests and responses to `debug.log` in the same directory as the executable.

---

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `↑` / `↓` | Navigate message history |
| `Ctrl+D` | Quit |

---

## Slash commands

Type these in the input box like a message:

| Command | Description |
|---------|-------------|
| `/clear` | Start a fresh session with the current model |

---

## File mentions

Type `@path/to/file` anywhere in your message to automatically inline the file's contents (or directory listing) before sending. This works relative to the current working directory.

Examples:
- `@main.go` — Include the contents of `main.go`
- `@internal/config/` — Include a directory listing

---

## Built-in tools

Flamingode exposes the following tools to the model:

| Tool | Description | Permission required |
|------|-------------|---------------------|
| `read_file` | Read the contents of a file | No |
| `write_file` | Write content to a file | No |
| `replace_text` | Replace text within a file | No |
| `list_directory` | List files in a directory | No |
| `grep` | Search for a pattern in files | No |
| `glob` | Find files matching a glob pattern | No |
| `exec_command` | Execute a shell command | **Yes** |

Commands that require permission will pause the conversation and prompt you to approve or deny before they run.

---

## License

MIT
