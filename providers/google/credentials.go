package google

import (
	"net/http"

	"github.com/vitaliiPsl/crappy-adk/providers"
)

const (
	apiKeyHeader = "X-Goog-Api-Key"

	unusedAPIKey = "unused"
)

type credentialsTransport struct {
	base   http.RoundTripper
	source providers.CredentialsSource
}

func (t credentialsTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	headers, err := t.source.Headers(request.Context())
	if err != nil {
		return nil, err
	}

	cloned := request.Clone(request.Context())

	cloned.Header.Del("Authorization")
	cloned.Header.Del(apiKeyHeader)

	for name, values := range headers {
		cloned.Header.Del(name)

		for _, value := range values {
			cloned.Header.Add(name, value)
		}
	}

	return t.base.RoundTrip(cloned)
}

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

func credentialsClient(source providers.CredentialsSource) *http.Client {
	return &http.Client{
		Transport: credentialsTransport{
			base:   http.DefaultTransport,
			source: source,
		},
	}
}
