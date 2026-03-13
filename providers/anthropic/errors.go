package anthropic

import (
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func convertError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return &kit.LLMError{
			Kind:      kit.ErrServerError,
			Message:   err.Error(),
			Retryable: false,
			Provider:  providerID,
			Cause:     err,
		}
	}

	kind := kit.ErrServerError
	retryable := false

	switch apiErr.StatusCode {
	case 400:
		if strings.Contains(apiErr.RawJSON(), "context") {
			kind = kit.ErrContextLength
		} else {
			kind = kit.ErrInvalidRequest
		}
	case 401:
		kind = kit.ErrAuthentication
	case 429:
		kind = kit.ErrRateLimit
		retryable = true
	case 529:
		kind = kit.ErrRateLimit
		retryable = true
	}

	return &kit.LLMError{
		Kind:       kind,
		Message:    apiErr.Error(),
		StatusCode: apiErr.StatusCode,
		Retryable:  retryable,
		Provider:   providerID,
		Cause:      err,
	}
}
