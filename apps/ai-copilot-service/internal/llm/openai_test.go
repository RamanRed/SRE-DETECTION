package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestOpenAIClientComplete(t *testing.T) {
	requestSeen := make(chan struct{}, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/openai/v1/chat/completions" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Errorf("method = %q", request.Method)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.Model != "llama-test" || body.Temperature != 0.2 || len(body.Messages) != 1 || body.Messages[0].Content != "diagnose me" {
			t.Errorf("unexpected request body: %+v", body)
		}
		requestSeen <- struct{}{}
		return response(http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"  root cause text  "}}]}`), nil
	})}

	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey:      "secret",
		BaseURL:     "https://provider.example/openai",
		Model:       "llama-test",
		Temperature: 0.2,
		HTTPClient:  httpClient,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	content, err := client.Complete(context.Background(), "diagnose me")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if content != "root cause text" {
		t.Fatalf("content = %q", content)
	}
	select {
	case <-requestSeen:
	default:
		t.Fatal("provider did not receive request")
	}
}

func TestOpenAIClientProviderErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantErrSub string
	}{
		{name: "HTTP error", status: http.StatusUnauthorized, body: `{"error":{"message":"bad key"}}`, wantErrSub: "HTTP 401: bad key"},
		{name: "malformed JSON", status: http.StatusOK, body: `{`, wantErrSub: "decode"},
		{name: "no choices", status: http.StatusOK, body: `{}`, wantErrSub: "no choices"},
		{name: "empty content", status: http.StatusOK, body: `{"choices":[{"message":{"content":" "}}]}`, wantErrSub: "empty content"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewOpenAIClient(OpenAIConfig{
				APIKey: "key", BaseURL: "https://provider.example", Model: "model",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return response(test.status, test.body), nil
				})},
			})
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			if _, err := client.Complete(context.Background(), "prompt"); err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErrSub)
			}
		})
	}
}

func TestOpenAIClientHonorsTimeout(t *testing.T) {
	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey: "key", BaseURL: "https://provider.example", Model: "model",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Millisecond,
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				select {
				case <-time.After(100 * time.Millisecond):
					return response(http.StatusOK, `{"choices":[{"message":{"content":"late"}}]}`), nil
				case <-request.Context().Done():
					return nil, request.Context().Err()
				}
			}),
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.Complete(context.Background(), "prompt"); err == nil {
		t.Fatal("complete succeeded, want timeout")
	}
}

func TestChatCompletionsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://api.openai.com":                   "https://api.openai.com/v1/chat/completions",
		"https://api.groq.com/openai/":             "https://api.groq.com/openai/v1/chat/completions",
		"https://example.test/v1":                  "https://example.test/v1/chat/completions",
		"https://example.test/v1/chat/completions": "https://example.test/v1/chat/completions",
	}
	for input, expected := range tests {
		actual, err := chatCompletionsEndpoint(input)
		if err != nil {
			t.Errorf("endpoint %q: %v", input, err)
			continue
		}
		if actual != expected {
			t.Errorf("endpoint %q = %q, want %q", input, actual, expected)
		}
	}
	if _, err := chatCompletionsEndpoint("localhost:8080"); err == nil {
		t.Fatal("relative endpoint succeeded")
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
