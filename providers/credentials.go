package providers

import (
	"context"
	"net/http"
)

// CredentialsSource resolves the headers that authorize a model request. It is
// asked for every request, so tokens that rotate or expire stay current without
// rebuilding the model.
type CredentialsSource interface {
	Headers(context.Context) (http.Header, error)
}

// Static returns a source that resolves the same headers for every request.
func Static(headers http.Header) CredentialsSource {
	return staticCredentials{headers: headers.Clone()}
}

type staticCredentials struct {
	headers http.Header
}

func (c staticCredentials) Headers(context.Context) (http.Header, error) {
	return c.headers, nil
}
