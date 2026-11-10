package geo

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// ErrProjection is the error of a projection that zuzu does not know, or
// of a parameter of a projection that it cannot read.
var ErrProjection = errors.New("bad projection")

// Projection maps longitude and latitude, in degrees, to the plane of a
// chart, and draws GeoJSON objects as paths. Its parameters follow the
// projections of Vega: scale, translate, center, rotate, clipAngle,
// clipExtent, parallels, precision, reflectX and reflectY.
type Projection struct {
	// Radius is the radius of the circle that a path draws for a point.
	Radius float64

	typ string
	// usa is set for albersUsa, which joins three projections.
	usa *albersUSA

	raw    rawFunc
	rawAt  func(y0, y1 float64) rawFunc
	y0, y1 float64

	k, x, y            float64
	lambda, phi        float64
	dLambda, dPhi, dGa float64
	alpha              float64
	sx, sy             float64
	theta              float64
	hasTheta           bool
	preclip            *clipper
	extent             *rectClip
	delta2             float64

	// The extent that the caller set on a Mercator projection, which also
	// clips to the square of the world.
	mercator   int
	userExtent *rectClip

	// Derived by recenter.
	rot         rotation
	projectXY   projectFunc
	transformXY func(x, y float64) (float64, float64)
}

const (
	notMercator = iota
	plainMercator
	transverse
)

// New returns a projection of a type of Vega, such as mercator,
// albersUsa or orthographic, with the defaults of that type. An empty type
// is mercator.
func New(typ string) (*Projection, error) {
	if typ == "" {
		typ = "mercator"
	}
	p := &Projection{typ: typ, Radius: 4.5}
	p.init(equirectangularRaw)
	switch strings.ToLower(typ) {
	case "albers":
		p.albers()
	case "albersusa":
		p.usa = newAlbersUSA()
	case "azimuthalequalarea":
		p.init(azimuthalEqualAreaRaw)
		p.setScale(124.75)
		p.setClipAngle(180 - 1e-3)
	case "azimuthalequidistant":
		p.init(azimuthalEquidistantRaw)
		p.setScale(79.4188)
		p.setClipAngle(180 - 1e-3)
	case "conicconformal":
		p.conic(conicConformalRaw)
		p.setScale(109.5)
		p.setParallels(30, 30)
	case "conicequalarea":
		p.conicEqualArea()
	case "conicequidistant":
		p.conic(conicEquidistantRaw)
		p.setScale(131.154)
		p.setCenter(0, 13.9389)
	case "equalearth":
		p.init(equalEarthRaw)
		p.setScale(177.158)
	case "equirectangular":
		p.setScale(152.63)
	case "gnomonic":
		p.init(gnomonicRaw)
		p.setScale(144.049)
		p.setClipAngle(60)
	case "mercator":
		p.init(mercatorRaw)
		p.mercator = plainMercator
		p.reclip()
		p.setScale(961 / tau)
	case "mollweide":
		p.init(mollweideRaw)
		p.setScale(169.529)
	case "naturalearth1":
		p.init(naturalEarth1Raw)
		p.setScale(175.295)
	case "orthographic":
		p.init(orthographicRaw)
		p.setScale(249.5)
		p.setClipAngle(90 + epsilon)
	case "stereographic":
		p.init(stereographicRaw)
		p.setScale(250)
		p.setClipAngle(142)
	case "transversemercator":
		p.init(transverseMercatorRaw)
		p.mercator = transverse
		p.reclip()
		p.setRotate(0, 0, 90)
		p.setScale(159.155)
	default:
		return nil, fmt.Errorf("the type %q: %w", typ, ErrProjection)
	}
	return p, nil
}

// Type returns the type of the projection, as the spec names it.
func (p *Projection) Type() string { return p.typ }

