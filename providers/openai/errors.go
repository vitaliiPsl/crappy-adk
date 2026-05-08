package openai

import (
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapStreamError(code, message string) error {
	switch responses.ResponseErrorCode(code) {
	case responses.ResponseErrorCodeRateLimitExceeded:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, message)
	case responses.ResponseErrorCodeInvalidPrompt:
		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, message)
	default:
		return fmt.Errorf("%w: %s: %s", kit.ErrServerError, code, message)
	}
}

func mapError(err error) error {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.StatusCode {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, apiErr.Message)
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, apiErr.Message)
	case 400:
		if apiErr.Code == "context_length_exceeded" {
			return fmt.Errorf("%w: %s", kit.ErrContextLength, apiErr.Message)
		}

		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, apiErr.Message)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, apiErr.Message)
	default:
		return err
	}
}
