// Package chunk defines lore's chunk data model and the structure-aware
// chunker, decided in https://github.com/cheetahbyte/lore/issues/7 and
// https://github.com/cheetahbyte/lore/issues/8.
package chunk

import "time"

// Chunk is one row of indexed, searchable content.
type Chunk struct {
	ID          string
	TenantID    string // always "local" outside a hosted deployment; see issue #10
	LibraryID   string // typed-prefix identity, e.g. "github:owner/repo"
	Version     string // "" for sources with no version concept
	DocURL      string
	SectionPath string // heading trail, e.g. "Installation > Configuration"
	Ordinal     int    // position within the document, for reassembly
	Content     string
	Embedding   []float32 // nil unless vector search is enabled, per issue #7
	ContentHash string
	FetchedAt   time.Time
}

// Page is one normalized, fetched document ready to be chunked.
type Page struct {
	URL         string
	Path        string // e.g. file path within a repo; "" where not applicable
	Content     string
	ContentType string // "markdown" | "html" | "text"
}
