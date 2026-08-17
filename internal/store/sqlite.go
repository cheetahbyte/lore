package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
	"github.com/ncruces/go-sqlite3/ext/vec1"
)

// SQLite is the Store implementation decided in issue #7/#10:
// ncruces/go-sqlite3 (pure Go, no CGO), FTS5 always on.
//
// FTS5 and vec1 cannot both be registered as dynamically-linked WASM
// extensions on the *same* go-sqlite3 connection (confirmed while
// implementing this: only the first ext/*.Register call on a connection
// actually takes effect, the second silently fails to register its
// virtual table module — an upstream limitation, not a design choice).
// So vector search, when enabled, lives in a second SQLite file with only
// vec1 registered, rather than one database with both. Chunk rowids are
// shared by construction (the vector row is inserted with the same rowid
// SQLite assigned the chunk row) so the two stay joinable in application
// code even though they're not literally the same file.
type SQLite struct {
	db    *sql.DB // index.db: chunks + FTS5
	vecDB *sql.DB // vectors.db: vec1; nil when vector search is disabled
}

// Open opens (creating if needed) the SQLite-backed Store at dataDir.
// vectorEnabled mirrors whether embeddings are configured, per issue #7 —
// the vectors.db file and its vec1 registration are skipped entirely when
// vector search is off, keeping the FTS-only default path dependency-free.
func Open(ctx context.Context, indexPath, vectorPath string, vectorEnabled bool) (*SQLite, error) {
	db, err := driver.Open("file:"+indexPath, fts5.Register)
	if err != nil {
		return nil, fmt.Errorf("store: open index db: %w", err)
	}
	if err := applyIndexSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	s := &SQLite{db: db}

	if vectorEnabled {
		vecDB, err := driver.Open("file:"+vectorPath, vec1.Register)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("store: open vector db: %w", err)
		}
		if err := applyVectorSchema(ctx, vecDB); err != nil {
			db.Close()
			vecDB.Close()
			return nil, err
		}
		s.vecDB = vecDB
	}

	return s, nil
}

func applyIndexSchema(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS chunks (
	tenant_id    TEXT NOT NULL,
	library_id   TEXT NOT NULL,
	version      TEXT NOT NULL DEFAULT '',
	doc_url      TEXT NOT NULL,
	section_path TEXT NOT NULL DEFAULT '',
	ordinal      INTEGER NOT NULL,
	content      TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	fetched_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chunks_library
	ON chunks(tenant_id, library_id, version);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
	content, section_path,
	content='chunks', content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
	INSERT INTO chunks_fts(rowid, content, section_path)
	VALUES (new.rowid, new.content, new.section_path);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, content, section_path)
	VALUES ('delete', old.rowid, old.content, old.section_path);
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, content, section_path)
	VALUES ('delete', old.rowid, old.content, old.section_path);
	INSERT INTO chunks_fts(rowid, content, section_path)
	VALUES (new.rowid, new.content, new.section_path);
END;
`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("store: apply index schema: %w", err)
	}
	return nil
}

func applyVectorSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks USING vec1(embedding)`); err != nil {
		return fmt.Errorf("store: create vec_chunks: %w", err)
	}
	// Cosine distance suits normalized text-embedding vectors (OpenAI,
	// Voyage, Ollama-served models) better than vec1's default L2. index:
	// "none" (one row per vector, no compression) is the only option that
	// doesn't require a trained model via vec1_train first.
	if _, err := db.ExecContext(ctx, `INSERT INTO vec_chunks(cmd, embedding) VALUES ('rebuild', '{"distance":"cos","index":"none"}')`); err != nil {
		return fmt.Errorf("store: configure vec_chunks distance metric: %w", err)
	}
	return nil
}

func (s *SQLite) Close() error {
	err := s.db.Close()
	if s.vecDB != nil {
		if verr := s.vecDB.Close(); verr != nil && err == nil {
			err = verr
		}
	}
	return err
}
