package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

// Provider is the small abstraction the triage engine needs from an LLM.
type Provider interface {
	Complete(context.Context, string) (string, error)
}

// OpenAIClient calls OpenAI-compatible chat completion APIs, including Groq.
type OpenAIClient struct {
	apiKey      string
	endpoint    string
	model       string
	temperature float64
	httpClient  *http.Client
}

type OpenAIConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	Timeout     time.Duration
	HTTPClient  *http.Client
}

func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("OpenAI API key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("OpenAI model is required")
	}
	endpoint, err := chatCompletionsEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	client := cfg.HTTPClient
	if client == nil {
		if cfg.Timeout <= 0 {
			return nil, errors.New("OpenAI timeout must be greater than zero")
		}
		client = &http.Client{Timeout: cfg.Timeout}
	}

	return &OpenAIClient{
		apiKey:      strings.TrimSpace(cfg.APIKey),
		endpoint:    endpoint,
		model:       strings.TrimSpace(cfg.Model),
		temperature: cfg.Temperature,
		httpClient:  client,
	}, nil
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (client *OpenAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	payload, err := json.Marshal(chatRequest{
		Model: client.model,
		Messages: []chatMessage{{
			Role:    "user",
			Content: prompt,
		}},
		Temperature: client.temperature,
		Stream:      false,
	})
	if err != nil {
		return "", fmt.Errorf("encode chat completion request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create chat completion request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call chat completion provider: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return "", fmt.Errorf("chat completion response exceeds %d bytes", maxResponseBytes)
	}

	var decoded chatResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return "", fmt.Errorf("chat completion provider returned HTTP %d", response.StatusCode)
		}
		return "", fmt.Errorf("decode chat completion response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := ""
		if decoded.Error != nil {
			message = strings.TrimSpace(decoded.Error.Message)
		}
		if message == "" {
			return "", fmt.Errorf("chat completion provider returned HTTP %d", response.StatusCode)
		}
		return "", fmt.Errorf("chat completion provider returned HTTP %d: %s", response.StatusCode, message)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("chat completion provider returned no choices")
	}

	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("chat completion provider returned empty content")
	}
	return content, nil
}

func chatCompletionsEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse OpenAI base URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("OpenAI base URL must be an absolute http or https URL")
	}

	switch {
	case strings.HasSuffix(parsed.Path, "/chat/completions"):
		return parsed.String(), nil
	case strings.HasSuffix(parsed.Path, "/v1"):
		parsed.Path += "/chat/completions"
	default:
		parsed.Path += "/v1/chat/completions"
	}
	return parsed.String(), nil
}
