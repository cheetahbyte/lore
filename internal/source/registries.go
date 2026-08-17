package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/cheetahbyte/lore/internal/identity"
)

func doJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// NPM is the npm: Source adapter, decided in issue #8. Versions come from
// the registry's own version list; per issue #9, the default version
// prefers the registry's own "latest" dist-tag over a blind semver-max
// (which would otherwise pick prerelease/canary builds that carry a
// higher base version than the actual stable release — confirmed against
// the real react packument, where dist-tags.latest is 19.2.8 but the
// highest semver-sortable version is a 19.3.0-canary-* build).
type NPM struct{ HTTPClient *http.Client }

func NewNPM() *NPM { return &NPM{HTTPClient: http.DefaultClient} }

func (n *NPM) Type() identity.SourceType { return identity.NPM }

type npmPackument struct {
	Readme   string                     `json:"readme"`
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]npmVersionEntry `json:"versions"`
}
type npmVersionEntry struct {
	Readme string `json:"readme"`
}

func (n *NPM) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	var doc npmPackument
	if err := doJSON(ctx, n.HTTPClient, "https://registry.npmjs.org/"+ref, &doc); err != nil {
		return "", nil, fmt.Errorf("npm: fetch packument: %w", err)
	}
	if len(doc.Versions) == 0 {
		return "", nil, fmt.Errorf("npm: package %q has no versions", ref)
	}
	versions := make([]string, 0, len(doc.Versions))
	for v := range doc.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	// "latest" leads the list — see the doc comment on NPM above.
	versions = moveToFront(versions, doc.DistTags["latest"])
	return identity.New(identity.NPM, ref), versions, nil
}

func (n *NPM) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	var doc npmPackument
	if err := doJSON(ctx, n.HTTPClient, "https://registry.npmjs.org/"+id.Ref(), &doc); err != nil {
		return nil, fmt.Errorf("npm: fetch packument: %w", err)
	}
	readme := doc.Versions[version].Readme
	if readme == "" {
		readme = doc.Readme
	}
	pageURL := fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", id.Ref(), version)
	if readme != "" {
		return []RawPage{{URL: pageURL, Content: readme, ContentType: "markdown"}}, nil
	}

	// Many packages (react among them) simply don't carry a readme in
	// their registry metadata at all, at any level. unpkg.com serves the
	// package's actual published files, so fall back to the README.md it
	// shipped in the tarball rather than silently indexing nothing.
	if content, err := fetchText(ctx, n.HTTPClient, fmt.Sprintf("https://unpkg.com/%s@%s/README.md", id.Ref(), version)); err == nil && content != "" {
		return []RawPage{{URL: pageURL, Content: content, ContentType: "markdown"}}, nil
	}

	return nil, fmt.Errorf("npm: no readme found for %s@%s (checked registry metadata and unpkg.com)", id.Ref(), version)
}

// moveToFront reorders versions so that preferred (if present in the
// slice) becomes element 0, leaving the rest in their existing order.
// Adapters use this to hand ingest.pickDefaultVersion a source-specific
// notion of "latest" instead of forcing a generic semver-max guess.
func moveToFront(versions []string, preferred string) []string {
	if preferred == "" {
		return versions
	}
	for i, v := range versions {
		if v == preferred {
			out := make([]string, 0, len(versions))
			out = append(out, preferred)
			out = append(out, versions[:i]...)
			out = append(out, versions[i+1:]...)
			return out
		}
	}
	return versions
}

// PyPI is the pypi: Source adapter, decided in issue #8.
type PyPI struct{ HTTPClient *http.Client }

func NewPyPI() *PyPI { return &PyPI{HTTPClient: http.DefaultClient} }

func (p *PyPI) Type() identity.SourceType { return identity.PyPI }

type pypiResponse struct {
	Info struct {
		Description string `json:"description"`
		Version     string `json:"version"` // PyPI's own notion of the current release, per issue #9
	} `json:"info"`
	Releases map[string]json.RawMessage `json:"releases"`
}

func (p *PyPI) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	var doc pypiResponse
	if err := doJSON(ctx, p.HTTPClient, "https://pypi.org/pypi/"+ref+"/json", &doc); err != nil {
		return "", nil, fmt.Errorf("pypi: fetch project: %w", err)
	}
	if len(doc.Releases) == 0 {
		return "", nil, fmt.Errorf("pypi: package %q has no releases", ref)
	}
	versions := make([]string, 0, len(doc.Releases))
	for v := range doc.Releases {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	versions = moveToFront(versions, doc.Info.Version)
	return identity.New(identity.PyPI, ref), versions, nil
}

func (p *PyPI) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	var doc pypiResponse
	url := fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", id.Ref(), version)
	if err := doJSON(ctx, p.HTTPClient, url, &doc); err != nil {
		return nil, fmt.Errorf("pypi: fetch release: %w", err)
	}
	if doc.Info.Description == "" {
		return nil, fmt.Errorf("pypi: no description found for %s@%s", id.Ref(), version)
	}
	pageURL := fmt.Sprintf("https://pypi.org/project/%s/%s/", id.Ref(), version)
	return []RawPage{{URL: pageURL, Content: doc.Info.Description, ContentType: "markdown"}}, nil
}

// GoPackage is the pkg.go.dev: Source adapter, decided in issue #8.
// Versions come from the Go module proxy's @v/list; content is the
// rendered pkg.go.dev page (HTML), converted the same way the url:
// adapter converts crawled pages, since pkg.go.dev has no JSON doc API.
type GoPackage struct {
	HTTPClient *http.Client
	URLAdapter *URL
}

func NewGoPackage() *GoPackage {
	return &GoPackage{HTTPClient: http.DefaultClient, URLAdapter: NewURL()}
}

func (g *GoPackage) Type() identity.SourceType { return identity.GoPackage }

func (g *GoPackage) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://proxy.golang.org/"+ref+"/@v/list", nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("pkg.go.dev: fetch version list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("pkg.go.dev: unexpected status %d listing versions for %q", resp.StatusCode, ref)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	var versions []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(buf)), "\n") {
		if line != "" {
			versions = append(versions, line)
		}
	}
	if len(versions) == 0 {
		versions = []string{"latest"}
	}

	// The proxy's own @latest endpoint is the module-aware notion of
	// "current version" (respects major-version suffixes and pseudo-
	// versions the way @v/list's raw tag listing doesn't), so prefer it
	// as the default, per issue #9.
	var latest struct {
		Version string `json:"Version"`
	}
	if err := doJSON(ctx, g.HTTPClient, "https://proxy.golang.org/"+ref+"/@latest", &latest); err == nil {
		versions = moveToFront(versions, latest.Version)
	}

	return identity.New(identity.GoPackage, ref), versions, nil
}

func (g *GoPackage) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	pageURL := fmt.Sprintf("https://pkg.go.dev/%s@%s", id.Ref(), version)
	body, err := fetchText(ctx, g.HTTPClient, pageURL)
	if err != nil {
		return nil, fmt.Errorf("pkg.go.dev: fetch doc page: %w", err)
	}
	text, _ := extractTextAndLinks(body, pageURL)
	return []RawPage{{URL: pageURL, Content: text, ContentType: "text"}}, nil
}

func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
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
