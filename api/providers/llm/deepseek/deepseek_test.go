package deepseek

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
)

type fakeHTTPDoer func(req *http.Request) (*http.Response, error)

func (f fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGenerateMapsOpenAICompatibleRequestAndResponse(t *testing.T) {
	adapter := NewAdapter(fakeHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.deepseek.com/chat/completions" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["model"] != "deepseek-provider-model" {
			t.Fatalf("unexpected model in request: %#v", body)
		}
		format, ok := body["response_format"].(map[string]interface{})
		if !ok || format["type"] != "json_object" {
			t.Fatalf("expected json response format, got %#v", body["response_format"])
		}
		if body["temperature"].(float64) != 0.2 {
			t.Fatalf("expected default params in request, got %#v", body)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"id":"deepseek-request-1",
				"choices":[{"message":{"role":"assistant","content":"{\"key_points\":\"ok\"}"}}],
				"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
			}`)),
		}, nil
	}))

	result, err := adapter.Generate(t.Context(), llm.ProviderConfig{
		BaseURL:           "https://api.deepseek.com",
		APIKey:            "secret-key",
		ProviderModelID:   "deepseek-provider-model",
		DefaultParamsJSON: `{"temperature":0.2}`,
	}, llm.GenerateRequest{
		Operation:      llm.OperationTextSummary,
		ResponseFormat: llm.ResponseFormatJSON,
		Messages: []llm.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if result.ProviderRequestID != "deepseek-request-1" || result.Content != `{"key_points":"ok"}` || result.Usage.TotalTokens != 6 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGenerateMapsProviderError(t *testing.T) {
	adapter := NewAdapter(fakeHTTPDoer(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"X-Request-ID": []string{"rate-limit-1"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"limited"}`)),
		}, nil
	}))

	_, err := adapter.Generate(t.Context(), llm.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		APIKey:          "secret-key",
		ProviderModelID: "deepseek-provider-model",
	}, llm.GenerateRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	providerErr, ok := providererror.From(err)
	if !ok {
		t.Fatalf("expected provider error, got %T %v", err, err)
	}
	if providerErr.Category != providererror.CategoryRateLimit || !providerErr.Retryable || providerErr.ProviderRequestID != "rate-limit-1" {
		t.Fatalf("unexpected provider error: %#v", providerErr)
	}
}
