package anthropic

import (
	"net/http"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/providers"
)

const apiKeyHeader = "X-Api-Key"

func resolveCredentials(options providers.ModelOptions) providers.CredentialsSource {
	switch {
	case options.Credentials != nil:
		return options.Credentials
	case options.APIKey != "":
		return providers.Static(http.Header{apiKeyHeader: {options.APIKey}})
	default:
		return nil
	}
}

func credentialsMiddleware(source providers.CredentialsSource) option.Middleware {
	return func(request *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		headers, err := source.Headers(request.Context())
		if err != nil {
			return nil, err
		}

		request.Header.Del("Authorization")
		request.Header.Del(apiKeyHeader)

		for name, values := range headers {
			request.Header.Del(name)

			for _, value := range values {
				request.Header.Add(name, value)
			}
		}

		return next(request)
	}
}
