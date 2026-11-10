// Package conform compares what zuzu works out for a spec with the
// references of vega.js (D17): the rows of each dataset, and the type and
// the domain of each scale. It compares meaning, not look, so it never
// compares a position or a color.
package conform

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
	"github.com/xo/zuzu/vegalite"
)

// Reference is a reference that the oracle wrote.
type Reference struct {
	Spec     string                    `json:"spec"`
	Random   bool                      `json:"random"`
	Compiled jsontext.Value            `json:"compiled"`
	Datasets map[string]jsontext.Value `json:"datasets"`
	Scales   map[string]struct {
		Type   string         `json:"type"`
		Domain jsontext.Value `json:"domain"`
	} `json:"scales"`
	Marks []MarkRef `json:"marks"`
	Error string    `json:"error"`
}

// MarkRef is one mark of the scene graph of vega.js, with the data of its
// items. A mark of an axis or a legend has no data.
type MarkRef struct {
	Path     string           `json:"path"`
	MarkType string           `json:"marktype"`
	Role     string           `json:"role"`
	Name     string           `json:"name"`
	Count    int              `json:"count"`
	Data     []jsontext.Value `json:"data"`
}

// Result is what a comparison found for one spec.
type Result struct {
	Spec string
	// Err is the error of zuzu, if it did not run the spec.
	Err error
	// Report holds the issues of the loose mode.
	Report zuzu.Report
	// Datasets and Scales count the datasets and the scales that the
	// reference holds, and the OK fields those that match.
	Datasets, DatasetsOK int
	Scales, ScalesOK     int
	// Marks counts the marks that draw data, which zuzu must bind to the
	// same items. The marks of the guides are left out, since zuzu draws
	// its own guides (D12).
	Marks, MarksOK int
	// Structure says that zuzu compiled a Vega-Lite spec to the same
	// datasets, scales and marks as vega-lite.js, and Same that it
	// compiled the same JSON.
	Structure, Same bool
	// Diffs says what differs.
	Diffs []string
}

// OK reports whether everything matched.
func (r Result) OK() bool {
	return r.Err == nil && r.Datasets == r.DatasetsOK && r.Scales == r.ScalesOK && r.Marks == r.MarksOK
}

// Read reads a reference.
func Read(path string) (*Reference, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ref Reference
	if err := json.Unmarshal(b, &ref); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &ref, nil
}

// Check runs the spec of a reference in zuzu and compares the result. The
// spec of a Vega-Lite reference is the Vega spec that vega-lite.js
// compiled, unless compile is true, when zuzu compiles the Vega-Lite spec
// itself. corpus is the folder of the corpus, which the loader reads.
func Check(ctx context.Context, ref *Reference, corpus string, compile bool) Result {
	res := Result{Spec: ref.Spec, Datasets: len(ref.Datasets), Scales: len(ref.Scales)}
	src := []byte(ref.Compiled)
	dialect := zuzu.Vega
	if compile && len(src) > 0 {
		b, err := os.ReadFile(filepath.Join(corpus, ref.Spec))
		if err != nil {
			res.Err = err
			return res
		}
		src, dialect = b, zuzu.VegaLite
		res.Structure = compareStructure(&res, src, ref.Compiled)
	}
	if len(src) == 0 {
		b, err := os.ReadFile(filepath.Join(corpus, ref.Spec))
		if err != nil {
			res.Err = err
			return res
		}
		src = b
	}
	// vega.js answers the name of a dataset that a group also defines
	// with the dataset of the last such group, so the reference holds the
	// data of one cell. The check leaves those names out.
	vegaSrc := []byte(ref.Compiled)
	if len(vegaSrc) == 0 && dialect == zuzu.Vega {
		vegaSrc = src
	}
	shadowed := shadowedData(vegaSrc)
	for name := range shadowed {
		if _, ok := ref.Datasets[name]; ok {
			res.Datasets--
		}
	}
	chart, _, err := zuzu.Parse(src, zuzu.WithDialect(dialect))
	if err != nil {
		res.Err = err
		return res
	}
	view, report, err := chart.NewView(ctx,
		zuzu.WithLoader(zuzu.FSLoader(os.DirFS(corpus))),
		zuzu.WithLocation(time.UTC),
		zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }),
	)
	res.Report = report
	if err != nil {
		res.Err = err
		return res
	}
	names := make([]string, 0, len(ref.Datasets))
	for name := range ref.Datasets {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if shadowed[name] {
			continue
		}
		want := ref.Datasets[name]
		rows, err := view.Data(name)
		if err != nil {
			res.Diffs = append(res.Diffs, fmt.Sprintf("data %s: %v", name, err))
			continue
		}
		if ref.Random {
			// The rows differ on each run, so only their number counts.
			var w []any
			_ = json.Unmarshal(want, &w)
			if len(w) == len(rows) {
				res.DatasetsOK++
			} else {
				res.Diffs = append(res.Diffs, fmt.Sprintf("data %s: %d rows, want %d", name, len(rows), len(w)))
			}
			continue
		}
		if d := compareRows(rows, want); d != "" {
			res.Diffs = append(res.Diffs, fmt.Sprintf("data %s: %s", name, d))
			continue
		}
		res.DatasetsOK++
	}
	snames := make([]string, 0, len(ref.Scales))
	for name := range ref.Scales {
		snames = append(snames, name)
	}
	slices.Sort(snames)
	for _, name := range snames {
		want := ref.Scales[name]
		s, err := view.Scale(name)
		if err != nil {
			res.Diffs = append(res.Diffs, fmt.Sprintf("scale %s: %v", name, err))
			continue
		}
		if d := compareScale(s, want.Type, want.Domain); d != "" {
			res.Diffs = append(res.Diffs, fmt.Sprintf("scale %s: %s", name, d))
			continue
		}
		res.ScalesOK++
	}
	compareMarks(&res, ref, view.Scene())
	return res
}

