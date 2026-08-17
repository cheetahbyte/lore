package source

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"
)

func archivePages(r io.Reader, baseURL string) ([]RawPage, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("source: open archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var pages []RawPage
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("source: read archive: %w", err)
		}
		name := strings.TrimPrefix(path.Clean(h.Name), "./")
		if name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			continue
		}
		if h.Typeflag != tar.TypeReg || !isIndexablePath(name) || h.Size > 2<<20 {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if len(content) == 0 || strings.IndexByte(string(content), 0) >= 0 {
			continue
		}
		name = stripArchiveRoot(name)
		pages = append(pages, RawPage{URL: baseURL + "/" + name, Path: name, Content: string(content), ContentType: contentType(name)})
	}
	return pages, nil
}

func isIndexablePath(name string) bool {
	lower := strings.ToLower(name)
	for _, ignored := range []string{"/node_modules/", "/vendor/", "/.git/", "/dist/", "/build/", "/target/", "/coverage/"} {
		if strings.Contains("/"+lower, ignored) {
			return false
		}
	}
	for _, ext := range []string{".md", ".mdx", ".rst", ".txt", ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".kt", ".rb", ".yaml", ".yml", ".json", ".toml"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func stripArchiveRoot(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func contentType(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".md") || strings.HasSuffix(strings.ToLower(name), ".mdx") || strings.HasSuffix(strings.ToLower(name), ".rst") {
		return "markdown"
	}
	return "text"
}
