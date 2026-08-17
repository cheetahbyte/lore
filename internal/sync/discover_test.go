package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverPackageLockAndGoMod(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"dependencies":{"react":"^19"}}`)
	write("package-lock.json", `{"packages":{"":{"name":"x"},"node_modules/react":{"version":"19.1.0"}}}`)
	write("go.mod", "module example\n\nrequire example.com/foo v1.2.3\n")
	targets, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, target := range targets {
		joined += target.String() + "\n"
	}
	for _, want := range []string{"npm:react@19.1.0", "pkg.go.dev:example.com/foo@v1.2.3"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
}
