package dataflow

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
)

// numbersOf returns the values of a field of rows that are numbers, in
// the order of the rows.
func numbersOf(f *Accessor, rows []Row) []float64 {
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		v := f.Get(r)
		if !valid(v) {
			continue
		}
		x := expr.Number(v)
		if !math.IsNaN(x) {
			out = append(out, x)
		}
	}
	return out
}

// coerced returns the values of a field of rows as numbers, with a null
// as 0, as JavaScript converts it, and without values that are not numbers.
func coerced(f *Accessor, rows []Row) []float64 {
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		v := f.Get(r)
		if v == nil {
			out = append(out, 0)
			continue
		}
		if x := expr.Number(v); !math.IsNaN(x) {
			out = append(out, x)
		}
	}
	return out
}

// quantileSorted returns the quantile p of sorted values, or NaN when
// there are none.
func quantileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	return quantile(sorted, math.Max(0, math.Min(1, p)))
}

// scott returns the bandwidth of a kernel density estimate by the rule of
// Scott: 1.06 times the lesser of the standard deviation and the
// interquartile range over 1.34 of the values, times a count to the power
// -1/5. As in vega.js, the count holds the nulls, which the spread leaves
// out.
func scott(values []float64, count int) float64 {
	n := len(values)
	if n < 2 {
		return 1
	}
	sorted := slices.Sorted(slices.Values(values))
	mean := 0.0
	for _, v := range sorted {
		mean += v
	}
	mean /= float64(n)
	ss := 0.0
	for _, v := range sorted {
		ss += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(ss / float64(n-1))
	iqr := (quantileSorted(sorted, 0.75) - quantileSorted(sorted, 0.25)) / 1.34
	spread := sd
	if iqr > 0 && iqr < sd {
		spread = iqr
	}
	return 1.06 * spread * math.Pow(float64(count), -0.2)
}

// gaussian returns the density and the cumulative distribution of a kernel
// density estimate with a gaussian kernel.
func gaussian(values []float64, h float64) (pdf, cdf func(float64) float64) {
	n := float64(len(values))
	c := 1 / (n * h * math.Sqrt(2*math.Pi))
	pdf = func(x float64) float64 {
		s := 0.0
		for _, v := range values {
			z := (x - v) / h
			s += math.Exp(-z * z / 2)
		}
		return c * s
	}
	cdf = func(x float64) float64 {
		s := 0.0
		for _, v := range values {
			s += 0.5 * math.Erfc(-(x-v)/(h*math.Sqrt2))
		}
		return s / n
	}
	return pdf, cdf
}

// sampleUniform samples a function at steps equal intervals of an extent,
// at steps+1 points.
func sampleUniform(f func(float64) float64, lo, hi float64, steps int) [][2]float64 {
	steps = max(steps, 1)
	out := make([][2]float64, steps+1)
	for i := range out {
		x := lo + (hi-lo)*float64(i)/float64(steps)
		out[i] = [2]float64{x, f(x)}
	}
	return out
}

// minRadians is the bend of a curve that makes the adaptive sampling add
// a point: half a degree.
const minRadians = 0.5 * math.Pi / 180

// sampleCurve samples a function over an extent for drawing. It starts at
// minSteps equal intervals, and halves an interval where the curve bends,
// down to an interval of the extent over maxSteps. A curve bends when the
// direction from the start of the interval to its middle differs from the
// direction to its end by more than half a degree, with x and y scaled to
// the extent and to the range of the first samples. zuzu chose this rule
// where the documentation of Vega names only minsteps and maxsteps, and it
// matches the references of the oracle (D6).
func sampleCurve(f func(float64) float64, lo, hi float64, minSteps, maxSteps int) [][2]float64 {
	first := sampleUniform(f, lo, hi, minSteps)
	if hi <= lo || maxSteps <= minSteps {
		return first
	}
	ymin, ymax := math.Inf(1), math.Inf(-1)
	for _, p := range first {
		ymin, ymax = math.Min(ymin, p[1]), math.Max(ymax, p[1])
	}
	sx, sy := 1/(hi-lo), 1.0
	if ymax > ymin {
		sy = 1 / (ymax - ymin)
	}
	stop := (hi - lo) / float64(maxSteps)
	angle := func(a, b [2]float64) float64 {
		return math.Atan2((b[1]-a[1])*sy, (b[0]-a[0])*sx)
	}
	out := [][2]float64{first[0]}
	var split func(a, b [2]float64)
	split = func(a, b [2]float64) {
		x := (a[0] + b[0]) / 2
		m := [2]float64{x, f(x)}
		if (b[0]-a[0])/2 >= stop*(1-1e-9) && math.Abs(angle(a, m)-angle(a, b)) > minRadians {
			split(a, m)
			out = append(out, m)
			split(m, b)
		}
	}
	for i := 1; i < len(first); i++ {
		split(first[i-1], first[i])
		out = append(out, first[i])
	}
	return out
}

// extentOf returns the least and the greatest of values.
func extentOf(values []float64) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return lo, hi
}

