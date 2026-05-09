# config

json config file should be read (or auto generated with sane defaults) on app run/start. json config file location should be ~/.flamingode/config.json

## providers

the providers section is a list of configured custom models. this is primarily to support local inference (ollama, llama.cpp, lm studio, etc)

### minimal example

```
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
  }
}
```

### full example

```
{
  "providers": {
    "ollama": {
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "models": [
        {
          "id": "llama3.1:8b",
          "name": "Llama 3.1 8B (Local)",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 32000,
          "cost": { "input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0 }
        }
      ]
    }
  },
  "tools": {
    "read_file": {
      "max_size": 100000
    }
  }
}
```

## tools

tool-specific configuration lives under the `tools` key.

### read_file

| key | type | default | description |
|-----|------|---------|-------------|
| `max_size` | integer | `100000` | Maximum file size in bytes that read_file will allow. Defaults to 100000 bytes (~25k tokens). |
