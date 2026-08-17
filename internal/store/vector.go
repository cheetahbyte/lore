package store

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// encodeVector packs a float32 vector into vec1's native format: a BLOB of
// little-endian 32-bit floats, per https://sqlite.org/vec1/doc/trunk/doc/vec1ref.md.
func encodeVector(v []float32) ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(len(v) * 4)
	if err := binary.Write(buf, binary.LittleEndian, v); err != nil {
		return nil, fmt.Errorf("store: encode vector: %w", err)
	}
	return buf.Bytes(), nil
}
