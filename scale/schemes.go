package scale

import (
	"fmt"
	"math"
	"strings"

	"github.com/xo/zuzu/internal/color"
)

// Scheme is a color scheme of Vega, such as viridis or tableau10, or a list
// of colors that a spec gives as its scheme (D46).
type Scheme struct {
	// Colors are the colors of the scheme.
	Colors []string
	// Continuous says that Colors are stops spread evenly from 0 to 1, and
	// that the scheme interpolates between them, as viridis does. Else the
	// scheme is a list of discrete colors, as tableau10 is.
	Continuous bool
	// Interpolate is the color space that a continuous scheme interpolates
	// in, as Options.Interpolate names it. A named scheme of Vega
	// interpolates in RGB, which "" also names.
	Interpolate string
}

// NamedScheme returns the scheme of Vega with a name in any case, such as
// viridis or tableau10, and whether Vega has it.
func NamedScheme(name string) (Scheme, bool) {
	name = strings.ToLower(name)
	if cs, ok := discreteSchemes[name]; ok {
		return Scheme{Colors: hexes(cs)}, true
	}
	if cs, ok := continuousSchemes[name]; ok {
		return Scheme{Colors: hexes(cs), Continuous: true}, true
	}
	return Scheme{}, false
}

// At returns the color of a continuous scheme at t, from 0 to 1. The stop
// below t and the stop above it mix in the color space of the scheme. A t
// outside 0 to 1 runs on from the first or the last two stops.
func (s Scheme) At(t float64) string {
	n := len(s.Colors) - 1
	switch {
	case n < 0:
		return ""
	case n == 0:
		return s.Colors[0]
	}
	// The conversion keeps the product from fusing with the subtraction
	// below, as in color.Mix.
	t = float64(t * float64(n))
	i := max(0, min(n-1, int(math.Floor(t))))
	a, _ := color.Parse(s.Colors[i])
	b, _ := color.Parse(s.Colors[i+1])
	return color.Mix(a, b, t-float64(i), s.Interpolate).Hex()
}

// Sample returns count colors of a continuous scheme, at 1/(count+1),
// 2/(count+1) and so on, so that neither end of the scheme is among them.
// This is how Vega takes the colors of a discrete scale from a continuous
// scheme. at maps t, from 0 to 1, to a color.
func Sample(at func(float64) string, count int) []any {
	out := make([]any, max(count, 0))
	for i := range out {
		out[i] = at(float64(i+1) / float64(count+1))
	}
	return out
}

// hexes writes colors of the form 0xrrggbb as #rrggbb.
func hexes(cs []uint32) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("#%06x", c)
	}
	return out
}
