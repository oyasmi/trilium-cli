// Package testutil holds test helpers. Importing it registers the -update
// flag, so every test package imports it to keep `go test ./... -update` working.
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// Update rewrites golden files instead of comparing against them.
var Update = flag.Bool("update", false, "rewrite testdata/*.golden files")

// Golden compares got with testdata/<name>.golden, or rewrites it with -update.
func Golden(t testing.TB, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *Update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run go test -update): %v", err)
	}
	if string(want) != got {
		t.Errorf("%s mismatch\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}
