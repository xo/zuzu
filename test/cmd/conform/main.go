// Command conform compares zuzu with the references of vega.js (D17), and
// prints the share of the datasets and the scales that match. Run it in the
// test folder after the oracle has written the references:
//
//	go run ./cmd/conform [-v] [-only text]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/xo/zuzu/test/conform"
)

func main() {
	verbose := flag.Bool("v", false, "print what differs")
	only := flag.String("only", "", "check only the specs whose path holds this text")
	dir := flag.String("testdata", "testdata", "the testdata folder")
	flag.Parse()
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	if err := run(ctx, *dir, *only, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "conform:", err)
		os.Exit(1)
	}
}

// run checks every reference and prints a summary.
func run(ctx context.Context, dir, only string, verbose bool) error {
	s, err := conform.Summarize(ctx, dir, only)
	if err != nil {
		return err
	}
	issues := map[string]int{}
	errCounts := map[string]int{}
	for _, res := range s.Results {
		for _, is := range res.Report {
			issues[is.Message+": "+is.Err.Error()]++
		}
		switch {
		case res.Err != nil:
			errCounts[firstLine(res.Err.Error())]++
			if verbose {
				fmt.Printf("ERR  %s: %v\n", res.Spec, res.Err)
			}
		case !res.OK() && verbose:
			fmt.Printf("DIFF %s\n", res.Spec)
			for _, d := range res.Diffs {
				fmt.Printf("\t%s\n", d)
			}
		}
	}
	fmt.Printf("specs:    %d of %d match (%.1f%%), %d could not run\n", s.SpecsOK, s.Specs, conform.Share(s.SpecsOK, s.Specs), s.Errors)
	fmt.Printf("datasets: %d of %d match (%.1f%%)\n", s.DatasetsOK, s.Datasets, conform.Share(s.DatasetsOK, s.Datasets))
	fmt.Printf("scales:   %d of %d match (%.1f%%)\n", s.ScalesOK, s.Scales, conform.Share(s.ScalesOK, s.Scales))
	fmt.Printf("marks:    %d of %d match (%.1f%%)\n", s.MarksOK, s.Marks, conform.Share(s.MarksOK, s.Marks))
	printTop("errors", errCounts)
	printTop("issues", issues)
	return nil
}

// firstLine returns the first line of a message, without the path.
func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// printTop prints the most common messages.
func printTop(title string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range counts {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n || list[i].n == list[j].n && list[i].k < list[j].k })
	fmt.Printf("%s:\n", title)
	for i, e := range list {
		if i >= 25 {
			break
		}
		fmt.Printf("%5d  %s\n", e.n, e.k)
	}
}
