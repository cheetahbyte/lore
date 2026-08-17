package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
		{[]string{"1.0.0", "2.0.0"}, "", []string{"1.0.0", "2.0.0"}},      // no preference: unchanged
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

func TestLLMsTxtFetchFollowsLargeIndex(t *testing.T) {
	const linkCount = 25

	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/llms-full.txt":
			http.NotFound(w, r)
		case "/llms.txt":
			var index strings.Builder
			for i := range linkCount {
				fmt.Fprintf(&index, "- [Page %d](%s/docs/%d.md)\n", i, server.URL, i)
			}
			_, _ = w.Write([]byte(index.String()))
		default:
			_, _ = fmt.Fprintf(w, "# Page\n\nContent for %s", r.URL.Path)
		}
	}))
	defer server.Close()

	source := &LLMsTxt{HTTPClient: server.Client()}
	id, _, err := source.Resolve(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	pages, err := source.Fetch(context.Background(), id, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// The index itself plus every linked page should be fetched. This guards
	// against a low crawl cap silently omitting entries late in real indexes.
	if got, want := len(pages), linkCount+1; got != want {
		t.Fatalf("fetched %d pages, want %d", got, want)
	}
	if !strings.Contains(pages[len(pages)-1].Content, "/docs/24.md") {
		t.Errorf("last linked page was not fetched: %q", pages[len(pages)-1].Content)
	}
}
