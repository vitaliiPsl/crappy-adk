package custom

import (
	"errors"

	openaisdk "github.com/openai/openai-go/v3"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func convertError(err error) error {
	var apiErr *openaisdk.Error
	if !errors.As(err, &apiErr) {
		return &kit.LLMError{
			Kind:      kit.ErrServerError,
			Message:   err.Error(),
			Retryable: false,
			Provider:  ProviderID,
			Cause:     err,
		}
	}

	kind := kit.ErrServerError
	retryable := false

	switch apiErr.Type {
	case "invalid_request_error":
		if apiErr.Code == "context_length_exceeded" {
			kind = kit.ErrContextLength
		} else {
			kind = kit.ErrInvalidRequest
		}
	case "rate_limit_error":
		kind = kit.ErrRateLimit
		retryable = true
	case "authentication_error":
		kind = kit.ErrAuthentication
	}

	return &kit.LLMError{
		Kind:       kind,
		Message:    apiErr.Message,
		StatusCode: apiErr.StatusCode,
		Retryable:  retryable,
		Provider:   ProviderID,
		Cause:      err,
	}
}