// Set sets a parameter of the projection from the value of a spec. A
// parameter that the type does not have is left out, as in Vega.
func (p *Projection) Set(name string, v any) error {
	nums := func(n int) ([]float64, error) {
		l, ok := v.([]any)
		if !ok || len(l) < n {
			return nil, fmt.Errorf("the %s %v: %w", name, v, ErrProjection)
		}
		out := make([]float64, len(l))
		for i, x := range l {
			out[i] = Number(x)
		}
		return out, nil
	}
	if p.usa != nil {
		switch name {
		case "scale":
			p.setScale(Number(v))
		case "translate":
			t, err := nums(2)
			if err != nil {
				return err
			}
			p.setTranslate(t[0], t[1])
		case "precision":
			p.usa.setPrecision(Number(v))
		}
		return nil
	}
	switch name {
	case "scale":
		p.setScale(Number(v))
	case "translate":
		t, err := nums(2)
		if err != nil {
			return err
		}
		p.setTranslate(t[0], t[1])
	case "center":
		c, err := nums(2)
		if err != nil {
			return err
		}
		p.setCenter(c[0], c[1])
	case "rotate":
		r, err := nums(2)
		if err != nil {
			return err
		}
		g := 0.0
		if len(r) > 2 {
			g = r[2]
		}
		if p.mercator == transverse {
			g += 90
		}
		p.setRotate(r[0], r[1], g)
	case "parallels":
		r, err := nums(2)
		if err != nil {
			return err
		}
		p.setParallels(r[0], r[1])
	case "clipAngle":
		if v == nil {
			p.setClipAngle(0)
		} else {
			p.setClipAngle(Number(v))
		}
	case "clipExtent":
		if v == nil {
			p.setClipExtent(nil)
			return nil
		}
		l, ok := v.([]any)
		if !ok || len(l) < 2 {
			return fmt.Errorf("the clipExtent %v: %w", v, ErrProjection)
		}
		a, _ := l[0].([]any)
		b, _ := l[1].([]any)
		if len(a) < 2 || len(b) < 2 {
			return fmt.Errorf("the clipExtent %v: %w", v, ErrProjection)
		}
		p.setClipExtent(&rectClip{Number(a[0]), Number(a[1]), Number(b[0]), Number(b[1])})
	case "precision":
		d := Number(v)
		p.delta2 = d * d
	case "reflectX":
		p.sx = 1
		if truthy(v) {
			p.sx = -1
		}
		p.recenter()
	case "reflectY":
		p.sy = 1
		if truthy(v) {
			p.sy = -1
		}
		p.recenter()
	}
	return nil
}

// Project maps a point in degrees to the plane. It reports false for a
// point that the projection does not show, which only albersUsa leaves
// out.
func (p *Projection) Project(lon, lat float64) (float64, float64, bool) {
	if p.usa != nil {
		return p.usa.project(lon, lat)
	}
	l, f := p.rot.forward(lon*radians, lat*radians)
	x, y := p.projectXY(l, f)
	return x, y, true
}

// Fit sets the scale and the translation so that a GeoJSON object fills
// the extent from (x0, y0) to (x1, y1), as fitExtent in Vega.
func (p *Projection) Fit(x0, y0, x1, y1 float64, object any) {
	clip := p.clipExtent()
	p.setScale(150)
	p.setTranslate(0, 0)
	if clip != nil {
		p.setClipExtent(nil)
	}
	b := &boundsSink{}
	b.reset()
	streamObject(object, p.stream(b))
	w, h := x1-x0, y1-y0
	k := math.Min(w/(b.x1-b.x0), h/(b.y1-b.y0))
	x := x0 + (w-k*(b.x1+b.x0))/2
	y := y0 + (h-k*(b.y1+b.y0))/2
	p.setScale(150 * k)
	p.setTranslate(x, y)
	if clip != nil {
		p.setClipExtent(clip)
	}
}

// init sets the defaults that every projection shares.
func (p *Projection) init(raw rawFunc) {
	p.raw = raw
	p.k, p.x, p.y = 150, 480, 250
	p.sx, p.sy = 1, 1
	p.preclip = antimeridian
	p.delta2 = 0.5
	p.recenter()
}

func (p *Projection) conic(at func(y0, y1 float64) rawFunc) {
	p.rawAt = at
	p.y0, p.y1 = 0, math.Pi/3
	p.init(at(p.y0, p.y1))
}

func (p *Projection) conicEqualArea() {
	p.conic(conicEqualAreaRaw)
	p.setScale(155.424)
	p.setCenter(0, 33.6442)
}

func (p *Projection) albers() {
	p.conicEqualArea()
	p.setParallels(29.5, 45.5)
	p.setScale(1070)
	p.setTranslate(480, 250)
	p.setRotate(96, 0, 0)
	p.setCenter(-0.6, 38.7)
}

