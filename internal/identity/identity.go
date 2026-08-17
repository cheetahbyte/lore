// Package identity implements lore's typed-prefix source identity scheme,
// decided in https://github.com/cheetahbyte/lore/issues/8.
package identity

import (
	"fmt"
	"strings"
)

type SourceType string

const (
	GitHub    SourceType = "github"
	NPM       SourceType = "npm"
	PyPI      SourceType = "pypi"
	GoPackage SourceType = "pkg.go.dev"
	LLMsTxt   SourceType = "llms-txt"
	URL       SourceType = "url"
)

var knownTypes = map[SourceType]bool{
	GitHub: true, NPM: true, PyPI: true, GoPackage: true, LLMsTxt: true, URL: true,
}

// ID is a canonical typed-prefix identity, e.g. "github:owner/repo" or
// "npm:react". The prefix names the Source adapter that owns it; the
// remainder is that adapter's own reference format.
type ID string

// Parse validates and returns s as an ID. s must contain a known type
// prefix followed by ":" and a non-empty reference.
func Parse(s string) (ID, error) {
	t, ref, found := strings.Cut(s, ":")
	if !found || ref == "" {
		return "", fmt.Errorf("identity: %q is not in typed-prefix form (expected type:ref)", s)
	}
	if !knownTypes[SourceType(t)] {
		return "", fmt.Errorf("identity: unknown source type %q in %q", t, s)
	}
	return ID(s), nil
}

// New builds an ID from a known type and reference without validating the
// reference's own format (that's the owning adapter's job).
func New(t SourceType, ref string) ID {
	return ID(string(t) + ":" + ref)
}

// Type returns the identity's source-type prefix.
func (id ID) Type() SourceType {
	t, _, _ := strings.Cut(string(id), ":")
	return SourceType(t)
}

// Ref returns the identity's reference, i.e. everything after the first ":".
func (id ID) Ref() string {
	_, ref, _ := strings.Cut(string(id), ":")
	return ref
}

func (id ID) String() string { return string(id) }
