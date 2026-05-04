package google

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapError(err error) error {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.Code {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, apiErr.Message)
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, apiErr.Message)
	case 400:
		msg := strings.ToLower(apiErr.Message)
		if strings.Contains(msg, "prompt token count") || strings.Contains(msg, "context limit") {
			return fmt.Errorf("%w: %s", kit.ErrContextLength, apiErr.Message)
		}

		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, apiErr.Message)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, apiErr.Message)
	default:
		return err
	}
}
