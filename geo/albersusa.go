package geo

// albersUSA joins three conic equal-area projections: the lower 48 states,
// Alaska smaller and below them on the left, and Hawaii beside Alaska. Each
// part clips to its own rectangle, so that a point shows in one part only.
type albersUSA struct {
	lower48, alaska, hawaii *Projection
}

func newAlbersUSA() *albersUSA {
	a := &albersUSA{}
	a.lower48 = &Projection{typ: "albers"}
	a.lower48.albers()
	a.alaska = &Projection{typ: "conicEqualArea"}
	a.alaska.conicEqualArea()
	a.alaska.setRotate(154, 0, 0)
	a.alaska.setCenter(-2, 58.5)
	a.alaska.setParallels(55, 65)
	a.hawaii = &Projection{typ: "conicEqualArea"}
	a.hawaii.conicEqualArea()
	a.hawaii.setRotate(157, 0, 0)
	a.hawaii.setCenter(-3, 19.9)
	a.hawaii.setParallels(8, 18)
	a.setScale(1070)
	return a
}

func (a *albersUSA) setScale(k float64) {
	a.lower48.setScale(k)
	a.alaska.setScale(k * 0.35)
	a.hawaii.setScale(k)
	a.setTranslate(a.lower48.x, a.lower48.y)
}

func (a *albersUSA) setTranslate(x, y float64) {
	k := a.lower48.k
	a.lower48.setTranslate(x, y)
	a.lower48.setClipExtent(&rectClip{x - 0.455*k, y - 0.238*k, x + 0.455*k, y + 0.238*k})
	a.alaska.setTranslate(x-0.307*k, y+0.201*k)
	a.alaska.setClipExtent(&rectClip{x - 0.425*k + epsilon, y + 0.120*k + epsilon, x - 0.214*k - epsilon, y + 0.234*k - epsilon})
	a.hawaii.setTranslate(x-0.205*k, y+0.212*k)
	a.hawaii.setClipExtent(&rectClip{x - 0.214*k + epsilon, y + 0.166*k + epsilon, x - 0.115*k - epsilon, y + 0.234*k - epsilon})
}

func (a *albersUSA) setPrecision(d float64) {
	a.lower48.delta2 = d * d
	a.alaska.delta2 = d * d
	a.hawaii.delta2 = d * d
}

// project returns the place of a point in the first part that shows it.
func (a *albersUSA) project(lon, lat float64) (float64, float64, bool) {
	for _, p := range []*Projection{a.lower48, a.alaska, a.hawaii} {
		var c pointCatch
		s := p.stream(&c)
		s.point(lon, lat)
		if c.ok {
			return c.x, c.y, true
		}
	}
	return 0, 0, false
}

func (a *albersUSA) stream(sink stream) stream {
	return multiplex{a.lower48.stream(sink), a.alaska.stream(sink), a.hawaii.stream(sink)}
}

// pointCatch keeps the last point that it receives.
type pointCatch struct {
	x, y float64
	ok   bool
}

func (c *pointCatch) point(x, y float64) { c.x, c.y, c.ok = x, y, true }
func (c *pointCatch) lineStart()         {}
func (c *pointCatch) lineEnd()           {}
func (c *pointCatch) polygonStart()      {}
func (c *pointCatch) polygonEnd()        {}
func (c *pointCatch) sphere()            {}