// readExtent reads a parameter that holds an extent, [min, max]. It
// reports false when the transform has none.
func readExtent(p *params, name string) (float64, float64, bool, error) {
	l, err := p.list(name)
	if err != nil || len(l) < 2 || l[0] == nil || l[1] == nil {
		return 0, 0, false, err
	}
	return expr.Number(l[0]), expr.Number(l[1]), true, nil
}

// kde estimates the density of a field of each group with a gaussian
// kernel, and samples it over an extent.
func kde(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	cumulative, err := p.boolean("cumulative", false)
	if err != nil {
		return nil, err
	}
	counts, err := p.boolean("counts", false)
	if err != nil {
		return nil, err
	}
	bandwidth, err := p.number("bandwidth", 0)
	if err != nil {
		return nil, err
	}
	minSteps, maxSteps, steps, err := readSteps(p)
	if err != nil {
		return nil, err
	}
	resolve, err := p.str("resolve", "independent")
	if err != nil {
		return nil, err
	}
	as, err := p.as("value", "density")
	if err != nil {
		return nil, err
	}
	lo, hi, hasExtent, err := readExtent(p, "extent")
	if err != nil {
		return nil, err
	}
	a := &aggregation{groupby: groupby}
	order, groups, keys := a.group(rows)
	if !hasExtent && resolve == "shared" {
		lo, hi = extentOf(numbersOf(field, rows))
	}
	var out []Row
	for _, k := range order {
		values := numbersOf(field, groups[k])
		if len(values) == 0 {
			continue
		}
		// As in vega.js, a null counts as the number 0 in the estimate,
		// in its bandwidth and in its count.
		all := coerced(field, groups[k])
		h := bandwidth
		if h <= 0 {
			h = scott(values, len(all))
		}
		pdf, cdf := gaussian(all, h)
		f := pdf
		if cumulative {
			f = cdf
		}
		if counts {
			n, base := float64(len(all)), f
			f = func(x float64) float64 { return n * base(x) }
		}
		glo, ghi := lo, hi
		if !hasExtent && resolve != "shared" {
			glo, ghi = extentOf(values)
		}
		var points [][2]float64
		switch {
		case steps > 0:
			points = sampleUniform(f, glo, ghi, steps)
		case resolve == "shared":
			// Shared densities have the same steps, so that they stack.
			points = sampleUniform(f, glo, ghi, maxSteps)
		default:
			points = sampleCurve(f, glo, ghi, minSteps, maxSteps)
		}
		for _, pt := range points {
			r := Row{}
			for i, g := range groupby {
				r[g.Name] = keys[k][i]
			}
			r[as[0]], r[as[1]] = pt[0], pt[1]
			out = append(out, r)
		}
	}
	return out, nil
}

// readSteps reads the parameters minsteps, maxsteps and steps of a
// sampling.
func readSteps(p *params) (minSteps, maxSteps, steps int, err error) {
	lo, err := p.number("minsteps", 25)
	if err != nil {
		return 0, 0, 0, err
	}
	hi, err := p.number("maxsteps", 200)
	if err != nil {
		return 0, 0, 0, err
	}
	n, err := p.number("steps", 0)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(lo), int(hi), int(n), nil
}

