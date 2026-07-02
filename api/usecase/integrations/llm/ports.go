package llm

import "context"

const (
	Scenario             = "llm"
	OperationTextSummary = "text_summary"

	ResponseFormatJSON = "json_object"
)

type Message struct {
	Role    string
	Content string
}

type GenerateRequest struct {
	Operation      string
	Messages       []Message
	ResponseFormat string
	Params         map[string]interface{}
}

type ProviderConfig struct {
	ChannelCode       string
	ProviderCode      string
	AdapterKey        string
	BaseURL           string
	APIKey            string
	ModelCode         string
	ProviderModelID   string
	DefaultParamsJSON string
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type GenerateResult struct {
	Content           string
	ProviderRequestID string
	Usage             Usage
}

type Adapter interface {
	Generate(ctx context.Context, cfg ProviderConfig, req GenerateRequest) (GenerateResult, error)
}
