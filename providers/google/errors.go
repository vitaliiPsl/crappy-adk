package google

import (
	"errors"
	"net/http"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func convertError(err error) error {
	var apiErr *genai.APIError
	if !errors.As(err, &apiErr) {
		return &kit.LLMError{
			Kind:      kit.ErrServerError,
			Message:   err.Error(),
			Retryable: false,
			Provider:  ProviderID,
			Cause:     err,
		}
	}

	retryable, kind := classifyStatusCode(apiErr.Code)

	return &kit.LLMError{
		Kind:       kind,
		Message:    apiErr.Message,
		StatusCode: apiErr.Code,
		Retryable:  retryable,
		Provider:   ProviderID,
		Cause:      err,
	}
}

func classifyStatusCode(code int) (bool, error) {
	switch code {
	case http.StatusTooManyRequests:
		return true, kit.ErrRateLimit
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true, kit.ErrServerError
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, kit.ErrAuthentication
	case http.StatusBadRequest:
		return false, kit.ErrInvalidRequest
	case http.StatusRequestEntityTooLarge:
		return false, kit.ErrContextLength
	default:
		return false, kit.ErrServerError
	}
}
