package openai

import (
	"encoding/json"
	"errors"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func convertError(err error) error {
	var streamErr *ssestream.StreamError
	if errors.As(err, &streamErr) {
		return convertStreamError(streamErr, err)
	}

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
	case "insufficient_quota", "rate_limit_error":
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

// sseErrorPayload matches the JSON structure inside an SSE error event:
// {"error":{"type":"...","code":"...","message":"..."}}
type sseErrorPayload struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func convertStreamError(streamErr *ssestream.StreamError, original error) error {
	var payload sseErrorPayload
	if err := json.Unmarshal(streamErr.Event.Data, &payload); err != nil {
		return &kit.LLMError{
			Kind:      kit.ErrServerError,
			Message:   streamErr.Message,
			Retryable: false,
			Provider:  ProviderID,
			Cause:     original,
		}
	}

	kind := kit.ErrServerError
	retryable := false

	switch payload.Error.Code {
	case "rate_limit_exceeded":
		kind = kit.ErrRateLimit
		retryable = true
	case "context_length_exceeded":
		kind = kit.ErrContextLength
	case "invalid_api_key":
		kind = kit.ErrAuthentication
	}

	return &kit.LLMError{
		Kind:      kind,
		Message:   payload.Error.Message,
		Retryable: retryable,
		Provider:  ProviderID,
		Cause:     original,
	}
}
