package dataflow

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
)

// chordFlows reads the groups and the matrix of flows of a dataset of
// links. The groups come in the order in which each first appears.
type chordFlows struct {
	keys  []any
	index map[string]int
	m     [][]float64
	// pairs maps each link to its source and target groups.
	pairs [][2]int
}

func readFlows(rows []Row, src, dst, val *Accessor) *chordFlows {
	f := &chordFlows{index: map[string]int{}}
	group := func(v any) int {
		k := key([]any{v})
		i, ok := f.index[k]
		if !ok {
			i = len(f.keys)
			f.index[k] = i
			f.keys = append(f.keys, v)
		}
		return i
	}
	for _, r := range rows {
		f.pairs = append(f.pairs, [2]int{group(src.Get(r)), group(dst.Get(r))})
	}
	n := len(f.keys)
	f.m = make([][]float64, n)
	for i := range f.m {
		f.m[i] = make([]float64, n)
	}
	for i, r := range rows {
		v := expr.Number(val.Get(r))
		if math.IsNaN(v) || v < 0 {
			v = 0
		}
		f.m[f.pairs[i][0]][f.pairs[i][1]] += v
	}
	return f
}

// order returns the indexes 0 to n-1, sorted by a value as a sort order
// asks: ascending, descending, or null for none.
func order(n int, by func(int) float64, sortBy string) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	switch sortBy {
	case "ascending":
		slices.SortStableFunc(out, func(a, b int) int { return cmp.Compare(by(a), by(b)) })
	case "descending":
		slices.SortStableFunc(out, func(a, b int) int { return cmp.Compare(by(b), by(a)) })
	}
	return out
}

// chordParams are the parameters that zuzu.chord and zuzu.chordribbons
// share.
type chordParams struct {
	src, dst, val *Accessor
	subgroups     string
	directed      bool
}

func (p *params) chordParams() (*chordParams, error) {
	c := &chordParams{}
	var err error
	if c.src, c.dst, c.val, err = p.flowFields(); err != nil {
		return nil, err
	}
	if c.subgroups, err = p.sortOrder("sortSubgroups"); err != nil {
		return nil, err
	}
	if c.directed, err = p.boolean("directed", false); err != nil {
		return nil, err
	}
	return c, nil
}

// sortOrder reads a sort order: ascending, descending, or null.
func (p *params) sortOrder(name string) (string, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return "", err
	}
	s := expr.String(v)
	if s != "ascending" && s != "descending" {
		return "", fmt.Errorf("the %s %q: %w", name, s, ErrParam)
	}
	return s, nil
}

// size returns the size of a group: the flow out of it, and with directed
// also the flow into it.
func (c *chordParams) size(f *chordFlows, i int) float64 {
	s := 0.0
	for j := range f.m {
		s += f.m[i][j]
		if c.directed {
			s += f.m[j][i]
		}
	}
	return s
}

// slots returns the order of the parts of a group: the flow out to each
// group, then with directed the flow in from each group. A part is
// (j, true) for the flow out to j and (j, false) for the flow in from j.
func (c *chordParams) slots(f *chordFlows, i int) []chordSlot {
	n := len(f.keys)
	var out []chordSlot
	for _, j := range order(n, func(j int) float64 { return f.m[i][j] }, c.subgroups) {
		out = append(out, chordSlot{j, true, f.m[i][j]})
	}
	if c.directed {
		for _, j := range order(n, func(j int) float64 { return f.m[j][i] }, c.subgroups) {
			out = append(out, chordSlot{j, false, f.m[j][i]})
		}
	}
	return out
}

type chordSlot struct {
	other int
	out   bool
	value float64
}

// chord lays out the groups of a chord diagram around a circle: each group
// an arc whose angle is its share of the flow, with padAngle between.
func chord(p *params, rows []Row) ([]Row, error) {
	c, err := p.chordParams()
	if err != nil {
		return nil, err
	}
	pad, err := p.number("padAngle", 0)
	if err != nil {
		return nil, err
	}
	groups, err := p.sortOrder("sortGroups")
	if err != nil {
		return nil, err
	}
	as, err := p.as("key", "index", "startAngle", "endAngle", "value")
	if err != nil {
		return nil, err
	}
	f := readFlows(rows, c.src, c.dst, c.val)
	n := len(f.keys)
	sizes := make([]float64, n)
	total := 0.0
	for i := range n {
		sizes[i] = c.size(f, i)
		total += sizes[i]
	}
	k := 0.0
	if total > 0 {
		k = math.Max(0, 2*math.Pi-pad*float64(n)) / total
	}
	out := make([]Row, n)
	x := 0.0
	for _, i := range order(n, func(i int) float64 { return sizes[i] }, groups) {
		start := x
		x += sizes[i] * k
		out[i] = Row{as[0]: f.keys[i], as[1]: float64(i), as[2]: start, as[3]: x, as[4]: sizes[i]}
		x += pad
	}
	return out, nil
}