// distribution is a probability distribution with a density and a
// cumulative distribution.
type distribution struct {
	pdf, cdf func(float64) float64
	// lo and hi are the extent of the data of a kernel density estimate.
	lo, hi float64
}

// readDistribution reads the distribution of a density transform: normal,
// lognormal, uniform, kde over the rows, or a mixture of them.
func readDistribution(p *params, v any, rows []Row) (*distribution, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("distribution %v: %w", v, ErrParam)
	}
	num := func(k string, def float64) (float64, error) {
		x, ok := m[k]
		if !ok || x == nil {
			return def, nil
		}
		if s, ok := x.(map[string]any); ok && s["signal"] != nil {
			out, err := p.eval(expr.String(s["signal"]))
			if err != nil {
				return 0, err
			}
			return expr.Number(out), nil
		}
		return expr.Number(x), nil
	}
	fn := expr.String(m["function"])
	switch fn {
	case "normal", "lognormal":
		mean, err := num("mean", 0)
		if err != nil {
			return nil, err
		}
		sd, err := num("stdev", 1)
		if err != nil {
			return nil, err
		}
		pdf := func(x float64) float64 {
			z := (x - mean) / sd
			return math.Exp(-z*z/2) / (sd * math.Sqrt(2*math.Pi))
		}
		cdf := func(x float64) float64 { return 0.5 * math.Erfc(-(x-mean)/(sd*math.Sqrt2)) }
		if fn == "lognormal" {
			pdf = func(x float64) float64 {
				if x <= 0 {
					return 0
				}
				z := (math.Log(x) - mean) / sd
				return math.Exp(-z*z/2) / (sd * x * math.Sqrt(2*math.Pi))
			}
			cdf = func(x float64) float64 {
				if x <= 0 {
					return 0
				}
				return 0.5 * math.Erfc(-(math.Log(x)-mean)/(sd*math.Sqrt2))
			}
		}
		return &distribution{pdf: pdf, cdf: cdf}, nil
	case "uniform":
		lo, err := num("min", 0)
		if err != nil {
			return nil, err
		}
		hi, err := num("max", 1)
		if err != nil {
			return nil, err
		}
		return &distribution{
			pdf: func(x float64) float64 {
				if x < lo || x > hi {
					return 0
				}
				return 1 / (hi - lo)
			},
			cdf: func(x float64) float64 { return math.Max(0, math.Min(1, (x-lo)/(hi-lo))) },
		}, nil
	case "kde":
		f, err := p.accessor(m["field"])
		if err != nil {
			return nil, err
		}
		values := numbersOf(f, rows)
		h, err := num("bandwidth", 0)
		if err != nil {
			return nil, err
		}
		if h <= 0 {
			h = scott(values, len(values))
		}
		pdf, cdf := gaussian(values, h)
		lo, hi := extentOf(values)
		return &distribution{pdf: pdf, cdf: cdf, lo: lo, hi: hi}, nil
	case "mixture":
		list, _ := m["distributions"].([]any)
		weights, _ := m["weights"].([]any)
		var parts []*distribution
		var ws []float64
		total := 0.0
		for i, d := range list {
			part, err := readDistribution(p, d, rows)
			if err != nil {
				return nil, err
			}
			w := 1.0
			if i < len(weights) {
				w = expr.Number(weights[i])
			}
			parts, ws, total = append(parts, part), append(ws, w), total+w
		}
		mix := func(get func(*distribution) func(float64) float64) func(float64) float64 {
			return func(x float64) float64 {
				s := 0.0
				for i, part := range parts {
					s += ws[i] / total * get(part)(x)
				}
				return s
			}
		}
		return &distribution{
			pdf: mix(func(d *distribution) func(float64) float64 { return d.pdf }),
			cdf: mix(func(d *distribution) func(float64) float64 { return d.cdf }),
		}, nil
	}
	return nil, fmt.Errorf("distribution %q: %w", fn, ErrParam)
}

