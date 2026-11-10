// Package targets checks that the specs of the targets of tiers 1 and 2
// that need the features of D30 run, and draws them.
package targets

import (
	"bytes"
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
)

// TestTargetSpecsDraw runs each spec of testdata/targets in the strict
// mode, so that a feature that zuzu does not support fails the test. Set
// ZUZU_TARGETS_OUT to a folder to write the SVG of each.
func TestTargetSpecsDraw(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../testdata/targets/*.vg.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no specs in testdata/targets")
	}
	out := os.Getenv("ZUZU_TARGETS_OUT")
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".vg.json")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			chart, _, err := zuzu.Parse(src, zuzu.WithStrict(true))
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			report, err := chart.SVG(context.Background(), &b,
				zuzu.WithLoader(zuzu.FSLoader(os.DirFS("../testdata/corpus"))),
				zuzu.WithLocation(time.UTC),
				zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }))
			if err != nil {
				t.Fatal(err)
			}
			for _, is := range report {
				t.Errorf("issue: %v", is)
			}
			if !bytes.Contains(b.Bytes(), []byte("<path")) {
				t.Error("the chart draws no path")
			}
			if out != "" {
				if err := os.WriteFile(filepath.Join(out, name+".svg"), b.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
