package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/cheetahbyte/lore/internal/identity"
)

// maxLLMsTxtLinks caps how many entries from an llms.txt link list are
// followed, so a single `lore add` can't turn into an unbounded crawl. Keep
// this high enough for real-world indexes such as react.dev, which currently
// lists well over 100 documentation pages.
const maxLLMsTxtLinks = 500

// LLMsTxt is the llms-txt: Source adapter, decided in issue #8. Sites of
// this type have no version concept, per issue #9 — Resolve always
// returns exactly one implicit version, "".
type LLMsTxt struct {
	HTTPClient *http.Client
}

func NewLLMsTxt() *LLMsTxt {
	return &LLMsTxt{HTTPClient: http.DefaultClient}
}

func (l *LLMsTxt) Type() identity.SourceType { return identity.LLMsTxt }

func (l *LLMsTxt) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	domain := strings.TrimPrefix(strings.TrimPrefix(ref, "https://"), "http://")
	domain = strings.TrimSuffix(domain, "/")
	if domain == "" {
		return "", nil, fmt.Errorf("llms-txt: empty domain in ref %q", ref)
	}
	return identity.New(identity.LLMsTxt, domain), []string{""}, nil
}

func (l *LLMsTxt) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	base := "https://" + id.Ref()

	content, sourceURL, err := l.fetchFirst(ctx, base+"/llms-full.txt", base+"/llms.txt")
	if err != nil {
		return nil, fmt.Errorf("llms-txt: fetch index: %w", err)
	}

	pages := []RawPage{{URL: sourceURL, Content: content, ContentType: "markdown"}}

	links := extractMarkdownLinks(content, base)
	if len(links) > maxLLMsTxtLinks {
		links = links[:maxLLMsTxtLinks]
	}
	for _, linkURL := range links {
		linked, err := l.fetchText(ctx, linkURL)
		if err != nil {
			continue // skip individual link failures
		}
		pages = append(pages, RawPage{URL: linkURL, Content: linked, ContentType: "markdown"})
	}
	return pages, nil
}

func (l *LLMsTxt) fetchFirst(ctx context.Context, urls ...string) (content, usedURL string, err error) {
	for _, u := range urls {
		content, err = l.fetchText(ctx, u)
		if err == nil {
			return content, u, nil
		}
	}
	return "", "", err
}

func (l *LLMsTxt) fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var markdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// extractMarkdownLinks pulls absolute-URL markdown links out of an
// llms.txt file's body, per the llms.txt convention of listing docs as
// `- [title](url): description` under H2 sections.
func extractMarkdownLinks(content, base string) []string {
	var urls []string
	for _, m := range markdownLinkRe.FindAllStringSubmatch(content, -1) {
		link := m[2]
		if strings.HasPrefix(link, "/") {
			link = base + link
		}
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			urls = append(urls, link)
		}
	}
	return urls
}
