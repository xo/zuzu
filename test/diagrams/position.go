// Package diagrams checks the fixtures of DOT and of Mermaid against the
// references of dot and of mmdc (D31), and measures how far the places of
// the nodes of a layout of zuzu lie from those of a reference (D40 part 11,
// D44 part 4).
package diagrams

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/expr"
	mathfont "github.com/xo/zuzu/text/math"
)

// Places holds the center of each node of a drawing, by its id.
type Places map[string][2]float64

// PositionError returns how far the nodes of a drawing lie from the nodes
// of its reference, as one number from 0 up, and the number of nodes that
// it compares (D40 part 11).
//
// It compares the nodes that both drawings have. Each drawing is first
// moved and scaled on its own: the box around the centers of those nodes
// gets its middle at 0, 0, and the diagonal of the box gets the length 1.
// The error is then the mean distance between the center of a node in one
// drawing and its center in the other. So 0 is the same places up to the
// scale, and 0.25 is a node a quarter of the diagonal away from its place
// on average. The scale takes out the size of the look, which is smaller in
// zuzu than in Mermaid, so that the number measures how the nodes lie to
// each other. The scale is the same in x and in y, so that a drawing that
// is wider or taller than its reference counts. With a scale of x and y
// apart, a small step of a row of nodes turns into a step across the whole
// drawing.
//
// It returns false when the drawings share fewer than two nodes, or when
// the centers of the nodes of one drawing all lie on one point.
func PositionError(got, want Places) (float64, int, bool) {
	var ids []string
	for id := range want {
		if _, ok := got[id]; ok {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		return 0, len(ids), false
	}
	slices.Sort(ids)
	a, aok := unit(got, ids)
	b, bok := unit(want, ids)
	if !aok || !bok {
		return 0, len(ids), false
	}
	sum := 0.0
	for i := range ids {
		sum += math.Hypot(a[i][0]-b[i][0], a[i][1]-b[i][1])
	}
	return sum / float64(len(ids)), len(ids), true
}

// unit returns the centers of the nodes ids of a drawing, moved so that
// the middle of the box around them is at 0, 0 and scaled so that the
// diagonal of the box is 1, and false when the box has no diagonal.
func unit(p Places, ids []string) ([][2]float64, bool) {
	lo := [2]float64{math.Inf(1), math.Inf(1)}
	hi := [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, id := range ids {
		for k := range 2 {
			lo[k], hi[k] = math.Min(lo[k], p[id][k]), math.Max(hi[k], p[id][k])
		}
	}
	d := math.Hypot(hi[0]-lo[0], hi[1]-lo[1])
	if d < 1e-9 {
		return nil, false
	}
	out := make([][2]float64, len(ids))
	for i, id := range ids {
		for k := range 2 {
			out[i][k] = (p[id][k] - (lo[k]+hi[k])/2) / d
		}
	}
	return out, true
}

// ReadPlaces reads the centers of the nodes of a reference that
// cmd/mmdcref wrote.
func ReadPlaces(path string) (Places, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Nodes []struct {
			ID string  `json:"id"`
			X  float64 `json:"x"`
			Y  float64 `json:"y"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	out := Places{}
	for _, n := range doc.Nodes {
		out[n.ID] = [2]float64{n.X, n.Y}
	}
	return out, nil
}

// ZuzuPlaces draws a source of Mermaid with zuzu, with the bundled fonts,
// the font of math and more options of Parse, such as zuzu.WithLayout or
// zuzu.WithLayoutEngine, and returns the center of each node, and the
// report of the render.
func ZuzuPlaces(ctx context.Context, src []byte, opts ...zuzu.Option) (Places, zuzu.Report, error) {
	opts = append([]zuzu.Option{zuzu.WithDialect(zuzu.Mermaid), zuzu.WithMath(mathfont.Font())}, opts...)
	c, report, err := zuzu.Parse(src, opts...)
	if err != nil {
		return nil, report, err
	}
	v, more, err := c.NewView(ctx)
	report = append(report, more...)
	if err != nil {
		return nil, report, err
	}
	rows, err := v.Data("layoutNodes")
	if err != nil {
		return nil, report, err
	}
	out := Places{}
	for _, r := range rows {
		out[expr.String(r["key"])] = [2]float64{expr.Number(r["x"]), expr.Number(r["y"])}
	}
	return out, report, nil
}
