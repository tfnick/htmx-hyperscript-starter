package siliconflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
)

const (
	defaultEmbeddingsPath = "/v1/embeddings"
	defaultModelCode      = "qwen3-embedding-0.6b"

	providerCodeSiliconFlow     = "siliconflow"
	providerSettingEndpointPath = "endpoint_path"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Adapter struct {
	client HTTPDoer
}

func NewAdapter(client HTTPDoer) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Adapter{client: client}
}

type embedRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

type embedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type embedUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type embedResponse struct {
	Object string      `json:"object"`
	Data   []embedData `json:"data"`
	Model  string      `json:"model"`
	Usage  embedUsage  `json:"usage"`
}

func (a *Adapter) Embed(ctx context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	if err := validateConfig(cfg); err != nil {
		return embedding.EmbedResult{}, err
	}
	if len(req.Texts) == 0 {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryValidation, false, "embedding texts are required", nil)
	}

	body, err := buildEmbedRequestBody(cfg, req)
	if err != nil {
		return embedding.EmbedResult{}, err
	}

	endpoint, err := embeddingEndpointURL(cfg.BaseURL, endpointPath(cfg.ProviderSettings))
	if err != nil {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryValidation, false, "embedding base URL is invalid", err)
	}

	responseBody, header, err := a.doEmbeddingRequest(ctx, cfg, endpoint, body)
	if err != nil {
		return embedding.EmbedResult{}, err
	}

	var parsed embedResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is invalid", err)
	}
	if len(parsed.Data) == 0 {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is empty", nil)
	}

	vectors := make([]embedding.Vector, len(parsed.Data))
	for i, data := range parsed.Data {
		if len(data.Embedding) == 0 {
			return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding vector is empty", nil)
		}
		vectors[i] = embedding.Vector{Values: data.Embedding}
	}

	providerModelID := strings.TrimSpace(cfg.ProviderModelID)
	if strings.TrimSpace(parsed.Model) != "" {
		providerModelID = strings.TrimSpace(parsed.Model)
	}

	return embedding.EmbedResult{
		Vectors:           vectors,
		ProviderRequestID: firstHeader(header, "X-Request-ID", "X-Request-Id"),
		ModelCode:         resultModelCode(cfg),
		ProviderModelID:   providerModelID,
		Dimensions:        len(vectors[0].Values),
		Usage:             embedding.Usage{PromptTokens: parsed.Usage.PromptTokens, TotalTokens: parsed.Usage.TotalTokens},
	}, nil
}

func validateConfig(cfg embedding.ProviderConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return providererror.New(providererror.CategoryValidation, false, "embedding base URL is required", nil)
	}
	if strings.TrimSpace(cfg.CredentialValue) == "" {
		return providererror.New(providererror.CategoryAuth, false, "embedding credential is required", nil)
	}
	if strings.TrimSpace(cfg.ProviderModelID) == "" {
		return providererror.New(providererror.CategoryValidation, false, "embedding model is required", nil)
	}
	return nil
}

func buildEmbedRequestBody(cfg embedding.ProviderConfig, req embedding.EmbedRequest) ([]byte, error) {
	payload := embedRequest{
		Model: strings.TrimSpace(cfg.ProviderModelID),
		Input: req.Texts,
	}
	if encodingFormat := paramString(req.Params, "encoding_format"); encodingFormat != "" {
		payload.EncodingFormat = encodingFormat
	}
	if dimensions := paramPositiveInt(req.Params, "dimensions"); dimensions > 0 {
		payload.Dimensions = dimensions
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, providererror.New(providererror.CategoryValidation, false, "embedding request is invalid", err)
	}
	return body, nil
}

func (a *Adapter) doEmbeddingRequest(ctx context.Context, cfg embedding.ProviderConfig, endpoint string, body []byte) ([]byte, http.Header, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, providererror.New(providererror.CategoryTemporary, true, "failed to create embedding request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.CredentialValue)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, nil, providererror.New(providererror.CategoryTemporary, true, "embedding provider request failed", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return nil, resp.Header, providererror.New(providererror.CategoryTemporary, true, "failed to read embedding response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, embeddingErrorFromStatus(resp.StatusCode, firstHeader(resp.Header, "X-Request-ID", "X-Request-Id"))
	}
	return responseBody, resp.Header, nil
}

func embeddingEndpointURL(base string, endpointPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}

	endpointPath = strings.TrimSpace(endpointPath)
	if endpointPath == "" {
		endpointPath = defaultEmbeddingsPath
	}
	if strings.HasPrefix(endpointPath, "http://") || strings.HasPrefix(endpointPath, "https://") {
		endpoint, err := url.Parse(endpointPath)
		if err != nil {
			return "", err
		}
		if endpoint.Scheme == "" || endpoint.Host == "" {
			return "", fmt.Errorf("endpoint URL must include scheme and host")
		}
		endpoint.RawQuery = ""
		endpoint.Fragment = ""
		return endpoint.String(), nil
	}
	if strings.HasPrefix(endpointPath, "/") {
		parsed.Path = endpointPath
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + endpointPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func endpointPath(settings map[string]interface{}) string {
	path := providerSettingString(settings, providerSettingEndpointPath)
	if path == "" {
		return defaultEmbeddingsPath
	}
	return path
}

func providerSettingString(settings map[string]interface{}, key string) string {
	if len(settings) == 0 {
		return ""
	}
	value, ok := settings[key]
	if !ok {
		return ""
	}
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func paramString(params map[string]interface{}, key string) string {
	if len(params) == 0 {
		return ""
	}
	raw, ok := params[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func paramPositiveInt(params map[string]interface{}, key string) int {
	if len(params) == 0 {
		return 0
	}
	switch value := params[key].(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	case json.Number:
		parsed, err := value.Int64()
		if err == nil && parsed > 0 {
			return int(parsed)
		}
	}
	return 0
}

func resultModelCode(cfg embedding.ProviderConfig) string {
	modelCode := strings.TrimSpace(cfg.ModelCode)
	if modelCode == "" {
		return defaultModelCode
	}
	return modelCode
}

func embeddingErrorFromStatus(statusCode int, providerRequestID string) *providererror.Error {
	category := providererror.CategoryProviderInternal
	retryable := statusCode >= 500 || statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		category = providererror.CategoryAuth
		retryable = false
	case http.StatusTooManyRequests:
		category = providererror.CategoryRateLimit
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		category = providererror.CategoryTimeout
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		category = providererror.CategoryValidation
		retryable = false
	default:
		if statusCode >= 400 && statusCode < 500 {
			category = providererror.CategoryPermanent
			retryable = false
		}
	}

	return &providererror.Error{
		Category:          category,
		Retryable:         retryable,
		SafeMessage:       "embedding provider request failed",
		ProviderCode:      providerCodeSiliconFlow,
		ProviderRequestID: providerRequestID,
	}
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}
