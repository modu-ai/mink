package ollama

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// httpClient handles HTTP communication with Ollama server.
// SPEC-GOOSE-LLM-001 §6.3: No external dependencies, uses stdlib net/http.
type httpClient struct {
	baseURL    string
	httpClient *http.Client
}

// validateBaseURL validates that the baseURL is a localhost address.
// Ollama runs locally, so we reject non-localhost URLs to prevent SSRF attacks.
func validateBaseURL(baseURL string) error {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid baseURL: %w", err)
	}

	// Only allow http or https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	// Only allow localhost addresses
	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("empty host in baseURL")
	}

	// Check if host is localhost or 127.0.0.1
	isLocalhost := isLocalhost(host)
	if !isLocalhost {
		return fmt.Errorf("baseURL must be localhost (got %s)", host)
	}

	return nil
}

// isLocalhost checks if a hostname is a localhost address.
func isLocalhost(host string) bool {
	// Check for common localhost variants
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}

	// Check if it's an IPv4 loopback address (127.x.x.x)
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}

	return false
}

func newHTTPClient(baseURL string) *httpClient {
	// Validate baseURL to prevent SSRF attacks
	if err := validateBaseURL(baseURL); err != nil {
		panic(fmt.Sprintf("invalid baseURL: %v", err))
	}

	return &httpClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			// 30-second timeout for all requests
			Timeout: 30 * time.Second,
			// Explicit TLS configuration (using defaults)
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// Use system defaults (safe for local connections)
					MinVersion: tls.VersionTLS12,
				},
				// Disable HTTP/2 for local connections (simpler debugging)
				ForceAttemptHTTP2: false,
			},
		},
	}
}

// chatRequest is the Ollama /api/chat request format.
type chatRequest struct {
	Model    string         `json:"model"`
	Messages []message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  requestOptions `json:"options,omitempty"`
}

// message is the Ollama message format.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// requestOptions contains optional generation parameters.
type requestOptions struct {
	Temperature float64  `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// chatResponse is the non-streaming Ollama /api/chat response format.
type chatResponse struct {
	Message         responseMessage `json:"message"`
	Done            bool            `json:"done"`
	PromptEvalCount int             `json:"prompt_eval_count"`
	EvalCount       int             `json:"eval_count"`
}

// responseMessage is the Ollama message format in responses.
type responseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatStreamLine is a single line in the streaming Ollama /api/chat response.
type chatStreamLine struct {
	Message         responseMessage `json:"message"`
	Done            bool            `json:"done"`
	PromptEvalCount int             `json:"prompt_eval_count,omitempty"`
	EvalCount       int             `json:"eval_count,omitempty"`
}

// showResponse is the Ollama /api/show response format.
type showResponse struct {
	License   string       `json:"license"`
	Modelfile string       `json:"modelfile"`
	Details   modelDetails `json:"details"`
}

// modelDetails contains model metadata.
type modelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
	ContextLength     int      `json:"context_length"`
}

// PostChat performs a non-streaming chat completion request.
func (c *httpClient) PostChat(ctx context.Context, req chatRequest) (*chatResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &httpError{Err: err, StatusCode: 0}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &httpError{
			Err:        fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			StatusCode: resp.StatusCode,
			Body:       bodyBytes,
		}
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// chatStream represents a streaming chat response.
type chatStream struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
}

// PostChatStream performs a streaming chat completion request.
func (c *httpClient) PostChatStream(ctx context.Context, req chatRequest) (*chatStream, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &httpError{Err: err, StatusCode: 0}
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &httpError{
			Err:        fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			StatusCode: resp.StatusCode,
			Body:       bodyBytes,
		}
	}

	return &chatStream{
		scanner: bufio.NewScanner(resp.Body),
		body:    resp.Body,
	}, nil
}

// Next reads the next NDJSON line from the stream.
func (s *chatStream) Next() (string, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return s.scanner.Text(), nil
}

// Close closes the stream.
func (s *chatStream) Close() error {
	return s.body.Close()
}

// GetShow fetches model information from /api/show.
func (c *httpClient) GetShow(ctx context.Context, model string) (*showResponse, error) {
	reqBody := map[string]string{"model": model}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/api/show"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &httpError{Err: err, StatusCode: 0}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &httpError{
			Err:        fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			StatusCode: resp.StatusCode,
			Body:       bodyBytes,
		}
	}

	var result showResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// httpError wraps HTTP errors with status code and body.
type httpError struct {
	Err        error
	StatusCode int
	Body       []byte
}

func (e *httpError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("HTTP error %d", e.StatusCode)
}

func (e *httpError) Unwrap() error {
	return e.Err
}

// parseChatStreamLine parses a single NDJSON line from the streaming response.
func parseChatStreamLine(line string) (*chatStreamLine, error) {
	var result chatStreamLine
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, fmt.Errorf("parse NDJSON line: %w (line: %q)", err, line)
	}
	return &result, nil
}