// density samples the density or the cumulative distribution of a
// probability distribution over an extent.
func density(p *params, rows []Row) ([]Row, error) {
	raw, _, err := p.raw("distribution")
	if err != nil {
		return nil, err
	}
	dist, err := readDistribution(p, raw, rows)
	if err != nil {
		return nil, err
	}
	method, err := p.str("method", "pdf")
	if err != nil {
		return nil, err
	}
	minSteps, maxSteps, steps, err := readSteps(p)
	if err != nil {
		return nil, err
	}
	as, err := p.as("value", "density")
	if err != nil {
		return nil, err
	}
	lo, hi, ok, err := readExtent(p, "extent")
	if err != nil {
		return nil, err
	}
	if !ok {
		lo, hi = dist.lo, dist.hi
		if lo >= hi {
			return nil, fmt.Errorf("density with no extent: %w", ErrParam)
		}
	}
	f := dist.pdf
	if method == "cdf" {
		f = dist.cdf
	}
	points := sampleCurve(f, lo, hi, minSteps, maxSteps)
	if steps > 0 {
		points = sampleUniform(f, lo, hi, steps)
	}
	out := make([]Row, len(points))
	for i, pt := range points {
		out[i] = Row{as[0]: pt[0], as[1]: pt[1]}
	}
	return out, nil
}

// quantiles computes quantiles of a field of each group, at a list of
// probabilities, or at steps of a probability from half a step.
func quantiles(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	probs, err := p.numbers("probs")
	if err != nil {
		return nil, err
	}
	if len(probs) == 0 {
		step, err := p.number("step", 0.01)
		if err != nil {
			return nil, err
		}
		if step <= 0 {
			return nil, fmt.Errorf("quantile step %v: %w", step, ErrParam)
		}
		for i := 0; ; i++ {
			v := step/2 + float64(i)*step
			if v >= 1 {
				break
			}
			probs = append(probs, v)
		}
	}
	as, err := p.as("prob", "value")
	if err != nil {
		return nil, err
	}
	a := &aggregation{groupby: groupby}
	order, groups, keys := a.group(rows)
	var out []Row
	for _, k := range order {
		sorted := slices.Sorted(slices.Values(numbersOf(field, groups[k])))
		for _, pr := range probs {
			r := Row{}
			for i, g := range groupby {
				r[g.Name] = keys[k][i]
			}
			r[as[0]], r[as[1]] = pr, quantileSorted(sorted, pr)
			out = append(out, r)
		}
	}
	return out, nil
}

// pairsOf returns the pairs of numbers of two fields of rows, where both
// are numbers.
func pairsOf(fx, fy *Accessor, rows []Row) (xs, ys []float64) {
	for _, r := range rows {
		x, y := fx.Get(r), fy.Get(r)
		if !valid(x) || !valid(y) {
			continue
		}
		a, b := expr.Number(x), expr.Number(y)
		if math.IsNaN(a) || math.IsNaN(b) {
			continue
		}
		xs, ys = append(xs, a), append(ys, b)
	}
	return xs, ys
}

// fit is a fitted model: its function, its coefficients and its
// coefficient of determination.
type fit struct {
	f        func(float64) float64
	coef     []float64
	rSquared float64
}

// polyFit fits a polynomial of an order by least squares, with the
// coefficients from the constant up.
func polyFit(xs, ys []float64, order int) []float64 {
	n := order + 1
	// The normal equations, as an augmented matrix.
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n+1)
	}
	for k, x := range xs {
		pow := make([]float64, 2*n)
		pow[0] = 1
		for j := 1; j < 2*n; j++ {
			pow[j] = pow[j-1] * x
		}
		for i := range n {
			for j := range n {
				m[i][j] += pow[i+j]
			}
			m[i][n] += pow[i] * ys[k]
		}
	}
	// Gaussian elimination with partial pivoting.
	for c := range n {
		best := c
		for r := c + 1; r < n; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[best][c]) {
				best = r
			}
		}
		m[c], m[best] = m[best], m[c]
		if m[c][c] == 0 {
			continue
		}
		for r := range n {
			if r == c {
				continue
			}
			f := m[r][c] / m[c][c]
			for j := c; j <= n; j++ {
				m[r][j] -= f * m[c][j]
			}
		}
	}
	coef := make([]float64, n)
	for i := range n {
		if m[i][i] != 0 {
			coef[i] = m[i][n] / m[i][i]
		}
	}
	return coef
}

