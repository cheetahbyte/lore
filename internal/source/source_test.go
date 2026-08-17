package source

import "testing"

func TestIsDocPath(t *testing.T) {
	cases := map[string]bool{
		"README.md":         true,
		"docs/guide.md":     true,
		"src/main.go":       false,
		"docs/nested/x.mdx": true,
		"top-level.md":      true,
		"src/notes.md":      false,
	}
	for path, want := range cases {
		if got := isDocPath(path); got != want {
			t.Errorf("isDocPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestExtractMarkdownLinks(t *testing.T) {
	content := "# Docs\n\n## Guides\n\n- [Getting Started](/docs/start): intro\n- [API](https://example.com/api): reference\n"
	links := extractMarkdownLinks(content, "https://example.com")
	want := map[string]bool{
		"https://example.com/docs/start": true,
		"https://example.com/api":        true,
	}
	if len(links) != len(want) {
		t.Fatalf("got %v links, want %d", links, len(want))
	}
	for _, l := range links {
		if !want[l] {
			t.Errorf("unexpected link %q", l)
		}
	}
}
