package deepseek

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
)

type fakeEmbeddingHTTPDoer func(req *http.Request) (*http.Response, error)

func (f fakeEmbeddingHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestEmbedMapsDeepSeekEmbeddingRequestAndResponse(t *testing.T) {
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.deepseek.com/v1/embedding" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["text"] != "hello" || body["input"] != nil || body["model"] != nil {
			t.Fatalf("unexpected DeepSeek embedding request: %#v", body)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": []string{"embed-request-1"}},
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[0.1,0.2,0.3]}`)),
		}, nil
	}))

	result, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		CredentialValue: "secret-key",
		ModelCode:       "deepseek-embedding",
		ProviderModelID: "deepseek-embedding",
	}, embedding.EmbedRequest{
		Texts: []string{"hello"},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if result.ProviderRequestID != "embed-request-1" || result.Dimensions != 3 || len(result.Vectors) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := result.Vectors[0].Values[1]; got != 0.2 {
		t.Fatalf("unexpected vector value: %v", got)
	}
}

func TestEmbedDeepSeekEmbeddingSendsOneRequestPerText(t *testing.T) {
	var requests []string
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		var body map[string]string
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, body["text"])

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[1,2]}`)),
		}, nil
	}))

	result, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		CredentialValue: "secret-key",
		ProviderModelID: "deepseek-embedding",
	}, embedding.EmbedRequest{
		Texts: []string{"first", "second"},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if len(requests) != 2 || requests[0] != "first" || requests[1] != "second" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	if len(result.Vectors) != 2 || result.Dimensions != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEmbedMapsOpenAICompatibleRequestAndResponse(t *testing.T) {
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.deepseek.com/embeddings" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}

		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["model"] != "deepseek-embedding" || body["text"] != nil {
			t.Fatalf("unexpected OpenAI-compatible request: %#v", body)
		}
		input, ok := body["input"].([]interface{})
		if !ok || len(input) != 2 || input[0] != "a" || input[1] != "b" {
			t.Fatalf("unexpected input: %#v", body["input"])
		}
		if body["dimensions"].(float64) != 64 {
			t.Fatalf("expected dimensions=64, got %#v", body)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": []string{"openai-embed-1"}},
			Body: io.NopCloser(strings.NewReader(`{
				"data":[
					{"index":0,"embedding":[0.1,0.2]},
					{"index":1,"embedding":[0.3,0.4]}
				],
				"usage":{"prompt_tokens":3,"total_tokens":5}
			}`)),
		}, nil
	}))

	result, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		CredentialValue: "secret-key",
		ProviderModelID: "deepseek-embedding",
		ProviderSettings: map[string]interface{}{
			"api_style":     "openai_compatible",
			"endpoint_path": "/v1/embedding",
		},
	}, embedding.EmbedRequest{
		Texts:  []string{"a", "b"},
		Params: map[string]interface{}{"dimensions": float64(64)},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if result.ProviderRequestID != "openai-embed-1" || result.Usage.TotalTokens != 5 || len(result.Vectors) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEmbedMapsProviderError(t *testing.T) {
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"X-Request-Id": []string{"auth-failed-1"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
		}, nil
	}))

	_, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		CredentialValue: "secret-key",
		ProviderModelID: "deepseek-embedding",
	}, embedding.EmbedRequest{
		Texts: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	providerErr, ok := providererror.From(err)
	if !ok {
		t.Fatalf("expected provider error, got %T %v", err, err)
	}
	if providerErr.Category != providererror.CategoryAuth || providerErr.Retryable || providerErr.ProviderRequestID != "auth-failed-1" {
		t.Fatalf("unexpected provider error: %#v", providerErr)
	}
}
