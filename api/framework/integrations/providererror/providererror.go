package providererror

import "errors"

const (
	CategoryAuth             = "auth"
	CategoryRateLimit        = "rate_limit"
	CategoryTimeout          = "timeout"
	CategoryValidation       = "validation"
	CategoryPolicyDenied     = "policy_denied"
	CategoryTemporary        = "temporary"
	CategoryPermanent        = "permanent"
	CategoryProviderInternal = "provider_internal"
)

type Error struct {
	Category          string
	Retryable         bool
	SafeMessage       string
	ProviderCode      string
	ProviderRequestID string
	Cause             error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.SafeMessage != "" {
		return e.SafeMessage
	}
	if e.Category != "" {
		return e.Category
	}
	return "provider request failed"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(category string, retryable bool, safeMessage string, cause error) *Error {
	return &Error{
		Category:    category,
		Retryable:   retryable,
		SafeMessage: safeMessage,
		Cause:       cause,
	}
}

func From(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var providerErr *Error
	if errors.As(err, &providerErr) {
		return providerErr, true
	}
	return nil, false
}
