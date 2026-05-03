package anthropic

import (
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func mapError(err error) error {
	var apiErr *anthropicsdk.Error
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.StatusCode {
	case 401, 403:
		return fmt.Errorf("%w: %s", kit.ErrAuthentication, apiErr.Error())
	case 429:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, apiErr.Error())
	case 529:
		return fmt.Errorf("%w: %s", kit.ErrRateLimit, apiErr.Error())
	case 400:
		return fmt.Errorf("%w: %s", kit.ErrInvalidRequest, apiErr.Error())
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", kit.ErrServerError, apiErr.Error())
	default:
		return err
	}
}
