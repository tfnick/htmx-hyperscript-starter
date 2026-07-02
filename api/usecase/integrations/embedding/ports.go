package embedding

import "context"

const OperationCreate = "embedding_create"

type ProviderConfig struct {
	ChannelID        string
	ChannelCode      string
	AdapterKey       string
	Provider         string
	CredentialValue   string
	BaseURL           string
	ModelCode         string
	ProviderModelID   string
	ModelSettings     map[string]interface{}
	ProviderSettings  map[string]interface{}
}

type EmbedRequest struct {
	Operation string
	Texts     []string
	Params    map[string]interface{}
}

type Vector struct {
	Values []float32
}

type Usage struct {
	PromptTokens int
	TotalTokens  int
}

type EmbedResult struct {
	Vectors           []Vector
	ProviderRequestID string
	ModelCode         string
	ProviderModelID   string
	Dimensions        int
	Usage             Usage
}

type Adapter interface {
	Embed(ctx context.Context, cfg ProviderConfig, req EmbedRequest) (EmbedResult, error)
}
