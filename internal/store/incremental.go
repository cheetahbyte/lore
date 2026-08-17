package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cheetahbyte/lore/internal/chunk"
)

func (s *SQLite) UpsertDocumentChunks(ctx context.Context, tenantID, libraryID, version, docURL string, chunks []chunk.Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin document upsert: %w", err)
	}
	defer tx.Rollback()
	rowids, err := deleteDocumentTx(ctx, tx, tenantID, libraryID, version, docURL)
	if err != nil {
		return err
	}
	insert, err := tx.PrepareContext(ctx, `INSERT INTO chunks (tenant_id, library_id, version, doc_url, section_path, ordinal, content, content_hash, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING rowid`)
	if err != nil {
		return fmt.Errorf("store: prepare document insert: %w", err)
	}
	defer insert.Close()
	var vecRows []struct {
		rowid  int64
		vector []float32
	}
	for _, c := range chunks {
		var rowid int64
		if err := insert.QueryRowContext(ctx, tenantID, libraryID, version, docURL, c.SectionPath, c.Ordinal, c.Content, c.ContentHash, c.FetchedAt.Unix()).Scan(&rowid); err != nil {
			return fmt.Errorf("store: insert document chunk: %w", err)
		}
		if c.Embedding != nil {
			vecRows = append(vecRows, struct {
				rowid  int64
				vector []float32
			}{rowid, c.Embedding})
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit document upsert: %w", err)
	}
	if s.vecDB == nil {
		return nil
	}
	vtx, err := s.vecDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin document vectors: %w", err)
	}
	defer vtx.Rollback()
	for _, rowid := range rowids {
		if _, err := vtx.ExecContext(ctx, `DELETE FROM vec_chunks WHERE rowid = ?`, rowid); err != nil {
			return err
		}
	}
	for _, row := range vecRows {
		blob, err := encodeVector(row.vector)
		if err != nil {
			return err
		}
		if _, err := vtx.ExecContext(ctx, `INSERT INTO vec_chunks(rowid, embedding) VALUES (?, ?)`, row.rowid, blob); err != nil {
			return err
		}
	}
	return vtx.Commit()
}

func (s *SQLite) DeleteDocument(ctx context.Context, tenantID, libraryID, version, docURL string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rowids, err := deleteDocumentTx(ctx, tx, tenantID, libraryID, version, docURL)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.vecDB != nil {
		for _, rowid := range rowids {
			if _, err := s.vecDB.ExecContext(ctx, `DELETE FROM vec_chunks WHERE rowid = ?`, rowid); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteDocumentTx(ctx context.Context, tx *sql.Tx, tenantID, libraryID, version, docURL string) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT rowid FROM chunks WHERE tenant_id = ? AND library_id = ? AND version = ? AND doc_url = ?`, tenantID, libraryID, version, docURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rowids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		rowids = append(rowids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE tenant_id = ? AND library_id = ? AND version = ? AND doc_url = ?`, tenantID, libraryID, version, docURL); err != nil {
		return nil, err
	}
	return rowids, nil
}