// regressionFit fits a model of a method to points: linear, log, exp, pow,
// quad or poly of an order. The methods log, exp and pow fit a line to the
// logarithms of the values, which zuzu chose where the documentation of
// Vega does not say how (D6).
func regressionFit(method string, order int, xs, ys []float64) *fit {
	var ft fit
	switch method {
	case "log":
		lx := make([]float64, len(xs))
		for i, x := range xs {
			lx[i] = math.Log(x)
		}
		c := polyFit(lx, ys, 1)
		ft = fit{f: func(x float64) float64 { return c[0] + c[1]*math.Log(x) }, coef: c}
	case "exp":
		ly := make([]float64, len(ys))
		for i, y := range ys {
			ly[i] = math.Log(y)
		}
		c := polyFit(xs, ly, 1)
		a := math.Exp(c[0])
		ft = fit{f: func(x float64) float64 { return a * math.Exp(c[1]*x) }, coef: []float64{a, c[1]}}
	case "pow":
		lx, ly := make([]float64, len(xs)), make([]float64, len(ys))
		for i := range xs {
			lx[i], ly[i] = math.Log(xs[i]), math.Log(ys[i])
		}
		c := polyFit(lx, ly, 1)
		a := math.Exp(c[0])
		ft = fit{f: func(x float64) float64 { return a * math.Pow(x, c[1]) }, coef: []float64{a, c[1]}}
	default:
		deg := 1
		switch method {
		case "quad":
			deg = 2
		case "poly":
			deg = order
		}
		c := polyFit(xs, ys, deg)
		ft = fit{coef: c, f: func(x float64) float64 {
			s := 0.0
			for _, ci := range slices.Backward(c) {
				s = s*x + ci
			}
			return s
		}}
	}
	mean := 0.0
	for _, y := range ys {
		mean += y
	}
	mean /= float64(len(ys))
	sse, sst := 0.0, 0.0
	for i, x := range xs {
		d := ys[i] - ft.f(x)
		sse += d * d
		sst += (ys[i] - mean) * (ys[i] - mean)
	}
	ft.rSquared = 1 - sse/sst
	return &ft
}

