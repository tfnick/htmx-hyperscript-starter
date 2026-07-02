package siliconflow

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

func TestEmbedMapsSiliconFlowEmbeddingRequestAndResponse(t *testing.T) {
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.siliconflow.cn/v1/embeddings" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["model"] != "Qwen/Qwen3-Embedding-0.6B" {
			t.Fatalf("unexpected model: %#v", body)
		}
		input, ok := body["input"].([]interface{})
		if !ok || len(input) != 2 || input[0] != "a" || input[1] != "b" {
			t.Fatalf("unexpected input: %#v", body["input"])
		}
		if body["encoding_format"] != "float" || body["dimensions"].(float64) != 64 {
			t.Fatalf("unexpected request params: %#v", body)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": []string{"sf-embed-1"}},
			Body: io.NopCloser(strings.NewReader(`{
				"data":[
					{"index":0,"embedding":[0.1,0.2]},
					{"index":1,"embedding":[0.3,0.4]}
				],
				"model":"Qwen/Qwen3-Embedding-0.6B",
				"usage":{"prompt_tokens":3,"total_tokens":5}
			}`)),
		}, nil
	}))

	result, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.siliconflow.cn",
		CredentialValue: "secret-key",
		ModelCode:       "qwen3-embedding-0.6b",
		ProviderModelID: "Qwen/Qwen3-Embedding-0.6B",
		ProviderSettings: map[string]interface{}{
			"endpoint_path": "/v1/embeddings",
		},
	}, embedding.EmbedRequest{
		Texts: []string{"a", "b"},
		Params: map[string]interface{}{
			"dimensions":      float64(64),
			"encoding_format": "float",
		},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if result.ProviderRequestID != "sf-embed-1" || result.Usage.TotalTokens != 5 || len(result.Vectors) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.ModelCode != "qwen3-embedding-0.6b" || result.ProviderModelID != "Qwen/Qwen3-Embedding-0.6B" || result.Dimensions != 2 {
		t.Fatalf("unexpected result model metadata: %#v", result)
	}
}

func TestEmbedUsesDefaultEndpointPath(t *testing.T) {
	adapter := NewAdapter(fakeEmbeddingHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.siliconflow.cn/v1/embeddings" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"index":0,"embedding":[1,2]}]}`)),
		}, nil
	}))

	result, err := adapter.Embed(t.Context(), embedding.ProviderConfig{
		BaseURL:         "https://api.siliconflow.cn",
		CredentialValue: "secret-key",
		ProviderModelID: "Qwen/Qwen3-Embedding-0.6B",
	}, embedding.EmbedRequest{
		Texts: []string{"hello"},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if result.ModelCode != "qwen3-embedding-0.6b" || result.Dimensions != 2 {
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
		BaseURL:         "https://api.siliconflow.cn",
		CredentialValue: "secret-key",
		ProviderModelID: "Qwen/Qwen3-Embedding-0.6B",
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
	if providerErr.Category != providererror.CategoryAuth || providerErr.Retryable || providerErr.ProviderCode != "siliconflow" || providerErr.ProviderRequestID != "auth-failed-1" {
		t.Fatalf("unexpected provider error: %#v", providerErr)
	}
}
