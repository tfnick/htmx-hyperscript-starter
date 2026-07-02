package deepseek

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
	apiStyleDeepSeekEmbedding = "deepseek_embedding"
	apiStyleOpenAICompatible  = "openai_compatible"

	providerSettingAPIStyle     = "api_style"
	providerSettingEndpointPath = "endpoint_path"

	defaultDeepSeekEmbeddingPath = "/v1/embedding"
	defaultOpenAIEmbeddingsPath  = "embeddings"
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

type openAIEmbedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type deepSeekEmbedRequest struct {
	Text string `json:"text"`
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

type openAIEmbedResponse struct {
	Object string      `json:"object"`
	Data   []embedData `json:"data"`
	Model  string      `json:"model"`
	Usage  embedUsage  `json:"usage"`
}

type deepSeekEmbedResponse struct {
	Embedding []float32   `json:"embedding"`
	Data      []embedData `json:"data"`
	Model     string      `json:"model"`
	Usage     embedUsage  `json:"usage"`
}

func (a *Adapter) Embed(ctx context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	if err := validateConfig(cfg); err != nil {
		return embedding.EmbedResult{}, err
	}
	if len(req.Texts) == 0 {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryValidation, false, "embedding texts are required", nil)
	}

	apiStyle, err := embeddingAPIStyle(cfg)
	if err != nil {
		return embedding.EmbedResult{}, err
	}

	if apiStyle == apiStyleOpenAICompatible {
		return a.embedOpenAICompatible(ctx, cfg, req)
	}
	return a.embedDeepSeek(ctx, cfg, req)
}

func (a *Adapter) embedDeepSeek(ctx context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	endpointPath := endpointPathForAPIStyle(cfg.ProviderSettings, apiStyleDeepSeekEmbedding)
	endpoint, err := embeddingEndpointURL(cfg.BaseURL, endpointPath)
	if err != nil {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryValidation, false, "embedding base URL is invalid", err)
	}

	vectors := make([]embedding.Vector, 0, len(req.Texts))
	var usage embedding.Usage
	providerRequestID := ""
	for _, text := range req.Texts {
		body, err := buildDeepSeekEmbedRequestBody(text)
		if err != nil {
			return embedding.EmbedResult{}, err
		}

		responseBody, header, err := a.doEmbeddingRequest(ctx, cfg, endpoint, body)
		if err != nil {
			return embedding.EmbedResult{}, err
		}
		if providerRequestID == "" {
			providerRequestID = firstHeader(header, "X-Request-ID", "X-Request-Id")
		}

		var parsed deepSeekEmbedResponse
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is invalid", err)
		}

		vector := parsed.Embedding
		if len(vector) == 0 && len(parsed.Data) > 0 {
			vector = parsed.Data[0].Embedding
		}
		if len(vector) == 0 {
			return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is empty", nil)
		}
		vectors = append(vectors, embedding.Vector{Values: vector})
		usage.PromptTokens += parsed.Usage.PromptTokens
		usage.TotalTokens += parsed.Usage.TotalTokens
	}

	return embedding.EmbedResult{
		Vectors:           vectors,
		ProviderRequestID: providerRequestID,
		ModelCode:         resultModelCode(cfg),
		ProviderModelID:   strings.TrimSpace(cfg.ProviderModelID),
		Dimensions:        len(vectors[0].Values),
		Usage:             usage,
	}, nil
}

func (a *Adapter) embedOpenAICompatible(ctx context.Context, cfg embedding.ProviderConfig, req embedding.EmbedRequest) (embedding.EmbedResult, error) {
	body, err := buildOpenAIEmbedRequestBody(cfg, req)
	if err != nil {
		return embedding.EmbedResult{}, err
	}

	endpointPath := endpointPathForAPIStyle(cfg.ProviderSettings, apiStyleOpenAICompatible)
	endpoint, err := embeddingEndpointURL(cfg.BaseURL, endpointPath)
	if err != nil {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryValidation, false, "embedding base URL is invalid", err)
	}

	responseBody, header, err := a.doEmbeddingRequest(ctx, cfg, endpoint, body)
	if err != nil {
		return embedding.EmbedResult{}, err
	}

	var parsed openAIEmbedResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is invalid", err)
	}
	if len(parsed.Data) == 0 {
		return embedding.EmbedResult{}, providererror.New(providererror.CategoryProviderInternal, true, "embedding response is empty", nil)
	}

	vectors := make([]embedding.Vector, len(parsed.Data))
	for i, data := range parsed.Data {
		vectors[i] = embedding.Vector{Values: data.Embedding}
	}

	return embedding.EmbedResult{
		Vectors:           vectors,
		ProviderRequestID: firstHeader(header, "X-Request-ID", "X-Request-Id"),
		ModelCode:         resultModelCode(cfg),
		ProviderModelID:   strings.TrimSpace(cfg.ProviderModelID),
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

func buildOpenAIEmbedRequestBody(cfg embedding.ProviderConfig, req embedding.EmbedRequest) ([]byte, error) {
	payload := openAIEmbedRequest{
		Model: cfg.ProviderModelID,
		Input: req.Texts,
	}
	if dim, ok := req.Params["dimensions"]; ok {
		if d, ok := dim.(float64); ok && d > 0 {
			payload.Dimensions = int(d)
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, providererror.New(providererror.CategoryValidation, false, "embedding request is invalid", err)
	}
	return body, nil
}

func buildDeepSeekEmbedRequestBody(text string) ([]byte, error) {
	body, err := json.Marshal(deepSeekEmbedRequest{Text: text})
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
		return "", fmt.Errorf("endpoint path is required")
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

func embeddingAPIStyle(cfg embedding.ProviderConfig) (string, error) {
	style := providerSettingString(cfg.ProviderSettings, providerSettingAPIStyle)
	switch style {
	case "", apiStyleDeepSeekEmbedding:
		return apiStyleDeepSeekEmbedding, nil
	case apiStyleOpenAICompatible:
		return apiStyleOpenAICompatible, nil
	default:
		return "", providererror.New(providererror.CategoryValidation, false, "embedding api style is invalid", nil)
	}
}

func endpointPathForAPIStyle(settings map[string]interface{}, apiStyle string) string {
	endpointPath := providerSettingString(settings, providerSettingEndpointPath)
	switch apiStyle {
	case apiStyleOpenAICompatible:
		if endpointPath == "" || endpointPath == defaultDeepSeekEmbeddingPath {
			return defaultOpenAIEmbeddingsPath
		}
		return endpointPath
	default:
		if endpointPath == "" {
			return defaultDeepSeekEmbeddingPath
		}
		return endpointPath
	}
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

func resultModelCode(cfg embedding.ProviderConfig) string {
	modelCode := strings.TrimSpace(cfg.ModelCode)
	if modelCode == "" {
		return "deepseek-embedding"
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
