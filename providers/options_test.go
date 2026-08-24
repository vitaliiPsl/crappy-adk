package providers

import (
	"context"
	"net/http"
	"testing"
)

func TestWithAPIKey(t *testing.T) {
	options := ModelOptions{}
	WithAPIKey("api-key")(&options)

	if options.APIKey != "api-key" {
		t.Fatalf("APIKey = %q, want api-key", options.APIKey)
	}
}

func TestWithCredentials(t *testing.T) {
	source := Static(http.Header{"Authorization": {"Bearer token"}})

	options := ModelOptions{}
	WithCredentials(source)(&options)

	headers, err := options.Credentials.Headers(context.Background())
	if err != nil {
		t.Fatalf("Headers() error = %v", err)
	}

	if got := headers.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("Authorization = %q, want Bearer token", got)
	}
}

func TestStaticClonesHeaders(t *testing.T) {
	headers := http.Header{"X-Test": {"original"}}
	source := Static(headers)

	headers.Set("X-Test", "changed")

	got, err := source.Headers(context.Background())
	if err != nil {
		t.Fatalf("Headers() error = %v", err)
	}

	if value := got.Get("X-Test"); value != "original" {
		t.Fatalf("X-Test = %q, want original", value)
	}
}
