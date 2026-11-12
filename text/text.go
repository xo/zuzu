// Package text measures text for the layout of zuzu (D11).
//
// The okage peer writes the API that measures and shapes text as okage does,
// when zuzu needs it. Until then, Estimate measures a string from the advance
// widths of the bundled IBM Plex Sans (D16), with no shaping and no kerning.
// The layout depends on the interface Measurer, so that the measurement of
// okage can take its place.
//
// A Fallback is a font for the characters that IBM Plex Sans does not
// have. The package text/emoji gives one with the emoji of Noto Color
// Emoji (D32). A character that no font has draws nothing and measures as
// nothing.
package text

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	gotext "github.com/go-text/typesetting/font"
)

// fonts holds the bundled weights of IBM Plex Sans, upright and italic,
// and their license.
//
//go:embed fonts/IBMPlexSans-Regular.ttf fonts/IBMPlexSans-Medium.ttf fonts/IBMPlexSans-SemiBold.ttf
//go:embed fonts/IBMPlexSans-Italic.ttf fonts/IBMPlexSans-MediumItalic.ttf fonts/IBMPlexSans-SemiBoldItalic.ttf
//go:embed fonts/LICENSE.txt
var fonts embed.FS

// Family is the family of the bundled font.
const Family = "IBM Plex Sans"

// EmojiFamily is the family of the font of the package text/emoji. The SVG
// writer names it for a text that holds a character that IBM Plex Sans
// does not have, so that a viewer draws an emoji in color (D32).
const EmojiFamily = "Noto Color Emoji"

// Fallback is a font for the characters that IBM Plex Sans does not have,
// such as the font of the package text/emoji (D32). It is safe for
// concurrent use, and it parses its font once, on its first use.
type Fallback struct {
	family string
	data   []byte
	parse  func() (*gotext.Font, error)
}

// NewFallback returns a fallback of a family from the bytes of a font
// file, TrueType or OpenType. The bytes must not change after the call.
func NewFallback(family string, data []byte) *Fallback {
	return &Fallback{family: family, data: data, parse: sync.OnceValues(func() (*gotext.Font, error) {
		ft, err := gotext.ParseTTF(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parsing the font %s: %w", family, err)
		}
		return ft.Font, nil
	})}
}

// Family returns the family of the font.
func (f *Fallback) Family() string {
	return f.family
}

// Data returns the bytes of the font file, so that a renderer such as
// okage can load it. Do not change them.
func (f *Fallback) Data() []byte {
	return f.data
}

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
// a renderer draws by a little. Each family measures as IBM Plex Sans. A
// character that the bundled font does not have measures in the first
// fallback that has it, and as nothing when no fallback has it. It is safe
// for concurrent use, and it keeps the widths it measured.
type Estimate struct {
	mu    sync.Mutex
	cache map[key]float64
	// faces are the faces of this Estimate. A face of go-text keeps a cache
	// of its own that is not safe for concurrent use, so each Estimate
	// makes its faces from the shared fonts and uses them under mu.
	faces []face
	// fallbacks are the fallbacks of NewEstimate, and fallbackFaces their
	// faces, which the first use makes. A fallback that does not parse
	// has a nil face.
	fallbacks     []*Fallback
	fallbackFaces []*gotext.Face
}

// key is a cached measurement.
type key struct {
	font Font
	s    string
}

// maxCache is the number of widths that an Estimate keeps before it starts
// again.
const maxCache = 1 << 14

// NewEstimate returns an Estimate, with the fallbacks for the characters
// that the bundled font does not have, in order.
func NewEstimate(fallbacks ...*Fallback) *Estimate {
	return &Estimate{cache: make(map[key]float64), fallbacks: fallbacks}
}

