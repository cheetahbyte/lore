package ingest

import (
	"context"
	"testing"

	"github.com/cheetahbyte/lore/internal/identity"
	"github.com/cheetahbyte/lore/internal/source"
	"github.com/cheetahbyte/lore/internal/store"
)

type fakeSource struct {
	pages    map[string][]source.RawPage // keyed by "ref@version"
	versions []string
}

func (f *fakeSource) Type() identity.SourceType { return identity.URL }

func (f *fakeSource) Resolve(ctx context.Context, ref string) (identity.ID, []string, error) {
	return identity.New(identity.URL, ref), f.versions, nil
}

func (f *fakeSource) Fetch(ctx context.Context, id identity.ID, version string) ([]source.RawPage, error) {
	return f.pages[id.Ref()+"@"+version], nil
}

func openTestStore(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), dir+"/index.db", dir+"/vectors.db", false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAdd_IndexesDefaultVersion(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	src := &fakeSource{
		// Resolve is documented to return its preferred/default version
		// first (see ingest.pickDefaultVersion) — v2.0.0 here mimics an
		// adapter that's already identified it as the latest.
		versions: []string{"v2.0.0", "v1.0.0"},
		pages: map[string][]source.RawPage{
			"example.com@v2.0.0": {{URL: "https://example.com/1", Content: "# Title\n\nHello world.", ContentType: "markdown"}},
		},
	}
	p := &Pipeline{Sources: source.Registry{identity.URL: src}, Store: st}

	id, version, err := p.Add(ctx, "url:example.com")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if version != "v2.0.0" {
		t.Errorf("expected default to resolve to v2.0.0, got %q", version)
	}

	results, err := st.SearchChunks(ctx, store.SearchRequest{
		TenantID: store.LocalTenant, LibraryID: id.String(), Version: version, Query: "hello", TopK: 5,
	})
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected the indexed content to be searchable")
	}
}

func TestRefresh_PreservesUnchangedPagesAlongsideChanged(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	src := &fakeSource{
		versions: []string{""},
		pages: map[string][]source.RawPage{
			"example.com@": {
				{URL: "https://example.com/a", Content: "content A", ContentType: "text"},
				{URL: "https://example.com/b", Content: "content B", ContentType: "text"},
			},
		},
	}
	p := &Pipeline{Sources: source.Registry{identity.URL: src}, Store: st}

	id, version, err := p.Add(ctx, "url:example.com")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Change only page B's content; page A stays byte-identical.
	src.pages["example.com@"][1].Content = "content B changed"

	if err := p.Refresh(ctx, id.String(), version); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	hashes, err := st.ContentHashes(ctx, store.LocalTenant, id.String(), version)
	if err != nil {
		t.Fatalf("ContentHashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("expected both pages' chunks to still be present after refresh, got %d: %v", len(hashes), hashes)
	}
	if _, ok := hashes["https://example.com/a"]; !ok {
		t.Error("unchanged page A's chunks were lost on refresh")
	}
}
