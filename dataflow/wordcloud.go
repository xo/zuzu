package dataflow

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
)

// varying reads a parameter that is a constant, or a field or an
// expression of the row, such as the font size of a word cloud.
func (p *params) varying(name string, def any) (func(Row) any, error) {
	v, ok, err := p.raw(name)
	if err != nil {
		return nil, err
	}
	if !ok || v == nil {
		return func(Row) any { return def }, nil
	}
	if m, ok := v.(map[string]any); ok && (m["field"] != nil || m["expr"] != nil) {
		a, err := p.accessor(m)
		if err != nil {
			return nil, err
		}
		return a.Get, nil
	}
	return func(Row) any { return v }, nil
}

// cloudWord is a word of a word cloud while the layout places it.
type cloudWord struct {
	row                 Row
	text, font, style   string
	weight              any
	size, angle         float64
	x, y, width, height float64
	// hw and hh are the half width and height of the box of the word,
	// along its own axes.
	hw, hh float64
	placed bool
}

// wordcloud places words in a rectangle without overlap, the largest
// first: each word starts near the middle, at a random place, and moves
// out along a spiral until it fits. The font size of each word, read from
// a field, maps to the range of sizes by a square root. The box of a word
// is an estimate from its size, since the layout does not measure text.
// A word that does not fit gets no place and a font size of 0, as in Vega.
func wordcloud(p *params, rows []Row) ([]Row, error) {
	text, err := p.field("text")
	if err != nil {
		return nil, err
	}
	if text == nil {
		return nil, fmt.Errorf("wordcloud needs a text field: %w", ErrParam)
	}
	size, err := p.numbers("size")
	if err != nil {
		return nil, err
	}
	if size == nil {
		size = []float64{500, 500}
	}
	if len(size) != 2 {
		return nil, fmt.Errorf("wordcloud needs a size of two numbers: %w", ErrParam)
	}
	padding, err := p.number("padding", 1)
	if err != nil {
		return nil, err
	}
	spiral, err := p.str("spiral", "archimedean")
	if err != nil {
		return nil, err
	}
	font, err := p.varying("font", "sans-serif")
	if err != nil {
		return nil, err
	}
	style, err := p.varying("fontStyle", "normal")
	if err != nil {
		return nil, err
	}
	weight, err := p.varying("fontWeight", "normal")
	if err != nil {
		return nil, err
	}
	rotate, err := p.varying("rotate", 0.0)
	if err != nil {
		return nil, err
	}
	fontSize, err := p.varying("fontSize", 14.0)
	if err != nil {
		return nil, err
	}
	sizeOf := func(r Row) float64 { return expr.Number(fontSize(r)) }
	if raw, ok, _ := p.raw("fontSize"); ok {
		if m, ok := raw.(map[string]any); ok && (m["field"] != nil || m["expr"] != nil) {
			rng, err := p.numbers("fontSizeRange")
			if err != nil {
				return nil, err
			}
			if _, set, _ := p.raw("fontSizeRange"); !set {
				rng = []float64{10, 50}
			}
			if len(rng) == 2 {
				sizeOf = sqrtScale(rows, fontSize, rng)
			}
		}
	}
	as, err := p.as("x", "y", "font", "fontSize", "fontStyle", "fontWeight", "angle")
	if err != nil {
		return nil, err
	}
	words := make([]*cloudWord, len(rows))
	for i, r := range rows {
		c := clone(r)
		w := &cloudWord{
			row: c, text: expr.String(text.Get(r)), font: expr.String(font(r)),
			style: expr.String(style(r)), weight: weight(r),
			size: sizeOf(r), angle: expr.Number(rotate(r)),
		}
		// The box of the ink of the word, turned by its angle, with the
		// padding: the letters are about 0.6 of the size wide and the
		// capitals 0.7 of it high.
		a := w.angle * math.Pi / 180
		tw := 0.6*w.size*float64(len([]rune(w.text))) + 2*padding
		th := 0.7*w.size + 2*padding
		w.width = math.Abs(tw*math.Cos(a)) + math.Abs(th*math.Sin(a))
		w.height = math.Abs(tw*math.Sin(a)) + math.Abs(th*math.Cos(a))
		w.hw, w.hh = tw/2, th/2
		words[i] = w
	}
	order := slices.Clone(words)
	slices.SortStableFunc(order, func(a, b *cloudWord) int { return cmp.Compare(b.size, a.size) })
	var placed []*cloudWord
	W, H := size[0], size[1]
	maxDelta := math.Hypot(W, H)
	for _, w := range order {
		if !(w.size > 0) || w.width > W || w.height > H {
			continue
		}
		x0 := W * (p.env.Random() + 0.5) / 2
		y0 := H * (p.env.Random() + 0.5) / 2
		dt := 1.0
		if p.env.Random() < 0.5 {
			dt = -1
		}
		for t := 0.0; ; t += dt {
			dx, dy := spiralAt(spiral, W/H, t)
			if math.Min(math.Abs(dx), math.Abs(dy)) >= maxDelta {
				break
			}
			w.x, w.y = x0+dx, y0+dy
			if w.x-w.width/2 < 0 || w.y-w.height/2 < 0 || w.x+w.width/2 > W || w.y+w.height/2 > H {
				continue
			}
			if !overlapsAny(w, placed) {
				w.placed = true
				placed = append(placed, w)
				break
			}
		}
	}
	out := make([]Row, len(words))
	for i, w := range words {
		r := w.row
		if w.placed {
			// The text sits on its baseline, below the middle of its ink
			// along its own axis.
			a := w.angle * math.Pi / 180
			d := 0.35 * w.size
			r[as[0]], r[as[1]] = w.x-d*math.Sin(a), w.y+d*math.Cos(a)
			r[as[3]] = w.size
		} else {
			r[as[0]], r[as[1]] = math.NaN(), math.NaN()
			r[as[3]] = 0.0
		}
		r[as[2]], r[as[4]], r[as[5]], r[as[6]] = w.font, w.style, w.weight, w.angle
		out[i] = r
	}
	return out, nil
}

