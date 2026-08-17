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

func TestMoveToFront(t *testing.T) {
	cases := []struct {
		versions  []string
		preferred string
		want      []string
	}{
		{[]string{"1.0.0", "2.0.0", "3.0.0"}, "2.0.0", []string{"2.0.0", "1.0.0", "3.0.0"}},
		{[]string{"1.0.0", "2.0.0"}, "1.0.0", []string{"1.0.0", "2.0.0"}},
		{[]string{"1.0.0", "2.0.0"}, "9.9.9", []string{"1.0.0", "2.0.0"}}, // not present: unchanged
		{[]string{"1.0.0", "2.0.0"}, "", []string{"1.0.0", "2.0.0"}},     // no preference: unchanged
	}
	for _, c := range cases {
		got := moveToFront(append([]string(nil), c.versions...), c.preferred)
		if len(got) != len(c.want) {
			t.Fatalf("moveToFront(%v, %q) = %v, want %v", c.versions, c.preferred, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("moveToFront(%v, %q) = %v, want %v", c.versions, c.preferred, got, c.want)
				break
			}
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
