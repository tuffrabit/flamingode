package apiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tuffrabit/flamingode/internal/version"
)

// Client is an HTTP client for the OpenAI Chat Completions API.
type Client struct {
	baseURL      string
	apiKey       string
	httpClient   *http.Client
	debug        bool
	debugLog     *log.Logger
	debugLogFile *os.File
}

// New creates a new OpenAI API client with the provided API key.
// It defaults to the official OpenAI API endpoint.
func New(apiKey string) *Client {
	return NewWithBaseURL(apiKey, "https://api.openai.com/v1")
}

// NewWithBaseURL creates a new OpenAI API client with a custom base URL.
// The baseURL should include the API version path (e.g. https://api.openai.com/v1).
func NewWithBaseURL(apiKey, baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

// SetHTTPClient replaces the default HTTP client.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// SetTimeout sets the overall request timeout on the HTTP client.
// This caps the total time for a request, including reading the response body.
// It also updates the transport's ResponseHeaderTimeout so the server has the
// full timeout window to begin sending headers.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = timeout
	}
}

// SetDebug enables or disables debug logging. When enabled, request and
// response bodies are appended to the file at logPath.
func (c *Client) SetDebug(enabled bool, logPath string) error {
	c.debug = enabled
	if c.debugLogFile != nil {
		c.debugLogFile.Close()
		c.debugLogFile = nil
		c.debugLog = nil
	}
	if enabled && logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		c.debugLogFile = f
		c.debugLog = log.New(f, "", 0)
	}
	return nil
}

func (c *Client) logDebug(kind, url string, body []byte) {
	if !c.debug || c.debugLog == nil {
		return
	}
	timestamp := time.Now().Format(time.RFC3339)
	c.debugLog.Printf("%s %s %s %s", timestamp, kind, url, body)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flamingode/"+version.Get())
	return req, nil
}