// regression fits a model of y on x for each group, and samples it over
// an extent, or writes its coefficients.
func regression(p *params, rows []Row) ([]Row, error) {
	fx, err := p.field("x")
	if err != nil {
		return nil, err
	}
	fy, err := p.field("y")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	method, err := p.str("method", "linear")
	if err != nil {
		return nil, err
	}
	order, err := p.number("order", 3)
	if err != nil {
		return nil, err
	}
	withParams, err := p.boolean("params", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as(fx.Name, fy.Name)
	if err != nil {
		return nil, err
	}
	lo, hi, hasExtent, err := readExtent(p, "extent")
	if err != nil {
		return nil, err
	}
	a := &aggregation{groupby: groupby}
	order2, groups, keys := a.group(rows)
	var out []Row
	for _, k := range order2 {
		xs, ys := pairsOf(fx, fy, groups[k])
		if len(xs) == 0 {
			continue
		}
		ft := regressionFit(method, int(order), xs, ys)
		if withParams {
			var key any
			if len(groupby) > 0 {
				key = keys[k]
			}
			coef := make([]any, len(ft.coef))
			for i, c := range ft.coef {
				coef[i] = c
			}
			out = append(out, Row{"keys": key, "coef": coef, "rSquared": ft.rSquared})
			continue
		}
		glo, ghi := lo, hi
		if !hasExtent {
			glo, ghi = extentOf(xs)
		}
		points := [][2]float64{{glo, ft.f(glo)}, {ghi, ft.f(ghi)}}
		if method != "linear" {
			points = sampleCurve(ft.f, glo, ghi, 25, 200)
		}
		for _, pt := range points {
			r := Row{}
			for i, g := range groupby {
				r[g.Name] = keys[k][i]
			}
			r[as[0]], r[as[1]] = pt[0], pt[1]
			out = append(out, r)
		}
	}
	return out, nil
}

// loessIterations is the number of passes of loess that weigh down the
// points far from the fit.
const loessIterations = 2

// loessFit fits a locally weighted line at each point, over the nearest
// share of the points that the bandwidth gives, with tricube weights, and
// then reweighs the points by their residuals, as the method of Cleveland
// does. It returns the fit at each point of xs, which are sorted.
func loessFit(xs, ys []float64, bandwidth float64) []float64 {
	n := len(xs)
	k := max(2, int(math.Floor(bandwidth*float64(n))))
	k = min(k, n)
	robust := make([]float64, n)
	for i := range robust {
		robust[i] = 1
	}
	fitted := make([]float64, n)
	tricube := func(u float64) float64 {
		u = math.Abs(u)
		if u >= 1 {
			return 0
		}
		v := 1 - u*u*u
		return v * v * v
	}
	for it := 0; ; it++ {
		left, right := 0, k-1
		for i := range n {
			x := xs[i]
			for right+1 < n && xs[right+1]-x < x-xs[left] {
				left++
				right++
			}
			edge := right
			if x-xs[left] > xs[right]-x {
				edge = left
			}
			maxd := math.Abs(xs[edge] - x)
			var sw, swx, swy, swxx, swxy float64
			for j := left; j <= right; j++ {
				u := 0.0
				if maxd > 0 {
					u = (xs[j] - x) / maxd
				}
				w := tricube(u) * robust[j]
				sw += w
				swx += w * xs[j]
				swy += w * ys[j]
				swxx += w * xs[j] * xs[j]
				swxy += w * xs[j] * ys[j]
			}
			if sw == 0 {
				fitted[i] = ys[i]
				continue
			}
			mx, my := swx/sw, swy/sw
			slope := 0.0
			if d := swxx/sw - mx*mx; math.Abs(d) > 1e-12 {
				slope = (swxy/sw - mx*my) / d
			}
			fitted[i] = my + slope*(x-mx)
		}
		if it == loessIterations {
			return fitted
		}
		res := make([]float64, n)
		for i := range res {
			res[i] = math.Abs(ys[i] - fitted[i])
		}
		med := quantileSorted(slices.Sorted(slices.Values(res)), 0.5)
		if med < 1e-12 {
			return fitted
		}
		for i, r := range res {
			u := r / (6 * med)
			if u >= 1 {
				robust[i] = 0
			} else {
				robust[i] = (1 - u*u) * (1 - u*u)
			}
		}
	}
}

// loess fits a smooth curve of y on x for each group, and writes the fit at
// each distinct value of x.
func loess(p *params, rows []Row) ([]Row, error) {
	fx, err := p.field("x")
	if err != nil {
		return nil, err
	}
	fy, err := p.field("y")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	bandwidth, err := p.number("bandwidth", 0.3)
	if err != nil {
		return nil, err
	}
	as, err := p.as(fx.Name, fy.Name)
	if err != nil {
		return nil, err
	}
	a := &aggregation{groupby: groupby}
	order, groups, keys := a.group(rows)
	var out []Row
	for _, k := range order {
		xs, ys := pairsOf(fx, fy, groups[k])
		if len(xs) == 0 {
			continue
		}
		idx := make([]int, len(xs))
		for i := range idx {
			idx[i] = i
		}
		slices.SortStableFunc(idx, func(a, b int) int {
			switch {
			case xs[a] < xs[b]:
				return -1
			case xs[a] > xs[b]:
				return 1
			}
			return 0
		})
		sx, sy := make([]float64, len(xs)), make([]float64, len(ys))
		for i, j := range idx {
			sx[i], sy[i] = xs[j], ys[j]
		}
		fitted := loessFit(sx, sy, bandwidth)
		for i := range sx {
			if i > 0 && sx[i] == sx[i-1] {
				continue
			}
			r := Row{}
			for j, g := range groupby {
				r[g.Name] = keys[k][j]
			}
			r[as[0]], r[as[1]] = sx[i], fitted[i]
			out = append(out, r)
		}
	}
	return out, nil
}
