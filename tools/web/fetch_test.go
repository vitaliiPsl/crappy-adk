package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchURL_PlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello from server"))
	}))
	defer srv.Close()

	out, err := fetchURL(context.Background(), srv.URL, 1000)
	if err != nil {
		t.Fatalf("fetchURL: %v", err)
	}

	if !strings.Contains(out, "Status: 200 OK") {
		t.Fatalf("output %q does not include status", out)
	}

	if !strings.Contains(out, "hello from server") {
		t.Fatalf("output %q does not include body", out)
	}
}

func TestFetchURL_HTMLToText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Hello</h1><p>world</p><script>bad()</script></body></html>`))
	}))
	defer srv.Close()

	out, err := fetchURL(context.Background(), srv.URL, 1000)
	if err != nil {
		t.Fatalf("fetchURL: %v", err)
	}

	if !strings.Contains(out, "Hello") || !strings.Contains(out, "world") {
		t.Fatalf("output %q does not include extracted text", out)
	}

	if strings.Contains(out, "bad()") {
		t.Fatalf("output %q should not include script content", out)
	}
}

func TestFetchURL_InvalidScheme(t *testing.T) {
	_, err := fetchURL(context.Background(), "file:///tmp/test.txt", 1000)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewFetch_InvalidTimeout(t *testing.T) {
	tool := NewFetch()
	zero := 0

	_, err := tool.Execute(context.Background(), map[string]any{
		"url":     "https://example.com",
		"timeout": zero,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "timeout must be greater than 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchURL_TruncatesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("a", 200)))
	}))
	defer srv.Close()

	out, err := fetchURL(context.Background(), srv.URL, 20)
	if err != nil {
		t.Fatalf("fetchURL: %v", err)
	}

	if !strings.Contains(out, "... truncated") {
		t.Fatalf("output %q should include truncation marker", out)
	}
}

func TestFetchURL_DoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("final destination"))
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/docs", http.StatusFound)
	}))
	defer redirect.Close()

	out, err := fetchURL(context.Background(), redirect.URL, 1000)
	if err != nil {
		t.Fatalf("fetchURL: %v", err)
	}

	if !strings.Contains(out, "Status: 302 Found") {
		t.Fatalf("output %q does not include redirect status", out)
	}

	if !strings.Contains(out, "Location: "+target.URL+"/docs") {
		t.Fatalf("output %q does not include redirect location", out)
	}

	if strings.Contains(out, "final destination") {
		t.Fatalf("output %q should not include redirected body", out)
	}
}
