package providers

import (
	"context"
	"net/http"
	"testing"
)

func TestWithBearerToken(t *testing.T) {
	options := ModelOptions{}
	WithAPIKey("api-key")(&options)
	WithBearerToken("bearer-token")(&options)

	if options.APIKey != "api-key" {
		t.Fatalf("APIKey = %q, want api-key", options.APIKey)
	}

	if options.BearerToken != "bearer-token" {
		t.Fatalf("BearerToken = %q, want bearer-token", options.BearerToken)
	}
}

func TestWithCredentialsSource(t *testing.T) {
	source := testCredentialsSource{}
	options := ModelOptions{}
	WithCredentialsSource(source)(&options)

	if options.CredentialsSource != source {
		t.Fatalf("CredentialsSource = %T, want testCredentialsSource", options.CredentialsSource)
	}
}

type testCredentialsSource struct{}

func (testCredentialsSource) Headers(context.Context) (http.Header, error) {
	return nil, nil
}

func TestWithHeadersClonesHeaders(t *testing.T) {
	headers := map[string]string{"X-Test": "original"}
	options := ModelOptions{}
	WithHeaders(headers)(&options)

	headers["X-Test"] = "changed"

	if options.Headers["X-Test"] != "original" {
		t.Fatalf("Headers = %v, want original X-Test", options.Headers)
	}
}
