// Package retrieval implements the hybrid FTS+vector fusion decided in
// https://github.com/cheetahbyte/lore/issues/7.
package retrieval

import "sort"

// DefaultK is the Reciprocal Rank Fusion constant both open-source
// precedents (docs-mcp-server, contextmine) converged on independently —
// see the research findings linked from issue #7.
const DefaultK = 60.0

// Fuse combines one or more ranked ID lists (best result first in each)
// into a single fused ranking via Reciprocal Rank Fusion, and returns IDs
// ordered best-first. An ID present in multiple rankings accumulates score
// from each. Pass a single ranking to use RRF as a no-op reordering (it
// preserves order in that case).
func Fuse(k float64, rankings ...[]string) []string {
	if k <= 0 {
		k = DefaultK
	}
	scores := make(map[string]float64)
	var order []string
	seen := make(map[string]bool)
	for _, ranking := range rankings {
		for i, id := range ranking {
			if !seen[id] {
				seen[id] = true
				order = append(order, id)
			}
			scores[id] += 1.0 / (k + float64(i+1))
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return scores[order[i]] > scores[order[j]]
	})
	return order
}
