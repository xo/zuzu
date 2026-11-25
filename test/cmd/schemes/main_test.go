package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// root is the folder of the root module of zuzu.
var root = filepath.Join("..", "..", "..")

// TestTheGeneratedFilesAreInStep runs the command into a temporary folder
// and checks that each file that it writes is the same in the repository,
// and that theme/licenses holds no other file. If it fails, run the
// command again, as docs/STYLE.md says.
func TestTheGeneratedFilesAreInStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := run(t.Context(), dir); err != nil {
		t.Fatal(err)
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range files {
		want, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("%s: %v. Run go run ./cmd/schemes in the test module", rel, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from what the command writes. Run go run ./cmd/schemes in the test module", rel)
		}
	}
	err = filepath.WalkDir(filepath.Join(root, "theme", "licenses"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("%s is not a file that the command writes", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTheLimitsMatchTheStyle checks the limits of the colors of a scheme
// from a style against their table in docs/STYLE.md.
func TestTheLimitsMatchTheStyle(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(root, "docs", "STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile("(?m)^\\| `(upstreamChroma|upstreamContrast|upstreamDistance|upstreamHueStep|riseColor|fallColor)` \\| `?([0-9.]+|#[0-9a-f]{6})`? \\|").
		FindAllStringSubmatch(string(b), -1)
	got := map[string]string{
		"upstreamChroma": fmt.Sprint(upstreamChroma), "upstreamContrast": fmt.Sprint(upstreamContrast),
		"upstreamDistance": fmt.Sprint(upstreamDistance), "upstreamHueStep": fmt.Sprint(upstreamHueStep),
		"riseColor": riseColor, "fallColor": fallColor,
	}
	if len(rows) != len(got) {
		t.Fatalf("docs/STYLE.md has %d rows of these limits, expected %d", len(rows), len(got))
	}
	for _, row := range rows {
		if got[row[1]] != row[2] {
			t.Errorf("%s: the command has %s and docs/STYLE.md has %s", row[1], got[row[1]], row[2])
		}
	}
}
