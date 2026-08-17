package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cheetahbyte/lore/internal/chunk"
	"github.com/cheetahbyte/lore/internal/retrieval"
)

// SearchChunks implements the retrieval pipeline decided in issue #7: FTS5
// always runs; when req.Vector is set (vector search is enabled and the
// caller supplied a query embedding), vec1's nearest-neighbor results are
// fused with the FTS5 ranking via Reciprocal Rank Fusion.
func (s *SQLite) SearchChunks(ctx context.Context, req SearchRequest) ([]ScoredChunk, error) {
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}

	version := req.Version
	if version == "" {
		v, err := s.LatestVersion(ctx, req.TenantID, req.LibraryID)
		if err != nil {
			return nil, err
		}
		version = v
	}

	ftsRanking, err := s.ftsRanking(ctx, req.TenantID, req.LibraryID, version, req.Query, topK)
	if err != nil {
		return nil, err
	}

	rankings := [][]string{ftsRanking}
	if s.vecDB != nil && req.Vector != nil {
		vecRanking, err := s.vecRanking(ctx, req.TenantID, req.LibraryID, version, req.Vector, topK)
		if err != nil {
			return nil, err
		}
		rankings = append(rankings, vecRanking)
	}

	fused := retrieval.Fuse(retrieval.DefaultK, rankings...)
	if len(fused) > topK {
		fused = fused[:topK]
	}

	return s.hydrateChunks(ctx, req.TenantID, req.LibraryID, version, fused)
}

// ftsRanking returns rowids (as strings) ranked by FTS5's bm25 relevance.
func (s *SQLite) ftsRanking(ctx context.Context, tenantID, libraryID, version, query string, topK int) ([]string, error) {
	matchExpr := ftsMatchExpr(query)
	if matchExpr == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT chunks.rowid
		FROM chunks_fts
		JOIN chunks ON chunks.rowid = chunks_fts.rowid
		WHERE chunks_fts MATCH ?
			AND chunks.tenant_id = ? AND chunks.library_id = ? AND chunks.version = ?
		ORDER BY rank
		LIMIT ?
	`, matchExpr, tenantID, libraryID, version, topK)
	if err != nil {
		return nil, fmt.Errorf("store: fts search: %w", err)
	}
	defer rows.Close()
	return scanRowidStrings(rows)
}

// vecRanking returns rowids (as strings) ranked by vec1 distance ascending
// (nearest first), restricted to the requested library+version.
func (s *SQLite) vecRanking(ctx context.Context, tenantID, libraryID, version string, queryVector []float32, topK int) ([]string, error) {
	memberRowids, err := s.libraryRowids(ctx, tenantID, libraryID, version)
	if err != nil {
		return nil, err
	}
	if len(memberRowids) == 0 {
		return nil, nil
	}
	blob, err := encodeVector(queryVector)
	if err != nil {
		return nil, err
	}
	// vec1 doesn't know about tenant/library/version scoping, so we search
	// wider than topK and filter to this library's rowids in application
	// code. Good enough for the local, single-tenant index sizes this is
	// designed for; a hosted backend can revisit this.
	rows, err := s.vecDB.QueryContext(ctx, `
		SELECT rowid FROM vec_chunks(?, ?)
	`, blob, topK*20)
	if err != nil {
		return nil, fmt.Errorf("store: vector search: %w", err)
	}
	defer rows.Close()

	member := make(map[int64]bool, len(memberRowids))
	for _, id := range memberRowids {
		member[id] = true
	}

	var out []string
	for rows.Next() {
		var rowid int64
		if err := rows.Scan(&rowid); err != nil {
			return nil, err
		}
		if member[rowid] {
			out = append(out, strconv.FormatInt(rowid, 10))
			if len(out) >= topK {
				break
			}
		}
	}
	return out, rows.Err()
}

func (s *SQLite) libraryRowids(ctx context.Context, tenantID, libraryID, version string) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT rowid FROM chunks WHERE tenant_id = ? AND library_id = ? AND version = ?
	`, tenantID, libraryID, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SQLite) hydrateChunks(ctx context.Context, tenantID, libraryID, version string, rankedRowids []string) ([]ScoredChunk, error) {
	out := make([]ScoredChunk, 0, len(rankedRowids))
	stmt, err := s.db.PrepareContext(ctx, `
		SELECT doc_url, section_path, ordinal, content, content_hash, fetched_at
		FROM chunks WHERE rowid = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("store: prepare hydrate: %w", err)
	}
	defer stmt.Close()

	for i, rowidStr := range rankedRowids {
		var c chunk.Chunk
		var fetchedAtUnix int64
		err := stmt.QueryRowContext(ctx, rowidStr).Scan(
			&c.DocURL, &c.SectionPath, &c.Ordinal, &c.Content, &c.ContentHash, &fetchedAtUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("store: hydrate chunk %s: %w", rowidStr, err)
		}
		c.TenantID = tenantID
		c.LibraryID = libraryID
		c.Version = version
		c.ID = rowidStr
		c.FetchedAt = time.Unix(fetchedAtUnix, 0)
		// Rank position (best first) stands in for score: RRF's fused
		// value isn't independently meaningful outside a fixed k, so
		// callers should treat this as a relative ordering signal.
		out = append(out, ScoredChunk{Chunk: c, Score: float64(len(rankedRowids) - i)})
	}
	return out, nil
}

func scanRowidStrings(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]string, error) {
	var out []string
	for rows.Next() {
		var rowid int64
		if err := rows.Scan(&rowid); err != nil {
			return nil, err
		}
		out = append(out, strconv.FormatInt(rowid, 10))
	}
	return out, rows.Err()
}

// ftsMatchExpr turns free text into an FTS5 MATCH expression: each token
// quoted (so punctuation/FTS5 syntax in the query can't break the query
// itself) and OR-joined, relying on FTS5's bm25 ranking to surface the
// best matches rather than requiring every term to be present.
func ftsMatchExpr(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		parts = append(parts, `"`+f+`"`)
	}
	return strings.Join(parts, " OR ")
}
