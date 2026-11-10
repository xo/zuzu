package dataflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// TextSize gives each character a width of 6 and each text a height of 10.
func (e *env) TextSize(text any, _ string, _ float64, _ string) (float64, float64) {
	return 6 * float64(len(expr.String(text))), 10
}

// MarkShapes reads the items of a mark from the dataset of its name. A mark
// whose name starts with line is a line, and any other mark is a rect.
func (e *env) MarkShapes(name string) (string, []Row, bool) {
	rows, ok := e.data[name]
	if !ok {
		return "", nil, false
	}
	if strings.HasPrefix(name, "line") {
		return "line", rows, true
	}
	return "rect", rows, true
}

// labelRow is the row of a text item for the item of a symbol at x, y, 4
// pixels wide.
func labelRow(text string, x, y float64) string {
	return `{"text":"` + text + `","datum":{"bounds":{"x1":` + expr.String(x-2) + `,"y1":` + expr.String(y-2) +
		`,"x2":` + expr.String(x+2) + `,"y2":` + expr.String(y+2) + `}}}`
}

func TestLabel(t *testing.T) {
	t.Parallel()
	project := `{"type":"project","fields":["text","x","y","opacity","align","baseline"]}`
	tests := []struct {
		name, rows, label, want string
	}{
		{"first anchor", `[` + labelRow("ab", 50, 50) + `]`, `"size":[100,100]`,
			// The corner offset of 1 runs along the diagonal.
			`[{"align":"right","baseline":"bottom","opacity":1,"text":"ab","x":47.292893218813454,"y":47.292893218813454}]`},
		// The left anchor of the second text overlaps the first text.
		{"next anchor", `[` + labelRow("ab", 50, 50) + `,` + labelRow("cd", 50, 50) + `]`, `"size":[100,100]`,
			`[{"align":"right","baseline":"bottom","opacity":1,"text":"ab","x":47.292893218813454,"y":47.292893218813454},` +
				`{"align":"right","baseline":"top","opacity":1,"text":"cd","x":47.292893218813454,"y":52.707106781186546}]`},
		{"inside the size", `[` + labelRow("ab", 4, 4) + `]`, `"size":[100,100],"anchor":["top-left","bottom-right"]`,
			`[{"align":"left","baseline":"top","opacity":1,"text":"ab","x":6.707106781186548,"y":6.707106781186548}]`},
		{"padding", `[` + labelRow("ab", 4, 4) + `]`, `"size":[100,100],"anchor":["top-left","bottom-right"],"padding":20`,
			`[{"align":"right","baseline":"bottom","opacity":1,"text":"ab","x":1.2928932188134539,"y":1.2928932188134539}]`},
		{"offsets", `[` + labelRow("ab", 50, 50) + `,` + labelRow("cd", 50, 50) + `]`, `"size":[100,100],"anchor":["top"],"offset":[1,12]`,
			`[{"align":"center","baseline":"bottom","opacity":1,"text":"ab","x":50,"y":47},` +
				`{"align":"center","baseline":"bottom","opacity":1,"text":"cd","x":50,"y":36}]`},
		{"other items", `[` + labelRow("ab", 50, 50) + `,` + labelRow("cd", 50, 40) + `]`, `"size":[100,100],"anchor":["top","bottom"]`,
			// The text above the first item covers the second item.
			`[{"align":"center","baseline":"top","opacity":1,"text":"ab","x":50,"y":53},` +
				`{"align":"center","baseline":"bottom","opacity":1,"text":"cd","x":50,"y":37}]`},
		{"base mark not avoided", `[` + labelRow("ab", 50, 50) + `,` + labelRow("cd", 50, 40) + `]`, `"size":[100,100],"anchor":["top","bottom"],"avoidBaseMark":false`,
			`[{"align":"center","baseline":"bottom","opacity":1,"text":"ab","x":50,"y":47},` +
				`{"align":"center","baseline":"bottom","opacity":1,"text":"cd","x":50,"y":37}]`},
		{"avoid a line", `[` + labelRow("ab", 50, 50) + `]`, `"size":[100,100],"anchor":["top","bottom"],"avoidMarks":["line"]`,
			`[{"align":"center","baseline":"top","opacity":1,"text":"ab","x":50,"y":53}]`},
		{"avoid a rect", `[` + labelRow("ab", 50, 50) + `]`, `"size":[100,100],"anchor":["top","bottom"],"avoidMarks":["box"]`,
			`[{"align":"center","baseline":"top","opacity":1,"text":"ab","x":50,"y":53}]`},
		{"middle", `[{"text":"ab","datum":{"bounds":{"x1":10,"y1":10,"x2":40,"y2":30}}}]`, `"size":[100,100],"anchor":["middle"]`,
			`[{"align":"center","baseline":"middle","opacity":1,"text":"ab","x":25,"y":20}]`},
		{"no place", `[` + labelRow("abcdefghijklmnopqrstuvwxyz", 50, 50) + `]`, `"size":[100,100]`,
			`[{"align":{},"baseline":{},"opacity":0,"text":"abcdefghijklmnopqrstuvwxyz","x":{},"y":{}}]`},
		{"sort", `[{"p":1,` + labelRow("ab", 50, 50)[1:] + `,{"p":0,` + labelRow("cd", 50, 50)[1:] + `]`, `"size":[100,100],"anchor":["top","bottom"],"sort":{"field":"p"}`,
			`[{"align":"center","baseline":"top","opacity":1,"text":"ab","x":50,"y":53},` +
				`{"align":"center","baseline":"bottom","opacity":1,"text":"cd","x":50,"y":47}]`},
		{"as", `[` + labelRow("ab", 50, 50) + `]`, `"size":[100,100],"anchor":["top"],"as":["x","y","opacity","align","baseline"]`,
			`[{"align":"center","baseline":"bottom","opacity":1,"text":"ab","x":50,"y":47}]`},
	}
	for _, test := range tests {
		e := newEnv()
		// The line runs across the place of a label above the item at 50,
		// 50, and the rect covers it.
		e.data["line"] = rowsOf(t, `[{"x":0,"y":44},{"x":100,"y":44}]`)
		e.data["box"] = rowsOf(t, `[{"bounds":{"x1":40,"y1":40,"x2":60,"y2":45}}]`)
		ts := `[{"type":"label",` + test.label + `},` + project + `]`
		if got := run(t, e, test.rows, ts); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", test.name, got, test.want)
		}
	}
}

func TestLabelErrors(t *testing.T) {
	t.Parallel()
	rows := rowsOf(t, `[`+labelRow("ab", 50, 50)+`]`)
	for _, test := range []struct {
		ts          string
		unsupported bool
	}{
		{`[{"type":"label"}]`, false},
		{`[{"type":"label","size":[10,10],"anchor":["up"]}]`, false},
		{`[{"type":"label","size":[10,10],"avoidMarks":["none"]}]`, false},
		{`[{"type":"label","size":[10,10],"method":"floodfill"}]`, true},
		{`[{"type":"label","size":[10,10],"lineAnchor":"start"}]`, true},
		{`[{"type":"label","size":[10,10],"markIndex":1}]`, true},
	} {
		_, err := Run(newEnv(), rows, transformsOf(t, test.ts), "/transform")
		if err == nil {
			t.Errorf("%s gives no error", test.ts)
			continue
		}
		if _, ok := errors.AsType[vega.Issue](err); ok != test.unsupported {
			t.Errorf("%s: the error %v is an issue: %t", test.ts, err, ok)
		}
	}
}