// Width returns the width of s in the font, in pixels.
func (e *Estimate) Width(f Font, s string) float64 {
	k := key{f, s}
	e.mu.Lock()
	defer e.mu.Unlock()
	if w, ok := e.cache[k]; ok {
		return w
	}
	if !e.load() {
		return 0
	}
	w := measure(e.faces, e.fallbackFaces, f, s)
	if len(e.cache) >= maxCache {
		clear(e.cache)
	}
	e.cache[k] = w
	return w
}

// Missing returns the characters of s that neither the bundled font nor a
// fallback has, each once, in the order of s. Such a character draws
// nothing. A character that draws nothing in any font, such as a space, a
// control character, a joiner or a selector of a variation, is never
// missing.
func (e *Estimate) Missing(s string) []rune {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.load() {
		return nil
	}
	var out []rune
	for _, r := range s {
		if invisible(r) || has(e.faces[0].face, r) || fallbackOf(e.fallbackFaces, r) != nil {
			continue
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// Split splits s into pieces that each draw in one kind of font: the
// characters of the bundled font, or a sequence of the characters of the
// fallbacks with the characters that join them, such as a joiner, a
// selector of a variation or a modifier of skin tone. okage falls back to
// another font for one character at a time, and it draws a sequence of
// emoji, such as a flag, as one glyph only when it shapes the sequence on
// its own, so a renderer shapes each piece apart (D32). A string with no
// character of a fallback is one piece.
func (e *Estimate) Split(s string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.load() || len(e.fallbackFaces) == 0 {
		return []string{s}
	}
	base := e.faces[0].face
	var out []string
	start, inFallback, joined := 0, false, false
	for i, r := range s {
		var fallback bool
		switch {
		case r == 0x200d || unicode.Is(unicode.Variation_Selector, r) || (inFallback && invisible(r) && !unicode.IsSpace(r)):
			fallback = inFallback
		case has(base, r) && (!inFallback || !joined):
			fallback = keycap(s[i:], base, e.fallbackFaces)
		default:
			fallback = true
		}
		if i > 0 && fallback != inFallback {
			out = append(out, s[start:i])
			start = i
		}
		inFallback, joined = fallback, r == 0x200d
	}
	return append(out, s[start:])
}

// load makes the faces of the Estimate once. It reports false when the
// bundled font does not parse.
func (e *Estimate) load() bool {
	if e.faces != nil {
		return true
	}
	fonts, err := parsed()
	if err != nil {
		return false
	}
	for _, pf := range fonts {
		e.faces = append(e.faces, face{gotext.NewFace(pf.font), pf.weight, pf.italic})
	}
	for _, fb := range e.fallbacks {
		var fc *gotext.Face
		if ft, err := fb.parse(); err == nil {
			fc = gotext.NewFace(ft)
		}
		e.fallbackFaces = append(e.fallbackFaces, fc)
	}
	return true
}

// keycap reports whether s starts with a keycap that a fallback draws: a
// digit, # or *, a selector of a variation or none, and the combining
// keycap, which the bundled font does not have.
func keycap(s string, base *gotext.Face, fallbacks []*gotext.Face) bool {
	if s == "" || !strings.ContainsRune("0123456789#*", rune(s[0])) {
		return false
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(s[1:], "️"), "︎")
	return strings.HasPrefix(rest, "⃣") && !has(base, 0x20e3) && fallbackOf(fallbacks, 0x20e3) != nil
}

// invisible reports whether a character draws nothing in any font: white
// space, a control character, a character of format, such as a joiner or a
// tag, or a selector of a variation.
func invisible(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) ||
		unicode.Is(unicode.Variation_Selector, r)
}

// has reports whether a face has a glyph for a character.
func has(fc *gotext.Face, r rune) bool {
	if fc == nil {
		return false
	}
	_, ok := fc.NominalGlyph(r)
	return ok
}

// fallbackOf returns the first face of fallbacks that has a glyph for a
// character, or nil.
func fallbackOf(fallbacks []*gotext.Face, r rune) *gotext.Face {
	for _, fc := range fallbacks {
		if has(fc, r) {
			return fc
		}
	}
	return nil
}

// face is one weight of the bundled font, upright or italic, as a face that
// measures.
type face struct {
	face   *gotext.Face
	weight int
	italic bool
}

// parsedFont is one parsed weight of the bundled font. A font of go-text does
// not change after it is parsed, so the parsed fonts are shared.
type parsedFont struct {
	font   *gotext.Font
	weight int
	italic bool
}

// parsed parses the bundled weights once.
var parsed = sync.OnceValues(func() ([]parsedFont, error) {
	var out []parsedFont
	for _, f := range []struct {
		file   string
		weight int
		italic bool
	}{
		{"IBMPlexSans-Regular.ttf", 400, false}, {"IBMPlexSans-Medium.ttf", 500, false}, {"IBMPlexSans-SemiBold.ttf", 600, false},
		{"IBMPlexSans-Italic.ttf", 400, true}, {"IBMPlexSans-MediumItalic.ttf", 500, true}, {"IBMPlexSans-SemiBoldItalic.ttf", 600, true},
	} {
		b, err := fonts.ReadFile("fonts/" + f.file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.file, err)
		}
		ft, err := gotext.ParseTTF(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.file, err)
		}
		out = append(out, parsedFont{ft.Font, f.weight, f.italic})
	}
	return out, nil
})

