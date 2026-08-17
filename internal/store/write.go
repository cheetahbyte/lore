package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cheetahbyte/lore/internal/chunk"
)

// UpsertChunks implements Store.UpsertChunks: a full re-index of
// tenantID/libraryID/version — existing chunks for that triple are
// replaced wholesale rather than diffed row-by-row, matching how #12's
// refresh flow already decides whether re-indexing is worth doing at all
// (via ContentHashes) before calling this.
func (s *SQLite) UpsertChunks(ctx context.Context, tenantID, libraryID, version string, chunks []chunk.Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin upsert tx: %w", err)
	}
	defer tx.Rollback()

	rowids, err := deleteChunksTx(ctx, tx, tenantID, libraryID, version)
	if err != nil {
		return err
	}

	insert, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks
			(tenant_id, library_id, version, doc_url, section_path, ordinal, content, content_hash, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING rowid
	`)
	if err != nil {
		return fmt.Errorf("store: prepare chunk insert: %w", err)
	}
	defer insert.Close()

	type vecRow struct {
		rowid  int64
		vector []float32
	}
	var vecRows []vecRow

	for _, c := range chunks {
		var rowid int64
		err := insert.QueryRowContext(ctx,
			tenantID, libraryID, version, c.DocURL, c.SectionPath, c.Ordinal, c.Content, c.ContentHash, c.FetchedAt.Unix(),
		).Scan(&rowid)
		if err != nil {
			return fmt.Errorf("store: insert chunk: %w", err)
		}
		if c.Embedding != nil {
			vecRows = append(vecRows, vecRow{rowid: rowid, vector: c.Embedding})
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit upsert tx: %w", err)
	}

	if s.vecDB == nil || (len(vecRows) == 0 && len(rowids) == 0) {
		return nil
	}

	vtx, err := s.vecDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin vector tx: %w", err)
	}
	defer vtx.Rollback()

	for _, rowid := range rowids {
		if _, err := vtx.ExecContext(ctx, `DELETE FROM vec_chunks WHERE rowid = ?`, rowid); err != nil {
			return fmt.Errorf("store: delete stale vector: %w", err)
		}
	}
	if len(vecRows) > 0 {
		vinsert, err := vtx.PrepareContext(ctx, `INSERT INTO vec_chunks(rowid, embedding) VALUES (?, ?)`)
		if err != nil {
			return fmt.Errorf("store: prepare vector insert: %w", err)
		}
		defer vinsert.Close()
		for _, vr := range vecRows {
			blob, err := encodeVector(vr.vector)
			if err != nil {
				return err
			}
			if _, err := vinsert.ExecContext(ctx, vr.rowid, blob); err != nil {
				return fmt.Errorf("store: insert vector: %w", err)
			}
		}
	}
	if err := vtx.Commit(); err != nil {
		return fmt.Errorf("store: commit vector tx: %w", err)
	}
	return nil
}

// DeleteLibrary implements Store.DeleteLibrary.
func (s *SQLite) DeleteLibrary(ctx context.Context, tenantID, libraryID, version string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin delete tx: %w", err)
	}
	defer tx.Rollback()

	rowids, err := deleteChunksTx(ctx, tx, tenantID, libraryID, version)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit delete tx: %w", err)
	}

	if s.vecDB != nil && len(rowids) > 0 {
		for _, rowid := range rowids {
			if _, err := s.vecDB.ExecContext(ctx, `DELETE FROM vec_chunks WHERE rowid = ?`, rowid); err != nil {
				return fmt.Errorf("store: delete vector: %w", err)
			}
		}
	}
	return nil
}

// deleteChunksTx deletes chunks matching tenant+library (and version, if
// non-empty), returning the rowids deleted so the caller can also clean up
// their corresponding rows in vecDB.
func deleteChunksTx(ctx context.Context, tx *sql.Tx, tenantID, libraryID, version string) ([]int64, error) {
	query := `SELECT rowid FROM chunks WHERE tenant_id = ? AND library_id = ?`
	args := []any{tenantID, libraryID}
	if version != "" {
		query += ` AND version = ?`
		args = append(args, version)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query chunks to delete: %w", err)
	}
	var rowids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store: scan rowid: %w", err)
		}
		rowids = append(rowids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	delQuery := `DELETE FROM chunks WHERE tenant_id = ? AND library_id = ?`
	delArgs := []any{tenantID, libraryID}
	if version != "" {
		delQuery += ` AND version = ?`
		delArgs = append(delArgs, version)
	}
	if _, err := tx.ExecContext(ctx, delQuery, delArgs...); err != nil {
		return nil, fmt.Errorf("store: delete chunks: %w", err)
	}
	return rowids, nil
}
