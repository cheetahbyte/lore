package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cheetahbyte/lore/internal/identity"
)

// GitHub is the github: Source adapter, decided in issue #8. It reads
// tags/releases for versioning (issue #9) and README/docs markdown files
// for content, using the unauthenticated GitHub REST API by default (set
// GITHUB_TOKEN or GH_TOKEN for a higher rate limit).
type GitHub struct {
	HTTPClient *http.Client
	Token      string
}

func NewGitHub() *GitHub {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return &GitHub{HTTPClient: http.DefaultClient, Token: token}
}

func (g *GitHub) Type() identity.SourceType { return identity.GitHub }

func (g *GitHub) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	owner, repo, ok := strings.Cut(ref, "/")
	if !ok || owner == "" || repo == "" {
		return "", nil, fmt.Errorf("github: ref %q must be in owner/repo form", ref)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := g.getJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=100", owner, repo), &tags); err != nil {
		return "", nil, fmt.Errorf("github: list tags: %w", err)
	}

	versions := make([]string, 0, len(tags))
	for _, t := range tags {
		versions = append(versions, t.Name)
	}

	// GitHub's "latest release" excludes pre-releases and drafts, which
	// raw tag order doesn't — prefer it as the default, per issue #9.
	var latestRelease struct {
		TagName string `json:"tag_name"`
	}
	if err := g.getJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo), &latestRelease); err == nil {
		versions = moveToFront(versions, latestRelease.TagName)
	}

	if len(versions) == 0 {
		var repoInfo struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := g.getJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), &repoInfo); err != nil {
			return "", nil, fmt.Errorf("github: get repo info: %w", err)
		}
		if repoInfo.DefaultBranch == "" {
			repoInfo.DefaultBranch = "main"
		}
		versions = []string{repoInfo.DefaultBranch}
	}

	return identity.New(identity.GitHub, owner+"/"+repo), versions, nil
}

func (g *GitHub) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	owner, repo, ok := strings.Cut(id.Ref(), "/")
	if !ok {
		return nil, fmt.Errorf("github: identity %q must be in owner/repo form", id)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	archiveURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, version)
	if body, err := g.get(ctx, archiveURL, "application/vnd.github+json"); err == nil {
		defer body.Close()
		if pages, archiveErr := archivePages(body, fmt.Sprintf("https://github.com/%s/%s/blob/%s", owner, repo, version)); archiveErr == nil && len(pages) > 0 {
			return pages, nil
		}
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, version)
	if err := g.getJSON(ctx, url, &tree); err != nil {
		return nil, fmt.Errorf("github: get tree: %w", err)
	}

	var pages []RawPage
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !isDocPath(entry.Path) {
			continue
		}
		content, err := g.getRaw(ctx, fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, version, entry.Path))
		if err != nil {
			continue // skip individual fetch failures rather than failing the whole ingest
		}
		pages = append(pages, RawPage{
			URL:         fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, version, entry.Path),
			Path:        entry.Path,
			Content:     content,
			ContentType: "markdown",
		})
	}
	return pages, nil
}

// isDocPath decides which repo files count as documentation: markdown
// files anywhere, favoring README* and docs/doc directories.
func isDocPath(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	if ext != ".md" && ext != ".mdx" {
		return false
	}
	lower := strings.ToLower(p)
	return strings.HasPrefix(lower, "readme") ||
		strings.Contains(lower, "docs/") ||
		strings.Contains(lower, "doc/") ||
		!strings.Contains(lower, "/") // top-level markdown files
}

func (g *GitHub) getJSON(ctx context.Context, url string, out any) error {
	body, err := g.get(ctx, url, "application/vnd.github+json")
	if err != nil {
		return err
	}
	defer body.Close()
	return json.NewDecoder(body).Decode(out)
}

func (g *GitHub) getRaw(ctx context.Context, url string) (string, error) {
	body, err := g.get(ctx, url, "")
	if err != nil {
		return "", err
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (g *GitHub) get(ctx context.Context, url, accept string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}
