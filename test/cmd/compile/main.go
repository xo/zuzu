// Command compile prints the Vega spec that zuzu compiles from a Vega-Lite
// spec, with the issues of the compile on standard error. Run it in the
// test folder:
//
//	go run ./cmd/compile testdata/corpus/vegalite/bar.vl.json
package main

import (
	"encoding/json/v2"
	"fmt"
	"os"

	"github.com/xo/zuzu/vega"
	"github.com/xo/zuzu/vegalite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: compile spec.vl.json")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "compile:", err)
		os.Exit(1)
	}
}

// run compiles a spec and prints the result.
func run(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s, err := vegalite.Parse(src)
	if err != nil {
		return err
	}
	out, report, err := vegalite.Compile(s)
	for _, is := range report {
		fmt.Fprintln(os.Stderr, "issue:", is.Error())
	}
	if err != nil {
		return err
	}
	b, err := vega.Write(out)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	b, err = json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}
