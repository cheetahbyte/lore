package retrieval

import (
	"reflect"
	"slices"
	"testing"
)

func TestFuse_SingleRankingPreservesOrder(t *testing.T) {
	got := Fuse(DefaultK, []string{"a", "b", "c"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fuse() = %v, want %v", got, want)
	}
}

func TestFuse_BoostsItemsRankedInBoth(t *testing.T) {
	fts := []string{"a", "b", "c"}
	vec := []string{"c", "a", "d"}
	got := Fuse(DefaultK, fts, vec)
	if got[0] != "a" {
		t.Errorf("expected %q (ranked highly in both lists) first, got %v", "a", got)
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if !slices.Contains(got, id) {
			t.Errorf("expected %q in fused result %v", id, got)
		}
	}
}
