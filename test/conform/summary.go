package conform

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Summary counts what matched over many references.
type Summary struct {
	Specs, SpecsOK       int
	Errors               int
	Datasets, DatasetsOK int
	Scales, ScalesOK     int
	Marks, MarksOK       int
	// Results holds the result of each spec, in the order of their paths.
	Results []Result
}

// Share returns n of all as a percentage.
func Share(n, all int) float64 {
	if all == 0 {
		return 0
	}
	return 100 * float64(n) / float64(all)
}

// Summarize checks each reference in the folder testdata/references whose
// path holds only, and counts what matched. It leaves out the reference of
// a spec that failed in vega.js.
func Summarize(ctx context.Context, testdata, only string) (*Summary, error) {
	paths, err := filepath.Glob(filepath.Join(testdata, "references", "*", "*.ref.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no references in %s: run the oracle first: %w", testdata, ErrNoReferences)
	}
	sort.Strings(paths)
	s := &Summary{}
	for _, p := range paths {
		if only != "" && !strings.Contains(p, only) {
			continue
		}
		ref, err := Read(p)
		if err != nil {
			return nil, err
		}
		if ref.Error != "" {
			continue
		}
		res := Check(ctx, ref, filepath.Join(testdata, "corpus"))
		s.Results = append(s.Results, res)
		s.Specs++
		s.Datasets += res.Datasets
		s.DatasetsOK += res.DatasetsOK
		s.Scales += res.Scales
		s.ScalesOK += res.ScalesOK
		s.Marks += res.Marks
		s.MarksOK += res.MarksOK
		switch {
		case res.Err != nil:
			s.Errors++
		case res.OK():
			s.SpecsOK++
		}
	}
	return s, nil
}

// Error is an error of the package conform.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// ErrNoReferences is the error of a testdata folder that holds no
// references, because the oracle has not run.
const ErrNoReferences Error = "no references"
