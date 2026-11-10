package conform

import (
	"errors"
	"testing"
)

// The exit of phase 3, as docs/PLAN.md states it, in percent.
const (
	minSpecs    = 80
	minDatasets = 85
	minScales   = 90
	minMarks    = 90
)

// TestPhase3Share checks that the share of the specs, the datasets, the
// scales and the marks that match the references stays at or above the
// exit of phase 3. It needs the references, which the oracle writes, so it
// skips without them. The conformance job of CI runs the oracle first.
func TestPhase3Share(t *testing.T) {
	t.Parallel()
	s, err := Summarize(t.Context(), "../testdata", "")
	if errors.Is(err, ErrNoReferences) {
		t.Skip("no references: run the oracle, as test/README.md says")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		n, all  int
		minimum float64
	}{
		{"specs", s.SpecsOK, s.Specs, minSpecs},
		{"datasets", s.DatasetsOK, s.Datasets, minDatasets},
		{"scales", s.ScalesOK, s.Scales, minScales},
		{"marks", s.MarksOK, s.Marks, minMarks},
	} {
		share := Share(c.n, c.all)
		t.Logf("%s: %d of %d match (%.1f%%)", c.name, c.n, c.all, share)
		if share < c.minimum {
			t.Errorf("%s: %.1f%% match, below the %v%% of phase 3", c.name, share, c.minimum)
		}
	}
	if s.Errors > 0 {
		t.Errorf("%d specs did not run", s.Errors)
	}
}
