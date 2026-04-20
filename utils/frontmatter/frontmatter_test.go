package frontmatter_test

import (
	"testing"

	"github.com/vitaliiPsl/crappy-adk/utils/frontmatter"
)

type meta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMeta meta
		wantBody string
		wantErr  bool
	}{
		{
			name:     "no frontmatter",
			input:    "just a body",
			wantMeta: meta{},
			wantBody: "just a body",
		},
		{
			name:     "frontmatter and body",
			input:    "---\nname: researcher\ndescription: does research\n---\n\nYou are a researcher.",
			wantMeta: meta{Name: "researcher", Description: "does research"},
			wantBody: "You are a researcher.",
		},
		{
			name:     "frontmatter only no body",
			input:    "---\nname: writer\n---\n",
			wantMeta: meta{Name: "writer"},
			wantBody: "",
		},
		{
			name:     "unclosed frontmatter treated as no frontmatter",
			input:    "---\nname: broken\n",
			wantMeta: meta{},
			wantBody: "---\nname: broken\n",
		},
		{
			name:     "empty input",
			input:    "",
			wantMeta: meta{},
			wantBody: "",
		},
		{
			name:    "invalid yaml",
			input:   "---\n: bad: yaml: here\n---\nbody",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, body, err := frontmatter.Unmarshal[meta](tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if got != tt.wantMeta {
				t.Errorf("meta = %+v, want %+v", got, tt.wantMeta)
			}

			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