// sqrtScale maps the values of a field from their extent to a range by a
// square root.
func sqrtScale(rows []Row, f func(Row) any, rng []float64) func(Row) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, r := range rows {
		v := expr.Number(f(r))
		if math.IsNaN(v) {
			continue
		}
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	sl, sh := math.Sqrt(lo), math.Sqrt(hi)
	return func(r Row) float64 {
		v := math.Sqrt(expr.Number(f(r)))
		if sh == sl {
			return (rng[0] + rng[1]) / 2
		}
		return rng[0] + (v-sl)/(sh-sl)*(rng[1]-rng[0])
	}
}

// spiralAt returns the point of a spiral at step t: an Archimedean spiral
// stretched to the shape of the rectangle, or a square spiral.
func spiralAt(kind string, e, t float64) (float64, float64) {
	if kind == "rectangular" {
		// The square spiral walks its sides at steps of 4.
		s := math.Abs(t)
		k := math.Floor(math.Sqrt(s))
		d := s - k*k
		x, y := k, -k
		switch {
		case d < k:
			y += 2 * d
		case d < 2*k:
			y, x = k, k-2*(d-k)
		default:
			x, y = -k, k-2*(d-2*k)
		}
		return 4 * e * x, 4 * y
	}
	t *= 0.1
	return e * t * math.Cos(t), t * math.Sin(t)
}

// overlapsAny reports whether the box of a word, turned by its angle,
// overlaps the box of a word already placed.
func overlapsAny(w *cloudWord, placed []*cloudWord) bool {
	for _, o := range placed {
		if math.Abs(w.x-o.x)*2 < w.width+o.width && math.Abs(w.y-o.y)*2 < w.height+o.height && boxesOverlap(w, o) {
			return true
		}
	}
	return false
}

// boxesOverlap tests two turned boxes on the four axes of their sides: they
// overlap unless one axis separates them.
func boxesOverlap(a, b *cloudWord) bool {
	dx, dy := b.x-a.x, b.y-a.y
	axes := func(w *cloudWord) [2][2]float64 {
		r := w.angle * math.Pi / 180
		c, s := math.Cos(r), math.Sin(r)
		return [2][2]float64{{c, s}, {-s, c}}
	}
	aa, ba := axes(a), axes(b)
	reach := func(w *cloudWord, ax [2][2]float64, u [2]float64) float64 {
		return w.hw*math.Abs(ax[0][0]*u[0]+ax[0][1]*u[1]) + w.hh*math.Abs(ax[1][0]*u[0]+ax[1][1]*u[1])
	}
	for _, u := range [][2]float64{aa[0], aa[1], ba[0], ba[1]} {
		d := math.Abs(dx*u[0] + dy*u[1])
		if d >= reach(a, aa, u)+reach(b, ba, u) {
			return false
		}
	}
	return true
}
