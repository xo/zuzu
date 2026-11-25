package vegalite_test

import (
	"fmt"
	"log"

	"github.com/xo/zuzu/vegalite"
)

func ExampleCompile() {
	spec, err := vegalite.Parse([]byte(`{
  "$schema": "https://vega.github.io/schema/vega-lite/v6.json",
  "data": {"values": [{"a": "x", "b": 1}, {"a": "y", "b": 2}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "a", "type": "nominal"},
    "y": {"field": "b", "type": "quantitative"}
  }
}`))
	if err != nil {
		log.Fatal(err)
	}
	vg, report, err := vegalite.Compile(spec)
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range vg.Scales {
		fmt.Println(s.Name, s.Type)
	}
	fmt.Println(vg.Marks[0].Type, len(report))
	// Output:
	// x band
	// y linear
	// rect 0
}