// guide reports whether a role belongs to a guide: an axis, a legend or a
// title.
func guide(role string) bool {
	for _, p := range []string{"axis", "legend", "title"} {
		if strings.HasPrefix(role, p) {
			return true
		}
	}
	return false
}

// markEntry is a mark with the data of its items.
type markEntry struct {
	label string
	data  []string
}

// sceneMarks walks a scene graph in the order of the oracle and returns the
// marks that draw data, without those of the guides.
func sceneMarks(m *scene.Mark, out []markEntry) []markEntry {
	if guide(m.Role) {
		return out
	}
	e := markEntry{label: fmt.Sprintf("%s %s %s", m.Type, m.Role, m.Name)}
	for i := range m.Items {
		e.data = append(e.data, canonical(plain(m.Items[i].Datum, 1, map[uintptr]bool{})))
	}
	out = append(out, e)
	for i := range m.Items {
		for j := range m.Items[i].Items {
			out = sceneMarks(&m.Items[i].Items[j], out)
		}
	}
	return out
}

// compareMarks compares the marks that draw data with the marks of the
// reference, in order: their type, role and name, and the data of their
// items as a multiset.
func compareMarks(res *Result, ref *Reference, s *scene.Scene) {
	var want []markEntry
	inGuide := ""
	for _, m := range ref.Marks {
		if inGuide != "" && strings.HasPrefix(m.Path, inGuide+"/") {
			continue
		}
		inGuide = ""
		if guide(m.Role) {
			inGuide = m.Path
			continue
		}
		e := markEntry{label: fmt.Sprintf("%s %s %s", m.MarkType, m.Role, m.Name)}
		for _, d := range m.Data {
			var v any
			_ = json.Unmarshal(d, &v)
			e.data = append(e.data, canonical(v))
		}
		want = append(want, e)
	}
	got := sceneMarks(&s.Root, nil)
	res.Marks = len(want)
	for i, w := range want {
		if i >= len(got) {
			res.Diffs = append(res.Diffs, fmt.Sprintf("mark %d %s: missing", i, w.label))
			continue
		}
		g := got[i]
		if g.label != w.label {
			res.Diffs = append(res.Diffs, fmt.Sprintf("mark %d: %s, want %s", i, g.label, w.label))
			continue
		}
		if len(g.data) != len(w.data) {
			res.Diffs = append(res.Diffs, fmt.Sprintf("mark %d %s: %d items, want %d%s", i, w.label, len(g.data), len(w.data), firstMissing(g.data, w.data)))
			continue
		}
		if len(w.data) > 0 && strings.Contains(w.data[0], `"bounds":`) && strings.Contains(w.data[0], `"mark":`) {
			// The data of the mark is the items of another mark, which hold
			// positions that zuzu does not share (D12). The count matches.
			res.MarksOK++
			continue
		}
		sg, sw := slices.Clone(g.data), slices.Clone(w.data)
		slices.Sort(sg)
		slices.Sort(sw)
		if !slices.Equal(sg, sw) {
			res.Diffs = append(res.Diffs, fmt.Sprintf("mark %d %s: items differ%s", i, w.label, firstMissing(g.data, w.data)))
			continue
		}
		res.MarksOK++
	}
	if len(got) > len(want) {
		res.Diffs = append(res.Diffs, fmt.Sprintf("%d marks, want %d", len(got), len(want)))
	}
}

