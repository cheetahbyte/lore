package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

func (s *SQLite) ListLibraries(ctx context.Context, tenantID string) ([]LibraryInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT library_id, version, max(fetched_at)
		FROM chunks
		WHERE tenant_id = ?
		GROUP BY library_id, version
		ORDER BY library_id, version
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list libraries: %w", err)
	}
	defer rows.Close()
	return scanLibraryInfos(rows)
}

func (s *SQLite) ResolveLibraries(ctx context.Context, tenantID, query string) ([]LibraryInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT library_id, version, max(fetched_at)
		FROM chunks
		WHERE tenant_id = ? AND lower(library_id) LIKE lower(?)
		GROUP BY library_id, version
		ORDER BY library_id, version
	`, tenantID, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("store: resolve libraries: %w", err)
	}
	defer rows.Close()
	return scanLibraryInfos(rows)
}

func scanLibraryInfos(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]LibraryInfo, error) {
	byLibrary := map[string]*LibraryInfo{}
	var order []string
	for rows.Next() {
		var libraryID, version string
		var fetchedAtUnix int64
		if err := rows.Scan(&libraryID, &version, &fetchedAtUnix); err != nil {
			return nil, fmt.Errorf("store: scan library row: %w", err)
		}
		info, ok := byLibrary[libraryID]
		if !ok {
			info = &LibraryInfo{LibraryID: libraryID}
			byLibrary[libraryID] = info
			order = append(order, libraryID)
		}
		if version != "" {
			info.Versions = append(info.Versions, version)
		}
		fetchedAt := time.Unix(fetchedAtUnix, 0)
		if fetchedAt.After(info.LastIndexedAt) {
			info.LastIndexedAt = fetchedAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]LibraryInfo, 0, len(order))
	for _, id := range order {
		out = append(out, *byLibrary[id])
	}
	return out, nil
}

// LatestVersion implements the default-version rule decided in issue #9:
// highest semver / registry-latest, falling back to the one version that
// exists for versionless sources.
func (s *SQLite) LatestVersion(ctx context.Context, tenantID, libraryID string) (string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT version FROM chunks WHERE tenant_id = ? AND library_id = ?
	`, tenantID, libraryID)
	if err != nil {
		return "", fmt.Errorf("store: query versions: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return "", err
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("store: library %q not indexed", libraryID)
	}
	return pickLatest(versions), nil
}

func pickLatest(versions []string) string {
	if len(versions) == 1 {
		return versions[0]
	}
	best := versions[0]
	bestSemver := semverKey(best)
	for _, v := range versions[1:] {
		if k := semverKey(v); k != "" && (bestSemver == "" || semver.Compare(k, bestSemver) > 0) {
			best, bestSemver = v, k
		} else if bestSemver == "" && k == "" && v > best {
			// Neither is valid semver: fall back to lexicographic order.
			best = v
		}
	}
	return best
}

// semverKey normalizes v into a form golang.org/x/mod/semver accepts (it
// requires a leading "v"), returning "" if v still isn't valid semver.
func semverKey(v string) string {
	k := v
	if !strings.HasPrefix(k, "v") {
		k = "v" + k
	}
	if !semver.IsValid(k) {
		return ""
	}
	return k
}

func (s *SQLite) ContentHashes(ctx context.Context, tenantID, libraryID, version string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT doc_url, content_hash FROM chunks
		WHERE tenant_id = ? AND library_id = ? AND version = ?
	`, tenantID, libraryID, version)
	if err != nil {
		return nil, fmt.Errorf("store: query content hashes: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var url, hash string
		if err := rows.Scan(&url, &hash); err != nil {
			return nil, err
		}
		out[url] = hash
	}
	return out, rows.Err()
}
