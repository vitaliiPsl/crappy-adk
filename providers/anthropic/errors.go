package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapError(err error) error {
	var apiErr *anthropicsdk.Error
	if !errors.As(err, &apiErr) {
		return err
	}

	detail := anthropicErrorDetail(apiErr)

	switch apiErr.StatusCode {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, detail)
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, detail)
	case 529:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, detail)
	case 400:
		raw := apiErr.RawJSON()
		if strings.Contains(raw, "prompt is too long") || strings.Contains(raw, "context limit") {
			return fmt.Errorf("%w: %s", kit.ErrContextLength, detail)
		}

		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, detail)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, detail)
	default:
		return err
	}
}

func anthropicErrorDetail(apiErr *anthropicsdk.Error) string {
	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(apiErr.RawJSON()), &body) == nil {
		if message := strings.TrimSpace(body.Error.Message); message != "" {
			return message
		}

		if errorType := strings.TrimSpace(body.Error.Type); errorType != "" {
			return errorType
		}
	}

	if message := http.StatusText(apiErr.StatusCode); message != "" {
		return message
	}

	return "request failed"
}
