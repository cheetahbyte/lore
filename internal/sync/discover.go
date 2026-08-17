package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Target struct {
	Ref     string
	Source  string
	Version string
}

func (t Target) String() string {
	if t.Version == "" {
		return t.Ref
	}
	return t.Ref + "@" + t.Version
}

func Discover(ctx context.Context, root string) ([]Target, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		root = filepath.Dir(root)
	}
	var targets []Target
	add := func(t Target) { targets = append(targets, t) }

	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Dependencies     map[string]string `json:"dependencies"`
			DevDependencies  map[string]string `json:"devDependencies"`
			PeerDependencies map[string]string `json:"peerDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for name := range pkg.Dependencies {
				add(Target{Ref: "npm:" + name, Source: "npm"})
			}
			for name := range pkg.DevDependencies {
				add(Target{Ref: "npm:" + name, Source: "npm"})
			}
			for name := range pkg.PeerDependencies {
				add(Target{Ref: "npm:" + name, Source: "npm"})
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "package-lock.json")); err == nil {
		var lock struct {
			Packages map[string]struct {
				Version string `json:"version"`
			} `json:"packages"`
		}
		if json.Unmarshal(data, &lock) == nil {
			for path, p := range lock.Packages {
				if !strings.HasPrefix(path, "node_modules/") || strings.Contains(path[len("node_modules/"):], "/node_modules/") || p.Version == "" {
					continue
				}
				add(Target{Ref: "npm:" + strings.TrimPrefix(path, "node_modules/"), Source: "npm", Version: p.Version})
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(strings.TrimSpace(strings.SplitN(line, "//", 2)[0]))
			if len(fields) >= 2 && fields[0] == "require" && !strings.HasPrefix(fields[1], "(") {
				addGo(add, fields)
			}
			if len(fields) >= 2 && fields[0] != "require" && strings.HasPrefix(fields[1], "v") {
				addGo(add, fields)
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(root, "requirements.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			name, version := requirement(line)
			if name != "" {
				add(Target{Ref: "pypi:" + name, Source: "pypi", Version: version})
			}
		}
	}

	seen := map[string]Target{}
	for _, t := range targets {
		if t.Ref == "" {
			continue
		}
		if old, ok := seen[t.Ref]; !ok || old.Version == "" {
			seen[t.Ref] = t
		}
	}
	targets = targets[:0]
	for _, t := range seen {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
	return targets, nil
}

func addGo(add func(Target), fields []string) {
	if len(fields) >= 2 && !strings.HasPrefix(fields[1], "(") && fields[1] != "module" {
		version := ""
		if len(fields) > 2 {
			version = fields[2]
		}
		add(Target{Ref: "pkg.go.dev:" + fields[1], Source: "pkg.go.dev", Version: version})
	}
}

func requirement(line string) (string, string) {
	if line == "" || strings.HasPrefix(line, "-") || strings.ContainsAny(line, "[]; ") {
		return "", ""
	}
	for _, op := range []string{"==", ">=", "~=", "<=", ">", "<"} {
		if i := strings.Index(line, op); i > 0 {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(strings.Fields(line[i+len(op):])[0])
		}
	}
	return line, ""
}
