package glob

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "main.go", true},
		{"**/*.go", "internal/server/server.go", true},
		{"**/*.go", "README.md", false},
		{"internal/**/*.go", "internal/server/server.go", true},
		{"internal/**/*.go", "cmd/main.go", false},
		{"*.go", "main.go", true},
		{"*.go", "internal/main.go", false},
		{"**/*_test.go", "main_test.go", true},
		{"**/*_test.go", "internal/server/server_test.go", true},
		{"**/*_test.go", "server.go", false},
	}

	for _, tc := range cases {
		got := Match(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