// chordribbons draws the ribbons of a chord diagram from its groups: each
// part of a group, its flow to or from another group, takes its share of
// the arc of the group in the order of the subgroups, and a ribbon joins
// the two parts of a link across the circle.
func chordribbons(p *params, rows []Row) ([]Row, error) {
	c, err := p.chordParams()
	if err != nil {
		return nil, err
	}
	name, err := p.str("groups", "")
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("zuzu.chordribbons needs groups: %w", ErrParam)
	}
	groupRows, err := p.env.Data(name)
	if err != nil {
		return nil, err
	}
	radius, err := p.number("radius", math.NaN())
	if err != nil {
		return nil, err
	}
	if math.IsNaN(radius) {
		return nil, fmt.Errorf("zuzu.chordribbons needs a radius: %w", ErrParam)
	}
	as, err := p.as("sourceStartAngle", "sourceEndAngle", "targetStartAngle", "targetEndAngle", "path")
	if err != nil {
		return nil, err
	}
	f := readFlows(rows, c.src, c.dst, c.val)
	n := len(f.keys)
	// The angles of each part: [group][other][out] is [start, end].
	type part = [2]float64
	parts := make([]map[[2]int]part, n)
	for _, g := range groupRows {
		i, ok := f.index[key([]any{g["key"]})]
		if !ok {
			continue
		}
		start, end := expr.Number(g["startAngle"]), expr.Number(g["endAngle"])
		size := c.size(f, i)
		k := 0.0
		if size > 0 {
			k = (end - start) / size
		}
		parts[i] = map[[2]int]part{}
		x := start
		for _, s := range c.slots(f, i) {
			dir := 0
			if s.out {
				dir = 1
			}
			parts[i][[2]int{s.other, dir}] = part{x, x + s.value*k}
			x += s.value * k
		}
	}
	drawn := map[[2]int]bool{}
	out := make([]Row, len(rows))
	for li, r := range rows {
		cr := clone(r)
		i, j := f.pairs[li][0], f.pairs[li][1]
		var a, b part
		if parts[i] != nil && parts[j] != nil {
			a = parts[i][[2]int{j, 1}]
			if c.directed {
				b = parts[j][[2]int{i, 0}]
			} else {
				b = parts[j][[2]int{i, 1}]
			}
		}
		cr[as[0]], cr[as[1]], cr[as[2]], cr[as[3]] = a[0], a[1], b[0], b[1]
		pair := [2]int{min(i, j), max(i, j)}
		if c.directed {
			pair = [2]int{i, j}
		}
		if drawn[pair] || parts[i] == nil || parts[j] == nil {
			cr[as[4]] = nil
		} else {
			drawn[pair] = true
			cr[as[4]] = ribbonPath(radius, a, b)
		}
		out[li] = cr
	}
	return out, nil
}

// ribbonPath draws a ribbon around the center, between two arcs of the
// circle of radius r. The angles start at the top and run clockwise, as
// the arc mark of Vega reads them.
func ribbonPath(r float64, a, b [2]float64) string {
	pt := func(t float64) string { return num(r*math.Sin(t)) + "," + num(-r*math.Cos(t)) }
	arc := func(s [2]float64) string {
		large := "0"
		if s[1]-s[0] > math.Pi {
			large = "1"
		}
		return "A" + num(r) + "," + num(r) + " 0 " + large + ",1 " + pt(s[1])
	}
	var sb strings.Builder
	sb.WriteString("M" + pt(a[0]) + arc(a))
	sb.WriteString("Q0,0 " + pt(b[0]) + arc(b))
	sb.WriteString("Q0,0 " + pt(a[0]) + "Z")
	return sb.String()
}
