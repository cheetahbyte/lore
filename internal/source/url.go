package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cheetahbyte/lore/internal/identity"
	"golang.org/x/net/html"
)

// CrawlOptions configures URL.Fetch, sourced from the per-source config
// decided in issue #3 (config.toml's [sources."url:..."] table). Passed
// via context since the Source interface itself (issue #8) doesn't carry
// per-call options — WithCrawlOptions/crawlOptionsFrom are the seam.
type CrawlOptions struct {
	Depth   int // hop count from the starting URL; 0 or 1 means "just this page"
	Include []string
	Exclude []string
}

type crawlOptionsKey struct{}

func WithCrawlOptions(ctx context.Context, opts CrawlOptions) context.Context {
	return context.WithValue(ctx, crawlOptionsKey{}, opts)
}

func crawlOptionsFrom(ctx context.Context) CrawlOptions {
	if opts, ok := ctx.Value(crawlOptionsKey{}).(CrawlOptions); ok {
		return opts
	}
	return CrawlOptions{Depth: 1}
}

// maxCrawledPages bounds a single Fetch regardless of Depth, so a
// misconfigured include pattern can't turn into an unbounded crawl.
const maxCrawledPages = 200

// URL is the url: Source adapter (crawled sites), decided in issue #8. No
// version concept, per issue #9 — Resolve always returns one implicit
// version, "".
type URL struct {
	HTTPClient *http.Client
}

func NewURL() *URL {
	return &URL{HTTPClient: http.DefaultClient}
}

func (u *URL) Type() identity.SourceType { return identity.URL }

func (u *URL) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	if _, err := url.ParseRequestURI(ref); err != nil {
		return "", nil, fmt.Errorf("url: invalid URL %q: %w", ref, err)
	}
	return identity.New(identity.URL, ref), []string{""}, nil
}

func (u *URL) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	opts := crawlOptionsFrom(ctx)
	if opts.Depth <= 0 {
		opts.Depth = 1
	}
	start := id.Ref()
	startHost, err := url.Parse(start)
	if err != nil {
		return nil, fmt.Errorf("url: parse start URL: %w", err)
	}

	visited := map[string]bool{}
	queue := []struct {
		url   string
		depth int
	}{{start, 1}}
	var pages []RawPage

	for len(queue) > 0 && len(pages) < maxCrawledPages {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur.url] || !matchesFilters(cur.url, opts) {
			continue
		}
		visited[cur.url] = true

		body, err := u.fetch(ctx, cur.url)
		if err != nil {
			continue
		}
		text, links := extractTextAndLinks(body, cur.url)
		pages = append(pages, RawPage{URL: cur.url, Content: text, ContentType: "text"})

		if cur.depth < opts.Depth {
			for _, link := range links {
				linkURL, err := url.Parse(link)
				if err != nil || linkURL.Host != startHost.Host {
					continue
				}
				if !visited[link] {
					queue = append(queue, struct {
						url   string
						depth int
					}{link, cur.depth + 1})
				}
			}
		}
	}
	return pages, nil
}

func matchesFilters(pageURL string, opts CrawlOptions) bool {
	if len(opts.Include) > 0 {
		matched := false
		for _, pattern := range opts.Include {
			if strings.Contains(pageURL, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range opts.Exclude {
		if strings.Contains(pageURL, pattern) {
			return false
		}
	}
	return true
}

func (u *URL) fetch(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, pageURL)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// extractTextAndLinks does a minimal HTML->text conversion (skip
// script/style, preserve heading markers so the shared chunker in
// internal/chunk can still find section boundaries) and collects same-doc
// links for further crawling.
func extractTextAndLinks(rawHTML, pageURL string) (text string, links []string) {
	base, _ := url.Parse(pageURL)
	tokenizer := html.NewTokenizer(strings.NewReader(rawHTML))
	var sb strings.Builder
	skipDepth := 0

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		tok := tokenizer.Token()
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			switch tok.Data {
			case "script", "style", "nav", "footer":
				if tt == html.StartTagToken {
					skipDepth++
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				sb.WriteString("\n" + strings.Repeat("#", int(tok.Data[1]-'0')) + " ")
			case "a":
				for _, attr := range tok.Attr {
					if attr.Key == "href" {
						if resolved, err := base.Parse(attr.Val); err == nil {
							links = append(links, resolved.String())
						}
					}
				}
			case "br", "p", "li":
				sb.WriteString("\n")
			}
		case html.EndTagToken:
			switch tok.Data {
			case "script", "style", "nav", "footer":
				if skipDepth > 0 {
					skipDepth--
				}
			case "h1", "h2", "h3", "h4", "h5", "h6", "p", "div":
				sb.WriteString("\n")
			}
		case html.TextToken:
			if skipDepth == 0 {
				sb.WriteString(strings.TrimSpace(tok.Data))
				sb.WriteString(" ")
			}
		}
	}
	return sb.String(), links
}
