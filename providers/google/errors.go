package google

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapError(err error) error {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	detail := strings.TrimSpace(apiErr.Message)
	if detail == "" {
		detail = strings.TrimSpace(apiErr.Status)
	}

	if detail == "" {
		detail = http.StatusText(apiErr.Code)
	}

	if detail == "" {
		detail = "request failed"
	}

	switch apiErr.Code {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, detail)
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, detail)
	case 400:
		msg := strings.ToLower(apiErr.Message)
		if strings.Contains(msg, "prompt token count") || strings.Contains(msg, "context limit") {
			return fmt.Errorf("%w: %s", kit.ErrContextLength, detail)
		}

		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, detail)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, detail)
	default:
		return err
	}
}
