// SPDX-License-Identifier: EPL-2.0

//go:build amd64 && !amd64.v3

package adapter

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write testdata/golden.txt again")

// TestGoldenLayouts lays out each fixture and compares the places with
// testdata/golden.txt, byte for byte. The file holds the bytes of amd64
// below the level v3, so the test runs only there (D41, docs/UPSTREAM.md).
// To write the file again, run go test ./internal/adapter -run
// TestGoldenLayouts -update on such a machine.
func TestGoldenLayouts(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	for _, f := range fixtures() {
		r, err := Layout(&f.g)
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		sb.WriteString(format(f.name, r, &f.g))
		sb.WriteString("\n")
	}
	path := filepath.Join("testdata", "golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(sb.String(), "\n")
	lines := strings.Split(string(want), "\n")
	for i := range max(len(got), len(lines)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(lines) {
			w = lines[i]
		}
		if g != w {
			t.Fatalf("testdata/golden.txt:%d:\n got %q\nwant %q", i+1, g, w)
		}
	}
}