// nearest returns the bundled face of a style, upright or italic, with the
// weight closest to a weight.
func nearest(fs []face, weight int, italic bool) *gotext.Face {
	var best *face
	for i := range fs {
		f := &fs[i]
		if f.italic != italic {
			continue
		}
		if best == nil || abs(f.weight-weight) < abs(best.weight-weight) {
			best = f
		}
	}
	if best == nil {
		best = &fs[0]
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
// parser of go-text, which okage uses too. A rune that the font does not
// hold measures in the first fallback that holds it, and as nothing when
// none does, because it draws nothing.
//
// A fallback draws a sequence of emoji as one glyph, which this sum does
// not shape. So a rune that joins the glyph before it measures as nothing:
// the rune after a zero width joiner, a modifier of skin tone, and the
// second of a pair of regional indicators that make a flag. A keycap after
// a digit, # or * takes the place of that character. A character of format
// and a selector of a variation that the font does not hold measure as
// nothing.
func measure(fs []face, fallbacks []*gotext.Face, f Font, s string) float64 {
	if len(fs) == 0 || f.Size <= 0 || s == "" {
		return 0
	}
	ft := nearest(fs, f.Weight, f.Italic)
	advance := func(fc *gotext.Face, r rune) float64 {
		gid, _ := fc.NominalGlyph(r)
		return float64(fc.HorizontalAdvance(gid)) / float64(fc.Upem()) * f.Size
	}
	var width float64
	// emoji says that the last glyph came from a fallback, joined that a
	// joiner follows it, and flag that it opens a flag.
	emoji, joined, flag := false, false, false
	var last rune
	for _, r := range s {
		if r == 0x200d {
			joined = emoji
			continue
		}
		if has(ft, r) && (!emoji || !joined) {
			width += advance(ft, r)
			emoji, joined, flag, last = false, false, false, r
			continue
		}
		fc := fallbackOf(fallbacks, r)
		switch {
		case invisible(r):
			continue
		case fc == nil:
			emoji, joined, flag = false, false, false
			continue
		case emoji && (joined || (r >= 0x1f3fb && r <= 0x1f3ff) || (flag && regional(r))):
			joined, flag = false, false
			continue
		case r == 0x20e3 && (last >= '0' && last <= '9' || last == '#' || last == '*'):
			width -= advance(ft, last)
		}
		width += advance(fc, r)
		emoji, joined, flag, last = true, false, regional(r), r
	}
	return width
}

// regional reports whether a rune is a regional indicator, a letter of a
// flag.
func regional(r rune) bool {
	return r >= 0x1f1e6 && r <= 0x1f1ff
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