// compareRows compares rows with the rows of a reference, as multisets.
func compareRows(rows []map[string]any, want jsontext.Value) string {
	var w []any
	if err := json.Unmarshal(want, &w); err != nil {
		var o map[string]any
		if json.Unmarshal(want, &o) == nil && o["error"] != nil {
			return fmt.Sprintf("the reference has an error: %v", o["error"])
		}
		return err.Error()
	}
	got := make([]string, len(rows))
	for i, r := range rows {
		got[i] = canonical(plain(r, 1, map[uintptr]bool{}))
	}
	exp := make([]string, len(w))
	for i, r := range w {
		exp[i] = canonical(r)
	}
	if len(got) != len(exp) {
		return fmt.Sprintf("%d rows, want %d%s", len(got), len(exp), firstMissing(got, exp))
	}
	sg, se := slices.Clone(got), slices.Clone(exp)
	slices.Sort(sg)
	slices.Sort(se)
	for i := range sg {
		if sg[i] != se[i] {
			return "rows differ" + firstMissing(got, exp)
		}
	}
	return ""
}

// firstMissing shows a row that one side has and the other does not.
func firstMissing(got, exp []string) string {
	count := map[string]int{}
	for _, g := range got {
		count[g]++
	}
	for _, e := range exp {
		if count[e] > 0 {
			count[e]--
			continue
		}
		var near string
		for _, g := range got {
			if count[g] > 0 {
				near = g
				break
			}
		}
		return fmt.Sprintf(":\n\t\twant %s\n\t\tgot  %s", clip(e), clip(near))
	}
	return ""
}

// clip shortens a long row.
func clip(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// compareScale compares the type and the domain of a scale.
func compareScale(s scale.Scale, typ string, domain jsontext.Value) string {
	if s.Type() != typ {
		return fmt.Sprintf("type %s, want %s", s.Type(), typ)
	}
	var w any
	if err := json.Unmarshal(domain, &w); err != nil {
		return err.Error()
	}
	got := canonical(plain(s.Domain(), 1, map[uintptr]bool{}))
	want := canonical(w)
	if got != want {
		return fmt.Sprintf("domain %s, want %s", clip(got), clip(want))
	}
	return ""
}

// plain turns a value of zuzu into the JSON that the oracle writes: a date
// is an ISO string in UTC, a number that is not finite is a string, and
// undefined is null. As the oracle does, an array below the second level
// becomes its length, an object below the fourth becomes "[deep]", and an
// array or an object that a row already holds becomes "[ref]".
func plain(v any, depth int, seen map[uintptr]bool) any {
	if p := pointer(v); p != 0 {
		if seen[p] {
			return "[ref]"
		}
	}
	switch x := v.(type) {
	case time.Time:
		if x.IsZero() {
			return nil
		}
		return x.UTC().Format("2006-01-02T15:04:05.000Z")
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return expr.String(x)
		}
		return x
	case []any:
		if depth > 2 {
			return map[string]any{"length": float64(len(x))}
		}
		if p := pointer(v); p != 0 {
			seen[p] = true
		}
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = plain(e, depth+1, seen)
		}
		return out
	case map[string]any:
		if depth > 4 {
			return "[deep]"
		}
		if p := pointer(v); p != 0 {
			seen[p] = true
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = plain(x[k], depth+1, seen)
		}
		return out
	}
	if expr.IsUndefined(v) {
		return nil
	}
	return v
}

// pointer returns the address of the elements of a non-empty array or of
// an object, to find one that a row holds twice, or 0.
func pointer(v any) uintptr {
	switch x := v.(type) {
	case []any:
		if len(x) > 0 {
			return reflect.ValueOf(x).Pointer()
		}
	case map[string]any:
		return reflect.ValueOf(x).Pointer()
	}
	return 0
}

