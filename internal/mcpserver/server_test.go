package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/cheetahbyte/lore/internal/chunk"
	"github.com/cheetahbyte/lore/internal/store"
)

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

func TestTools_EndToEnd(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	if err := st.UpsertChunks(ctx, store.LocalTenant, "github:facebook/react", "v18.0.0", []chunk.Chunk{
		{DocURL: "https://x", SectionPath: "Install", Content: "npm install react to get started", ContentHash: "h", FetchedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: st}

	_, listOut, err := s.listLibraries(ctx, nil, listLibrariesInput{})
	if err != nil {
		t.Fatalf("listLibraries: %v", err)
	}
	if len(listOut.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(listOut.Libraries))
	}

	_, resolveOut, err := s.resolveLibrary(ctx, nil, resolveLibraryInput{Query: "react"})
	if err != nil {
		t.Fatalf("resolveLibrary: %v", err)
	}
	if len(resolveOut.Matches) != 1 || resolveOut.Matches[0].LibraryID != "github:facebook/react" {
		t.Fatalf("expected to resolve react, got %+v", resolveOut.Matches)
	}

	_, searchOut, err := s.searchDocs(ctx, nil, searchDocsInput{
		LibraryID: "github:facebook/react", Query: "install", TopK: 5,
	})
	if err != nil {
		t.Fatalf("searchDocs: %v", err)
	}
	if len(searchOut.Results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if searchOut.Results[0].SectionPath != "Install" {
		t.Errorf("expected result under Install, got %q", searchOut.Results[0].SectionPath)
	}
}
