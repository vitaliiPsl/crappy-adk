package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapStreamError(code, message string) error {
	detail := errorDetail(message, code, 0)

	switch responses.ResponseErrorCode(code) {
	case responses.ResponseErrorCodeRateLimitExceeded:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, detail)
	case responses.ResponseErrorCodeInvalidPrompt:
		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, detail)
	default:
		return fmt.Errorf("%w: %s", kit.ErrServerError, detail)
	}
}

func mapError(err error) error {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return err
	}

	detail := openAIErrorDetail(apiErr)

	switch apiErr.StatusCode {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, detail)
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, detail)
	case 400:
		if apiErr.Code == "context_length_exceeded" {
			return fmt.Errorf("%w: %s", kit.ErrContextLength, detail)
		}

		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, detail)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, detail)
	default:
		return err
	}
}

func openAIErrorDetail(apiErr *openai.Error) string {
	if message := strings.TrimSpace(apiErr.Message); message != "" {
		return message
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string          `json:"message"`
		Detail  json.RawMessage `json:"detail"`
	}
	if json.Unmarshal([]byte(apiErr.RawJSON()), &body) == nil {
		if message := strings.TrimSpace(body.Error.Message); message != "" {
			return message
		}

		if message := strings.TrimSpace(body.Message); message != "" {
			return message
		}

		if len(body.Detail) > 0 {
			var detail string
			if json.Unmarshal(body.Detail, &detail) == nil && strings.TrimSpace(detail) != "" {
				return strings.TrimSpace(detail)
			}

			if detail := strings.TrimSpace(string(body.Detail)); detail != "" && detail != "null" {
				return detail
			}
		}
	}

	return errorDetail("", apiErr.Code, apiErr.StatusCode)
}

func errorDetail(message, code string, status int) string {
	if message = strings.TrimSpace(message); message != "" {
		return message
	}

	if code = strings.TrimSpace(code); code != "" {
		return code
	}

	if message = http.StatusText(status); message != "" {
		return message
	}

	return "request failed"
}
