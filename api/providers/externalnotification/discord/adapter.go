package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/externalnotification"
)

type Adapter struct {
	client *http.Client
}

type credentialConfig struct {
	WebhookURL string `json:"webhook_url"`
}

func NewAdapter(client *http.Client) Adapter {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return Adapter{client: client}
}

func (a Adapter) Send(ctx context.Context, cfg externalnotification.ProviderConfig, msg externalnotification.SendRequest) (externalnotification.SendResult, error) {
	cred, err := decodeCredential(cfg.Credential)
	if err != nil {
		return externalnotification.SendResult{}, externalnotification.ProviderError{Code: "config_invalid", Message: "Discord webhook credential is invalid"}
	}
	if strings.TrimSpace(cred.WebhookURL) == "" {
		return externalnotification.SendResult{}, externalnotification.ProviderError{Code: "config_invalid", Message: "Discord webhook URL is required"}
	}

	payload := map[string]interface{}{
		"content": renderTextMessage(msg),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return externalnotification.SendResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cred.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return externalnotification.SendResult{}, externalnotification.ProviderError{Code: "config_invalid", Message: "Discord webhook URL is invalid"}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return externalnotification.SendResult{}, externalnotification.ProviderError{Code: "timeout", Message: "Discord webhook request failed", Retryable: true}
	}
	defer resp.Body.Close()

	responseSnapshot := fmt.Sprintf(`{"status_code":%d}`, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "provider_error"
		retryable := false
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			code = "provider_5xx"
			retryable = true
		} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			code = "unauthorized"
		}
		return externalnotification.SendResult{}, externalnotification.ProviderError{
			Code:      code,
			Message:   "Discord webhook returned non-success status",
			Retryable: retryable,
		}
	}
	return externalnotification.SendResult{ResponseSnapshot: responseSnapshot}, nil
}

func decodeCredential(value string) (credentialConfig, error) {
	var cfg credentialConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &cfg); err != nil {
		return cfg, err
	}
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
	return cfg, nil
}

func renderTextMessage(msg externalnotification.SendRequest) string {
	var b strings.Builder
	b.WriteString("**")
	b.WriteString(strings.TrimSpace(msg.Title))
	b.WriteString("**")
	for _, field := range msg.Fields {
		value := strings.TrimSpace(field.Value)
		if value == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(field.Label))
		b.WriteString(": ")
		b.WriteString(value)
	}
	return b.String()
}
