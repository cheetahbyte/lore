package chunk

import "strings"

import "testing"

func TestSplit_HeadingsAndSections(t *testing.T) {
	md := "# Title\n\nIntro text.\n\n## Installation\n\nRun this.\n\n## Configuration\n\nSet that.\n"
	// A tiny target size forces every section into its own piece, so the
	// heading-stack bookkeeping itself is what's under test here.
	pieces := Split(md, 1)
	if len(pieces) == 0 {
		t.Fatal("expected at least one piece")
	}
	found := map[string]bool{}
	for _, p := range pieces {
		found[p.SectionPath] = true
	}
	if !found["Title > Installation"] {
		t.Errorf("expected a piece under %q, got sections %v", "Title > Installation", found)
	}
	if !found["Title > Configuration"] {
		t.Errorf("expected a piece under %q, got sections %v", "Title > Configuration", found)
	}
}

func TestSplit_NeverSplitsInsideFence(t *testing.T) {
	fence := "```go\n" + strings.Repeat("x = 1\n", 500) + "```"
	md := "# Title\n\n" + fence + "\n"
	pieces := Split(md, DefaultTargetSize)
	for _, p := range pieces {
		if strings.Contains(p.Content, "```") {
			opens := strings.Count(p.Content, "```")
			if opens%2 != 0 {
				t.Errorf("piece contains an unterminated fence: %d backtick-fences", opens)
			}
		}
	}
	// The whole fence must appear intact in exactly one piece.
	var joined int
	for _, p := range pieces {
		if strings.Contains(p.Content, fence) {
			joined++
		}
	}
	if joined != 1 {
		t.Errorf("expected the fence to appear whole in exactly one piece, got %d", joined)
	}
}

func TestSplit_MergesSmallSectionsUpToTarget(t *testing.T) {
	md := "# T\n\n## A\n\nshort a\n\n## B\n\nshort b\n\n## C\n\nshort c\n"
	pieces := Split(md, 1000)
	if len(pieces) != 1 {
		t.Errorf("expected small sections to merge into 1 piece, got %d", len(pieces))
	}
}
