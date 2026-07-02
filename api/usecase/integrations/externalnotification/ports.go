package externalnotification

import "context"

const OperationSend = "send"

type ProviderConfig struct {
	ProviderCode string
	AdapterKey   string
	Credential   string
	ConfigJSON   string
}

type MessageField struct {
	Label string
	Value string
}

type SendRequest struct {
	Title      string
	Summary    string
	EventTopic string
	Fields     []MessageField
}

type SendResult struct {
	ProviderRequestID string
	ResponseSnapshot  string
}

type Adapter interface {
	Send(context.Context, ProviderConfig, SendRequest) (SendResult, error)
}

type ProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e ProviderError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "external notification provider error"
}
