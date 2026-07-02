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
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
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

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage llm.Usage `json:"usage"`
}

func (a *Adapter) Generate(ctx context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	if err := validateConfig(cfg); err != nil {
		return llm.GenerateResult{}, err
	}
	if len(req.Messages) == 0 {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryValidation, false, "LLM messages are required", nil)
	}

	body, err := buildChatRequestBody(cfg, req)
	if err != nil {
		return llm.GenerateResult{}, err
	}

	endpoint, err := chatCompletionsURL(cfg.BaseURL)
	if err != nil {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryValidation, false, "LLM base URL is invalid", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to create LLM request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryTemporary, true, "LLM provider request failed", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to read LLM response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return llm.GenerateResult{}, providerErrorFromStatus(resp.StatusCode, firstHeader(resp.Header, "X-Request-ID", "X-Request-Id"))
	}

	var parsed chatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryProviderInternal, true, "LLM response is invalid", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return llm.GenerateResult{}, providererror.New(providererror.CategoryProviderInternal, true, "LLM response is empty", nil)
	}

	return llm.GenerateResult{
		Content:           parsed.Choices[0].Message.Content,
		ProviderRequestID: parsed.ID,
		Usage:             parsed.Usage,
	}, nil
}

func validateConfig(cfg llm.ProviderConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return providererror.New(providererror.CategoryValidation, false, "LLM base URL is required", nil)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return providererror.New(providererror.CategoryAuth, false, "LLM credential is required", nil)
	}
	if strings.TrimSpace(cfg.ProviderModelID) == "" {
		return providererror.New(providererror.CategoryValidation, false, "LLM model is required", nil)
	}
	return nil
}

func buildChatRequestBody(cfg llm.ProviderConfig, req llm.GenerateRequest) ([]byte, error) {
	payload := map[string]interface{}{}
	if strings.TrimSpace(cfg.DefaultParamsJSON) != "" {
		if err := json.Unmarshal([]byte(cfg.DefaultParamsJSON), &payload); err != nil {
			return nil, providererror.New(providererror.CategoryValidation, false, "LLM model params are invalid", err)
		}
	}
	for key, value := range req.Params {
		payload[key] = value
	}

	messages := make([]chatMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" || content == "" {
			return nil, providererror.New(providererror.CategoryValidation, false, "LLM message is invalid", nil)
		}
		messages = append(messages, chatMessage{Role: role, Content: content})
	}

	payload["model"] = cfg.ProviderModelID
	payload["messages"] = messages
	if req.ResponseFormat == llm.ResponseFormatJSON {
		payload["response_format"] = responseFormat{Type: "json_object"}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, providererror.New(providererror.CategoryValidation, false, "LLM request is invalid", err)
	}
	return body, nil
}

func chatCompletionsURL(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func providerErrorFromStatus(statusCode int, providerRequestID string) *providererror.Error {
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
		SafeMessage:       "LLM provider request failed",
		ProviderRequestID: providerRequestID,
	}
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
		if values, ok := header[name]; ok && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