// canonical writes a value as JSON with sorted keys, and numbers with 8
// significant digits, so that two sums in a different order match. A
// number nearer zero than 1e-9 is zero.
// pathString matches the path of a link or a shape, and pathNumber each
// number in it.
var (
	pathString = regexp.MustCompile(`^M[-0-9.eE,]+[MLHVCSQTAZmlhvcsqtaz][-0-9.eE, MLHVCSQTAZmlhvcsqtaz]*$`)
	pathNumber = regexp.MustCompile(`-?[0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?`)
)

func canonical(v any) string {
	var b strings.Builder
	write(&b, v)
	return b.String()
}

func write(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case float64:
		if math.Abs(x) < 1e-9 {
			x = 0
		}
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
			return
		}
		b.WriteString(strconv.FormatFloat(x, 'g', 8, 64))
	case string:
		if pathString.MatchString(x) {
			// The numbers of an SVG path compare as numbers do.
			x = pathNumber.ReplaceAllStringFunc(x, func(n string) string {
				f, err := strconv.ParseFloat(n, 64)
				if err != nil {
					return n
				}
				var nb strings.Builder
				write(&nb, f)
				return nb.String()
			})
		}
		q, _ := json.Marshal(x)
		b.Write(q)
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			write(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k, e := range x {
			if e == nil {
				// The oracle writes undefined as null, and zuzu leaves out a
				// field that is undefined, so a null field counts as absent.
				continue
			}
			keys = append(keys, k)
		}
		slices.Sort(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			q, _ := json.Marshal(k)
			b.Write(q)
			b.WriteByte(':')
			write(b, x[k])
		}
		b.WriteByte('}')
	default:
		fmt.Fprintf(b, "%v", x)
	}
}

// compareStructure compiles a Vega-Lite spec with zuzu and compares the
// result with the spec that vega-lite.js compiled: the names of the
// datasets, the names and the types of the scales, and the names and the
// types of the marks. It sets Same when the JSON is the same.
func compareStructure(res *Result, src []byte, want jsontext.Value) bool {
	spec, err := vegalite.Parse(src)
	if err != nil {
		return false
	}
	got, _, err := vegalite.Compile(spec)
	if err != nil {
		return false
	}
	b, err := vega.Write(got)
	if err != nil {
		return false
	}
	var g, w map[string]any
	if json.Unmarshal(b, &g) != nil || json.Unmarshal(want, &w) != nil {
		return false
	}
	res.Same = canonical(g) == canonical(w)
	return canonical(structure(g)) == canonical(structure(w))
}

// structure returns the datasets, the scales and the marks of a Vega spec,
// by name and type.
func structure(spec map[string]any) map[string]any {
	names := func(key string, fields ...string) []any {
		list, _ := spec[key].([]any)
		var out []any
		for _, e := range list {
			m, _ := e.(map[string]any)
			var parts []string
			for _, f := range fields {
				parts = append(parts, fmt.Sprint(m[f]))
			}
			out = append(out, strings.Join(parts, " "))
		}
		slices.SortFunc(out, func(a, b any) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
		return out
	}
	var marks []any
	var walk func(list []any)
	walk = func(list []any) {
		for _, e := range list {
			m, _ := e.(map[string]any)
			marks = append(marks, fmt.Sprint(m["name"], " ", m["type"]))
			if sub, ok := m["marks"].([]any); ok {
				walk(sub)
			}
		}
	}
	list, _ := spec["marks"].([]any)
	walk(list)
	return map[string]any{"data": names("data", "name"), "scales": names("scales", "name", "type"), "marks": marks}
}

// shadowedData returns the names of the datasets of the top level of a
// Vega spec that a group mark also defines.
func shadowedData(src []byte) map[string]bool {
	var spec struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
		Marks []any `json:"marks"`
	}
	if json.Unmarshal(src, &spec) != nil {
		return nil
	}
	top := map[string]bool{}
	for _, d := range spec.Data {
		top[d.Name] = true
	}
	out := map[string]bool{}
	var walk func(marks []any)
	walk = func(marks []any) {
		for _, m := range marks {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			data, _ := mm["data"].([]any)
			for _, d := range data {
				if dm, ok := d.(map[string]any); ok {
					if name, ok := dm["name"].(string); ok && top[name] {
						out[name] = true
					}
				}
			}
			inner, _ := mm["marks"].([]any)
			walk(inner)
		}
	}
	walk(spec.Marks)
	return out
}
