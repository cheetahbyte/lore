package store

import (
	"context"
	"testing"
	"time"

	"github.com/cheetahbyte/lore/internal/chunk"
)

func openTestStore(t *testing.T, vectorEnabled bool) *SQLite {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), dir+"/index.db", dir+"/vectors.db", vectorEnabled)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndSearch_FTSOnly(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, false)

	chunks := []chunk.Chunk{
		{DocURL: "https://x/1", SectionPath: "Install", Content: "Run go install to install the tool", ContentHash: "h1", FetchedAt: time.Now()},
		{DocURL: "https://x/2", SectionPath: "Config", Content: "Set the API key in the config file", ContentHash: "h2", FetchedAt: time.Now()},
	}
	if err := s.UpsertChunks(ctx, LocalTenant, "github:owner/repo", "v1.0.0", chunks); err != nil {
		t.Fatalf("UpsertChunks: %v", err)
	}

	results, err := s.SearchChunks(ctx, SearchRequest{
		TenantID: LocalTenant, LibraryID: "github:owner/repo", Version: "v1.0.0",
		Query: "install", TopK: 5,
	})
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].Chunk.SectionPath != "Install" {
		t.Errorf("expected top result under Install, got %q", results[0].Chunk.SectionPath)
	}
}

func TestUpsertReplacesExistingVersion(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, false)

	first := []chunk.Chunk{{DocURL: "https://x/1", Content: "old content", ContentHash: "h1", FetchedAt: time.Now()}}
	if err := s.UpsertChunks(ctx, LocalTenant, "npm:react", "18.0.0", first); err != nil {
		t.Fatalf("UpsertChunks (first): %v", err)
	}
	second := []chunk.Chunk{{DocURL: "https://x/2", Content: "new content", ContentHash: "h2", FetchedAt: time.Now()}}
	if err := s.UpsertChunks(ctx, LocalTenant, "npm:react", "18.0.0", second); err != nil {
		t.Fatalf("UpsertChunks (second): %v", err)
	}

	hashes, err := s.ContentHashes(ctx, LocalTenant, "npm:react", "18.0.0")
	if err != nil {
		t.Fatalf("ContentHashes: %v", err)
	}
	if len(hashes) != 1 || hashes["https://x/2"] != "h2" {
		t.Errorf("expected only the second upsert's chunk to remain, got %v", hashes)
	}
}

func TestListAndResolveLibraries(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, false)

	c := []chunk.Chunk{{DocURL: "u", Content: "c", ContentHash: "h", FetchedAt: time.Now()}}
	if err := s.UpsertChunks(ctx, LocalTenant, "github:facebook/react", "v18.0.0", c); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertChunks(ctx, LocalTenant, "npm:lodash", "", c); err != nil {
		t.Fatal(err)
	}

	libs, err := s.ListLibraries(ctx, LocalTenant)
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d: %+v", len(libs), libs)
	}

	resolved, err := s.ResolveLibraries(ctx, LocalTenant, "react")
	if err != nil {
		t.Fatalf("ResolveLibraries: %v", err)
	}
	if len(resolved) != 1 || resolved[0].LibraryID != "github:facebook/react" {
		t.Errorf("expected to resolve react, got %+v", resolved)
	}
}

func TestLatestVersion_PicksHighestSemver(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, false)
	c := []chunk.Chunk{{DocURL: "u", Content: "c", ContentHash: "h", FetchedAt: time.Now()}}

	for _, v := range []string{"v1.0.0", "v2.1.0", "v1.9.0"} {
		if err := s.UpsertChunks(ctx, LocalTenant, "github:owner/repo", v, c); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := s.LatestVersion(ctx, LocalTenant, "github:owner/repo")
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if latest != "v2.1.0" {
		t.Errorf("LatestVersion = %q, want v2.1.0", latest)
	}
}

func TestSearchChunks_WithVectorFusion(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, true)

	chunks := []chunk.Chunk{
		{DocURL: "https://x/1", Content: "alpha content about installing", ContentHash: "h1", FetchedAt: time.Now(), Embedding: []float32{1, 0, 0, 0}},
		{DocURL: "https://x/2", Content: "beta content about configuring", ContentHash: "h2", FetchedAt: time.Now(), Embedding: []float32{0, 1, 0, 0}},
	}
	if err := s.UpsertChunks(ctx, LocalTenant, "url:https://docs.example.com", "", chunks); err != nil {
		t.Fatalf("UpsertChunks: %v", err)
	}

	results, err := s.SearchChunks(ctx, SearchRequest{
		TenantID: LocalTenant, LibraryID: "url:https://docs.example.com",
		Query: "content", TopK: 5, Vector: []float32{1, 0, 0, 0},
	})
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 fused results, got %d", len(results))
	}
	if results[0].Chunk.DocURL != "https://x/1" {
		t.Errorf("expected the vector-nearest chunk first, got %q", results[0].Chunk.DocURL)
	}
}

func TestDeleteLibrary(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, false)
	c := []chunk.Chunk{{DocURL: "u", Content: "c", ContentHash: "h", FetchedAt: time.Now()}}

	if err := s.UpsertChunks(ctx, LocalTenant, "pypi:requests", "2.0.0", c); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLibrary(ctx, LocalTenant, "pypi:requests", ""); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}
	libs, err := s.ListLibraries(ctx, LocalTenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 0 {
		t.Errorf("expected no libraries after delete, got %+v", libs)
	}
}
