package dot

import (
	"slices"
	"testing"
)

// TestReportOrder checks that the issues of the attributes come in the
// order of their names, the same in each run.
func TestReportOrder(t *testing.T) {
	t.Parallel()
	src := `digraph { zz=1 aa=2 mm=3 qq=4
		a [zz=1 aa=2 mm=3 qq=4]
		a -> b [zz=1 aa=2 mm=3 qq=4]
		subgraph cluster_x { zz=1 aa=2 mm=3 qq=4 c }
	}`
	var first []string
	for range 20 {
		_, report, err := Parse(src, false)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, is := range report {
			got = append(got, is.Error())
		}
		if first == nil {
			first = got
			if len(got) == 0 {
				t.Fatal("no issues")
			}
			continue
		}
		if !slices.Equal(got, first) {
			t.Fatalf("the order changed:\n%q\n%q", first, got)
		}
	}
}