func (c *Client) doRequest(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if c.debug {
		c.logDebug("response", req.URL.String(), body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return c.decodeError(resp)
	}

	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func (c *Client) decodeError(resp *http.Response) error {
	var payload struct {
		Error *APIError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("http error %d", resp.StatusCode)
	}
	if payload.Error != nil {
		payload.Error.StatusCode = resp.StatusCode
		return payload.Error
	}
	return fmt.Errorf("http error %d", resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// ChatCompletionRequest is the body for POST /chat/completions.
type ChatCompletionRequest struct {
	Model         string                  `json:"model"`
	Messages      []ChatCompletionMessage `json:"messages"`
	Temperature   *float64                `json:"temperature,omitempty"`
	MaxTokens     *int                    `json:"max_tokens,omitempty"`
	Stream        bool                    `json:"stream,omitempty"`
	StreamOptions *StreamOptions          `json:"stream_options,omitempty"`
	TopP          *float64                `json:"top_p,omitempty"`
	N             *int                    `json:"n,omitempty"`
	Stop          interface{}             `json:"stop,omitempty"` // string or []string
	PresencePenalty  *float64             `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64             `json:"frequency_penalty,omitempty"`
	User             string               `json:"user,omitempty"`
	Tools            []Tool               `json:"tools,omitempty"`
	ToolChoice       interface{}          `json:"tool_choice,omitempty"` // "none", "auto", or ToolChoiceObject
	ResponseFormat   interface{}          `json:"response_format,omitempty"`
	Seed             *int                 `json:"seed,omitempty"`
}

// StreamOptions controls streaming behaviour.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// Tool describes a callable tool for the model.
type Tool struct {
	Type     string               `json:"type"`
	Function FunctionDefinition   `json:"function"`
}

// FunctionDefinition describes a function tool schema.
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

// ChatCompletionMessage is a message in the request conversation.
type ChatCompletionMessage struct {
	Role             string        `json:"role"`
	Content          string        `json:"-"`
	ContentParts     []ContentPart `json:"-"`
	ReasoningContent string        `json:"-"`
	Name             string        `json:"name,omitempty"`
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

// MarshalJSON implements custom marshalling so Content can be either a string
// or an array of content parts.
func (m ChatCompletionMessage) MarshalJSON() ([]byte, error) {
	type msg ChatCompletionMessage
	raw := struct {
		Content interface{} `json:"content"`
		msg
	}{
		msg: msg(m),
	}
	if len(m.ContentParts) > 0 {
		raw.Content = m.ContentParts
	} else {
		raw.Content = m.Content
	}
	return json.Marshal(raw)
}

// ContentPart represents a single part of a multi-part message.
type ContentPart struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
	Refusal    string      `json:"refusal,omitempty"`
	File       *FilePart   `json:"file,omitempty"`
}

// ImageURL holds an image reference for vision models.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// InputAudio holds audio data for audio inputs.
type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// FilePart holds a file reference for chat completions.
type FilePart struct {
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// ToolCall describes a tool invocation generated by the model.
type ToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function,omitempty"`
}

// FunctionCall describes a single function invocation.
type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// ChatCompletionResponse is the non-streaming response from POST /chat/completions.
type ChatCompletionResponse struct {
	ID                string                     `json:"id"`
	Object            string                     `json:"object"`
	Created           int64                      `json:"created"`
	Model             string                     `json:"model"`
	Choices           []Choice                   `json:"choices"`
	Usage             Usage                      `json:"usage"`
	SystemFingerprint string                     `json:"system_fingerprint,omitempty"`
	ServiceTier       string                     `json:"service_tier,omitempty"`
}

// Choice is a single completion choice.
type Choice struct {
	Index        int                         `json:"index"`
	Message      ChatCompletionMessageResponse `json:"message"`
	FinishReason string                      `json:"finish_reason"`
	Logprobs     *Logprobs                   `json:"logprobs,omitempty"`
}

// ChatCompletionMessageResponse is a message returned by the model.
type ChatCompletionMessageResponse struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	Refusal     string       `json:"refusal,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
}

// Annotation is a model-generated annotation (e.g. URL citation).
type Annotation struct {
	Type        string      `json:"type"`
	URLCitation URLCitation `json:"url_citation"`
}

// URLCitation represents a URL citation annotation.
type URLCitation struct {
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

// Usage reports token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Logprobs holds log probability information.
type Logprobs struct {
	Content []LogprobContent `json:"content,omitempty"`
	Refusal string           `json:"refusal,omitempty"`
}

// LogprobContent holds log probability data for a single token.
type LogprobContent struct {
	Token       string       `json:"token"`
	Bytes       []int        `json:"bytes,omitempty"`
	Logprob     float64      `json:"logprob"`
	TopLogprobs []TopLogprob `json:"top_logprobs,omitempty"`
}

// TopLogprob holds a candidate token and its log probability.
type TopLogprob struct {
	Token   string  `json:"token"`
	Bytes   []int   `json:"bytes,omitempty"`
	Logprob float64 `json:"logprob"`
}

// ChatCompletionChunk is a single SSE chunk from a streaming response.
type ChatCompletionChunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	Choices           []ChunkChoice `json:"choices"`
	Usage             *Usage        `json:"usage,omitempty"`
	SystemFingerprint string        `json:"system_fingerprint,omitempty"`
}

// ChunkChoice is a choice inside a streamed chunk.
type ChunkChoice struct {
	Index        int       `json:"index"`
	Delta        Delta     `json:"delta"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Logprobs     *Logprobs `json:"logprobs,omitempty"`
}

// Delta is the incremental message content in a stream chunk.
type Delta struct {
	Role             string     `json:"role,omitempty"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Refusal          string     `json:"refusal,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// ChatCompletionDeleted is the response when deleting a completion.
type ChatCompletionDeleted struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Object  string `json:"object"`
}

// ChatCompletionList is the response from listing completions.
type ChatCompletionList struct {
	Data    []ChatCompletionResponse `json:"data"`
	HasMore bool                     `json:"has_more"`
}

// ChatCompletionMessagesResponse holds messages for a given completion ID.
type ChatCompletionMessagesResponse struct {
	Data []ChatCompletionMessageResponse `json:"data"`
}

// APIError represents an error returned by the OpenAI API.
type APIError struct {
	Message    string `json:"message"`
	Type       string `json:"type"`
	Param      string `json:"param,omitempty"`
	Code       string `json:"code,omitempty"`
	StatusCode int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openai api error %d (%s): %s", e.StatusCode, e.Type, e.Message)
}

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

// NewTextMessage creates a simple text message.
func NewTextMessage(role, text string) ChatCompletionMessage {
	return ChatCompletionMessage{Role: role, Content: text}
}

// NewContentPartsMessage creates a message from content parts.
func NewContentPartsMessage(role string, parts ...ContentPart) ChatCompletionMessage {
	return ChatCompletionMessage{Role: role, ContentParts: parts}
}

// TextPart returns a text content part.
func TextPart(text string) ContentPart {
	return ContentPart{Type: "text", Text: text}
}

// ImageURLPart returns an image_url content part.
func ImageURLPart(url, detail string) ContentPart {
	return ContentPart{Type: "image_url", ImageURL: &ImageURL{URL: url, Detail: detail}}
}

// InputAudioPart returns an input_audio content part.
func InputAudioPart(data, format string) ContentPart {
	return ContentPart{Type: "input_audio", InputAudio: &InputAudio{Data: data, Format: format}}
}

// FilePartContent returns a file content part.
func FilePartContent(fileID string) ContentPart {
	return ContentPart{Type: "file", File: &FilePart{FileID: fileID}}
}

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

// CreateChatCompletion sends a non-streaming chat completion request.
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.logDebug("request", httpReq.URL.String(), body)

	var resp ChatCompletionResponse
	if err := c.doRequest(httpReq, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ChatCompletionStream reads server-sent events from a streaming completion request.
type ChatCompletionStream struct {
	reader  io.ReadCloser
	scanner *bufio.Scanner
}

// Recv returns the next ChatCompletionChunk from the stream.
// When the stream is finished it returns io.EOF.
func (s *ChatCompletionStream) Recv() (ChatCompletionChunk, error) {
	var chunk ChatCompletionChunk
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return chunk, io.EOF
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return chunk, err
		}
		return chunk, nil
	}
	if err := s.scanner.Err(); err != nil {
		return chunk, err
	}
	return chunk, io.EOF
}

// Close closes the underlying response body.
func (s *ChatCompletionStream) Close() error {
	if s.reader != nil {
		return s.reader.Close()
	}
	return nil
}

// CreateChatCompletionStream initiates a streaming chat completion.
func (c *Client) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionStream, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	c.logDebug("request", httpReq.URL.String(), body)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, c.decodeError(resp)
	}

	return &ChatCompletionStream{
		reader:  resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// ListChatCompletions retrieves a list of chat completions.
func (c *Client) ListChatCompletions(ctx context.Context, limit int, order string) (*ChatCompletionList, error) {
	u, err := url.Parse(c.baseURL + "/chat/completions")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if order != "" {
		q.Set("order", order)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flamingode/"+version.Get())
	c.logDebug("request", req.URL.String(), nil)

	var list ChatCompletionList
	if err := c.doRequest(req, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// GetChatCompletion retrieves a specific chat completion by ID.
func (c *Client) GetChatCompletion(ctx context.Context, completionID string) (*ChatCompletionResponse, error) {
	path := "/chat/completions/" + url.PathEscape(completionID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	c.logDebug("request", req.URL.String(), nil)

	var resp ChatCompletionResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateChatCompletion updates the status of a chat completion.
func (c *Client) UpdateChatCompletion(ctx context.Context, completionID, status string) (*ChatCompletionResponse, error) {
	path := "/chat/completions/" + url.PathEscape(completionID)
	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.logDebug("request", req.URL.String(), body)

	var resp ChatCompletionResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteChatCompletion deletes a chat completion by ID.
func (c *Client) DeleteChatCompletion(ctx context.Context, completionID string) (*ChatCompletionDeleted, error) {
	path := "/chat/completions/" + url.PathEscape(completionID)
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	c.logDebug("request", req.URL.String(), nil)

	var resp ChatCompletionDeleted
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetChatCompletionMessages retrieves messages for a specific chat completion.
func (c *Client) GetChatCompletionMessages(ctx context.Context, completionID string) (*ChatCompletionMessagesResponse, error) {
	path := "/chat/completions/" + url.PathEscape(completionID) + "/messages"
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	c.logDebug("request", req.URL.String(), nil)

	var resp ChatCompletionMessagesResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
