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
// the registry's own version list; readmes are per-version, per issue #9.
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
	versions := make([]string, 0, len(doc.Versions))
	for v := range doc.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		return "", nil, fmt.Errorf("npm: package %q has no versions", ref)
	}
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
	if readme == "" {
		return nil, nil
	}
	pageURL := fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", id.Ref(), version)
	return []RawPage{{URL: pageURL, Content: readme, ContentType: "markdown"}}, nil
}

// PyPI is the pypi: Source adapter, decided in issue #8.
type PyPI struct{ HTTPClient *http.Client }

func NewPyPI() *PyPI { return &PyPI{HTTPClient: http.DefaultClient} }

func (p *PyPI) Type() identity.SourceType { return identity.PyPI }

type pypiResponse struct {
	Info struct {
		Description string `json:"description"`
	} `json:"info"`
	Releases map[string]json.RawMessage `json:"releases"`
}

func (p *PyPI) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	var doc pypiResponse
	if err := doJSON(ctx, p.HTTPClient, "https://pypi.org/pypi/"+ref+"/json", &doc); err != nil {
		return "", nil, fmt.Errorf("pypi: fetch project: %w", err)
	}
	versions := make([]string, 0, len(doc.Releases))
	for v := range doc.Releases {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		return "", nil, fmt.Errorf("pypi: package %q has no releases", ref)
	}
	return identity.New(identity.PyPI, ref), versions, nil
}

func (p *PyPI) Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error) {
	var doc pypiResponse
	url := fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", id.Ref(), version)
	if err := doJSON(ctx, p.HTTPClient, url, &doc); err != nil {
		return nil, fmt.Errorf("pypi: fetch release: %w", err)
	}
	if doc.Info.Description == "" {
		return nil, nil
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