// recenter works out the transform from the raw projection to the plane.
func (p *Projection) recenter() {
	cx, cy := p.raw(p.lambda, p.phi)
	cx, cy = p.k*cx*p.sx, -p.k*cy*p.sy
	dx, dy := p.x-cx, p.y-cy
	k, sx, sy := p.k, p.sx, p.sy
	if p.alpha == 0 {
		p.transformXY = func(x, y float64) (float64, float64) {
			return dx + k*x*sx, dy - k*y*sy
		}
	} else {
		ca, sa := math.Cos(p.alpha), math.Sin(p.alpha)
		a, b := ca*k, sa*k
		p.transformXY = func(x, y float64) (float64, float64) {
			x *= sx
			y *= sy
			return a*x - b*y + dx, dy - b*x - a*y
		}
	}
	p.rot = rotation{p.dLambda, p.dPhi, p.dGa}
	raw, tr := p.raw, p.transformXY
	p.projectXY = func(lambda, phi float64) (float64, float64) {
		return tr(raw(lambda, phi))
	}
}

func (p *Projection) setScale(k float64) {
	if p.usa != nil {
		p.usa.setScale(k)
		return
	}
	p.k = k
	p.recenter()
	p.reclip()
}

func (p *Projection) setTranslate(x, y float64) {
	if p.usa != nil {
		p.usa.setTranslate(x, y)
		return
	}
	p.x, p.y = x, y
	p.recenter()
	p.reclip()
}

func (p *Projection) setCenter(lon, lat float64) {
	if p.mercator == transverse {
		lon, lat = -lat, lon
	}
	p.lambda = math.Mod(lon, 360) * radians
	p.phi = math.Mod(lat, 360) * radians
	p.recenter()
	p.reclip()
}

func (p *Projection) setRotate(l, f, g float64) {
	p.dLambda = math.Mod(l, 360) * radians
	p.dPhi = math.Mod(f, 360) * radians
	p.dGa = math.Mod(g, 360) * radians
	p.recenter()
}

func (p *Projection) setParallels(y0, y1 float64) {
	if p.rawAt == nil {
		return
	}
	p.y0, p.y1 = y0*radians, y1*radians
	p.raw = p.rawAt(p.y0, p.y1)
	p.recenter()
}

func (p *Projection) setClipAngle(a float64) {
	if a != 0 && !math.IsNaN(a) {
		p.theta, p.hasTheta = a*radians, true
		p.preclip = circleClip(p.theta)
		return
	}
	p.hasTheta = false
	p.preclip = antimeridian
}

// setClipExtent sets the rectangle that clips the projected geometry, or
// removes it if e is nil.
func (p *Projection) setClipExtent(e *rectClip) {
	if p.mercator != notMercator {
		p.userExtent = e
		p.reclip()
		return
	}
	p.extent = e
}

// clipExtent returns the rectangle that the caller set.
func (p *Projection) clipExtent() *rectClip {
	if p.mercator != notMercator {
		return p.userExtent
	}
	return p.extent
}

// reclip clips a Mercator projection to the square of the world about its
// center, which keeps the poles, at infinity, out of the path.
func (p *Projection) reclip() {
	if p.mercator == notMercator {
		return
	}
	k := math.Pi * p.k
	l, f := p.rot.invert(0, 0)
	l, f = p.rot.forward(l*degrees*radians, f*degrees*radians)
	tx, ty := p.projectXY(l, f)
	e := rectClip{tx - k, ty - k, tx + k, ty + k}
	if u := p.userExtent; u != nil {
		if p.mercator == plainMercator {
			e = rectClip{math.Max(tx-k, u.x0), u.y0, math.Min(tx+k, u.x1), u.y1}
		} else {
			e = rectClip{u.x0, math.Max(ty-k, u.y0), u.x1, math.Min(ty+k, u.y1)}
		}
	}
	p.extent = &e
}

func truthy(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case nil:
		return false
	case string:
		return v != ""
	}
	f := Number(v)
	return f != 0 && !math.IsNaN(f)
}

// stream returns the stream that projects a geometry in degrees to sink:
// it turns the globe, clips it, resamples and projects it, and clips the
// result to the extent.
func (p *Projection) stream(sink stream) stream {
	if p.usa != nil {
		return p.usa.stream(sink)
	}
	post := sink
	if p.extent != nil {
		post = p.extent.stream(sink)
	}
	s := p.preclip.stream(newResampler(p.projectXY, p.delta2, post))
	rot := p.rot
	s = &pointMap{next: s, f: rot.forward}
	return &pointMap{next: s, f: func(x, y float64) (float64, float64) {
		return x * radians, y * radians
	}}
}
