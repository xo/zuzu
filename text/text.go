// Package text measures text for the layout of zuzu (D11).
//
// The okage peer writes the API that measures and shapes text as okage does,
// when zuzu needs it. Until then, Estimate measures a string from the advance
// widths of the bundled IBM Plex Sans (D16), with no shaping and no kerning.
// The layout depends on the interface Measurer, so that the measurement of
// okage can take its place.
package text

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"

	gotext "github.com/go-text/typesetting/font"
)

// fonts holds the bundled weights of IBM Plex Sans and their license.
//
//go:embed fonts/IBMPlexSans-Regular.ttf fonts/IBMPlexSans-Medium.ttf fonts/IBMPlexSans-SemiBold.ttf fonts/LICENSE.txt
var fonts embed.FS

// Family is the family of the bundled font.
const Family = "IBM Plex Sans"

// Fonts returns the bundled font files and their license, so that a
// renderer such as okage can load them. The files are in the folder fonts.
func Fonts() fs.FS {
	return fonts
}

// Font is a font at a size.
type Font struct {
	Family string  // the family, such as IBM Plex Sans
	Size   float64 // the size in pixels
	Weight int     // the weight, from 100 to 900, where 400 is normal
	Italic bool
}

// Measurer measures the width of a string in a font, in pixels.
type Measurer interface {
	Width(f Font, s string) float64
}

// Weight reads a font weight as Vega writes it: a number, or normal or bold.
// Anything else is normal.
func Weight(s string) int {
	switch strings.TrimSpace(s) {
	case "", "normal":
		return 400
	case "bold":
		return 700
	case "lighter":
		return 300
	case "bolder":
		return 700
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 400
	}
	return n
}

// Estimate measures text from the advance widths of the bundled font. It
// does not shape or kern, so a measured width can differ from the width that
// a renderer draws by a little. Each family measures as IBM Plex Sans. It is
// safe for concurrent use, and it keeps the widths it measured.
type Estimate struct {
	mu    sync.Mutex
	cache map[key]float64
	// faces are the faces of this Estimate. A face of go-text keeps a cache
	// of its own that is not safe for concurrent use, so each Estimate
	// makes its faces from the shared fonts and uses them under mu.
	faces []face
}

// key is a cached measurement.
type key struct {
	font Font
	s    string
}

// maxCache is the number of widths that an Estimate keeps before it starts
// again.
const maxCache = 1 << 14

// NewEstimate returns an Estimate.
func NewEstimate() *Estimate {
	return &Estimate{cache: make(map[key]float64)}
}

// Width returns the width of s in the font, in pixels.
func (e *Estimate) Width(f Font, s string) float64 {
	k := key{f, s}
	e.mu.Lock()
	defer e.mu.Unlock()
	if w, ok := e.cache[k]; ok {
		return w
	}
	if e.faces == nil {
		fonts, err := parsed()
		if err != nil {
			return 0
		}
		for _, pf := range fonts {
			e.faces = append(e.faces, face{gotext.NewFace(pf.font), pf.weight})
		}
	}
	w := measure(e.faces, f, s)
	if len(e.cache) >= maxCache {
		clear(e.cache)
	}
	e.cache[k] = w
	return w
}

// face is one weight of the bundled font, as a face that measures.
type face struct {
	face   *gotext.Face
	weight int
}

// parsedFont is one parsed weight of the bundled font. A font of go-text does
// not change after it is parsed, so the parsed fonts are shared.
type parsedFont struct {
	font   *gotext.Font
	weight int
}

// parsed parses the bundled weights once.
var parsed = sync.OnceValues(func() ([]parsedFont, error) {
	var out []parsedFont
	for _, f := range []struct {
		file   string
		weight int
	}{{"IBMPlexSans-Regular.ttf", 400}, {"IBMPlexSans-Medium.ttf", 500}, {"IBMPlexSans-SemiBold.ttf", 600}} {
		b, err := fonts.ReadFile("fonts/" + f.file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.file, err)
		}
		ft, err := gotext.ParseTTF(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.file, err)
		}
		out = append(out, parsedFont{ft.Font, f.weight})
	}
	return out, nil
})

// nearest returns the bundled weight closest to a weight.
func nearest(fs []face, weight int) *gotext.Face {
	best := fs[0]
	for _, f := range fs[1:] {
		if abs(f.weight-weight) < abs(best.weight-weight) {
			best = f
		}
	}
	return best.face
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// measure sums the advance widths of the glyphs of s, read with the font
// parser of go-text, which okage uses too. A rune that the font does not hold
// measures as the glyph that the font draws in its place.
func measure(fs []face, f Font, s string) float64 {
	if len(fs) == 0 || f.Size <= 0 || s == "" {
		return 0
	}
	ft := nearest(fs, f.Weight)
	upm := float64(ft.Upem())
	var units float64
	for _, r := range s {
		gid, _ := ft.NominalGlyph(r)
		units += float64(ft.HorizontalAdvance(gid))
	}
	return units / upm * f.Size
}

// Ellipsis is the text that Truncate puts at the end of a cut string, as
// Vega does.
const Ellipsis = "…"

// Truncate cuts s to the longest start that fits in limit with the
// ellipsis after it, as Vega cuts a text to its limit. A string that fits is
// returned as it is. A limit of zero or less is no limit.
func Truncate(m Measurer, f Font, s string, limit float64) string {
	if limit <= 0 || m.Width(f, s) <= limit {
		return s
	}
	if m.Width(f, Ellipsis) > limit {
		return ""
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if m.Width(f, string(runes[:mid])+Ellipsis) <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.TrimRightFunc(string(runes[:lo]), func(r rune) bool { return r == ' ' }) + Ellipsis
}
