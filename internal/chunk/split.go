package chunk

import "strings"

// DefaultTargetSize is the merge target used by Split when the caller
// doesn't override it, per the char-based sizing decided in issue #7.
const DefaultTargetSize = 1500

// Piece is one structure-aware chunk of markdown, before the caller fills
// in the rest of the Chunk fields (library id, version, tenant, etc).
type Piece struct {
	SectionPath string
	Content     string
}

// Split performs the chunking strategy decided in issue #7: split first at
// natural boundaries (headings, code fences, paragraph breaks), then
// greedily merge adjacent pieces up to targetSize — never splitting inside
// a fenced code block, even if the fence itself exceeds targetSize.
func Split(markdown string, targetSize int) []Piece {
	if targetSize <= 0 {
		targetSize = DefaultTargetSize
	}
	return mergeBlocks(splitBlocks(markdown), targetSize)
}

type block struct {
	sectionPath string
	content     string
	isFence     bool
}

func splitBlocks(markdown string) []block {
	lines := strings.Split(markdown, "\n")
	var blocks []block
	var headingStack []string
	var buf []string
	inFence := false
	var fenceMarker string

	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		if text != "" {
			blocks = append(blocks, block{sectionPath: strings.Join(headingStack, " > "), content: text})
		}
		buf = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inFence {
			buf = append(buf, line)
			if strings.HasPrefix(trimmed, fenceMarker) {
				inFence = false
				blocks = append(blocks, block{
					sectionPath: strings.Join(headingStack, " > "),
					content:     strings.Join(buf, "\n"),
					isFence:     true,
				})
				buf = nil
			}
			continue
		}

		if marker, ok := fenceStart(trimmed); ok {
			flush()
			inFence = true
			fenceMarker = marker
			buf = append(buf, line)
			continue
		}

		if level, title, ok := parseHeading(trimmed); ok {
			flush()
			if level-1 < len(headingStack) {
				headingStack = headingStack[:level-1]
			}
			for len(headingStack) < level-1 {
				headingStack = append(headingStack, "")
			}
			headingStack = append(headingStack, title)
			continue
		}

		if trimmed == "" {
			flush()
			continue
		}

		buf = append(buf, line)
	}
	// Unterminated fence at EOF: flush whatever was collected as-is rather
	// than losing it.
	flush()
	return blocks
}

func fenceStart(trimmed string) (marker string, ok bool) {
	if strings.HasPrefix(trimmed, "```") {
		return "```", true
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~", true
	}
	return "", false
}

func parseHeading(line string) (level int, title string, ok bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i > 6 || i >= len(line) || line[i] != ' ' {
		return 0, "", false
	}
	return i, strings.TrimSpace(line[i:]), true
}

func mergeBlocks(blocks []block, targetSize int) []Piece {
	var pieces []Piece
	var cur strings.Builder
	var curSection string

	flush := func() {
		if cur.Len() > 0 {
			pieces = append(pieces, Piece{SectionPath: curSection, Content: strings.TrimSpace(cur.String())})
			cur.Reset()
		}
	}

	for _, b := range blocks {
		wouldOverflow := cur.Len() > 0 && cur.Len()+len(b.content)+2 > targetSize
		if wouldOverflow {
			flush()
		}
		if cur.Len() == 0 {
			curSection = b.sectionPath
		} else {
			cur.WriteString("\n\n")
		}
		cur.WriteString(b.content)
	}
	flush()
	return pieces
}
