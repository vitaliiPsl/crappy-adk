package web

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	fetchName        = "web_fetch"
	fetchDescription = "Fetch a web page by URL and return a readable text snapshot. Use this to inspect documentation pages, articles, APIs, and other public web content."

	defaultMaxChars = 12_000
	maxFetchBytes   = 256_000
	defaultTimeout  = 20 * time.Second
)

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	blockTagRe    = regexp.MustCompile(`(?i)</?(p|div|section|article|main|aside|header|footer|nav|li|ul|ol|table|tr|td|th|h[1-6]|br)[^>]*>`)
	tagRe         = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe       = regexp.MustCompile(`[ \t\r\f\v]+`)
)

type FetchInput struct {
	URL      string `json:"url" jsonschema:"HTTP or HTTPS URL to fetch"`
	MaxChars *int   `json:"max_chars,omitempty" jsonschema:"Maximum number of characters to return. Defaults to 12000."`
	Timeout  *int   `json:"timeout,omitempty" jsonschema:"Timeout in seconds. Defaults to 20s."`
}

func NewFetch() kit.Tool {
	return tool.MustFunction(
		fetchName,
		fetchDescription,
		func(ctx context.Context, input FetchInput) (string, error) {
			maxChars := defaultMaxChars
			if input.MaxChars != nil && *input.MaxChars > 0 {
				maxChars = *input.MaxChars
			}

			timeout := defaultTimeout
			if input.Timeout != nil {
				if *input.Timeout <= 0 {
					return "", fmt.Errorf("timeout must be greater than 0")
				}

				timeout = time.Duration(*input.Timeout) * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return fetchURL(ctx, input.URL, maxChars)
		},
	)
}

func fetchURL(ctx context.Context, rawURL string, maxChars int) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("url must use http or https")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "crappy-adk/web-fetch")

	client := &http.Client{
		Timeout: timeoutFromContext(ctx, defaultTimeout),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, http.ErrUseLastResponse) {
			return "", fmt.Errorf("fetch url: unexpected redirect handling error")
		}

		return "", fmt.Errorf("fetch url: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	text := formatBody(contentType, string(body))
	text = truncateText(text, maxChars)

	var out strings.Builder
	fmt.Fprintf(&out, "URL: %s\n", parsed.String())
	fmt.Fprintf(&out, "Status: %s\n", resp.Status)

	if contentType != "" {
		fmt.Fprintf(&out, "Content-Type: %s\n", contentType)
	}

	if location := resp.Header.Get("Location"); location != "" {
		fmt.Fprintf(&out, "Location: %s\n", location)
	}

	out.WriteString("\n")
	out.WriteString(text)

	return strings.TrimSpace(out.String()), nil
}

func timeoutFromContext(ctx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout > 0 {
			return timeout
		}
	}

	return fallback
}

func formatBody(contentType, body string) string {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return htmlToText(body)
	}

	return strings.TrimSpace(body)
}

// TODO: convert html to markdow
func htmlToText(body string) string {
	text := scriptStyleRe.ReplaceAllString(body, " ")
	text = blockTagRe.ReplaceAllString(text, "\n")
	text = tagRe.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)

	lines := strings.Split(text, "\n")

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(spaceRe.ReplaceAllString(line, " "))
		if line != "" {
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

func truncateText(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}

	return text[:maxChars] + "\n\n... truncated"
}
