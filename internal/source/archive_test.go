package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestArchivePagesFiltersAndStripsRoot(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tarw := tar.NewWriter(gz)
	for _, f := range []struct{ name, body string }{{"pkg-1.0/README.md", "# hi"}, {"pkg-1.0/node_modules/x.js", "bad"}, {"pkg-1.0/image.png", "bad"}} {
		if err := tarw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	pages, err := archivePages(&buf, "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].Path != "README.md" {
		t.Fatalf("pages = %#v", pages)
	}
}
