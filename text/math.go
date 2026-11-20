package text

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/texmath"
	gotext "github.com/xo/okage/typesetting/font"
)

// MathFont is a font of math, with an OpenType table MATH, for the math of
// KaTeX in a label of Mermaid (D36). The package text/math gives STIX Two
// Math. A face of go-text is not safe for concurrent use, so a MathFont
// lays out one formula at a time.
type MathFont struct {
	data []byte

	mu   sync.Mutex
	once sync.Once
	face *gotext.Face
	err  error
}

// NewMathFont returns a font of math from the data of an OpenType font
// with a table MATH. The font is parsed on its first use.
func NewMathFont(data []byte) *MathFont {
	return &MathFont{data: data}
}

// Formula is a formula of TeX laid out, in pixels. Its origin is the left
// end of its baseline. Height is its extent above the baseline, and Depth
// its extent below it.
type Formula struct {
	Width, Height, Depth float64
	// Paths are the outlines of the formula, in the syntax of the d
	// attribute of SVG, relative to its origin.
	Paths []MathPath
}

// MathPath is an outline of a formula and its color. An empty color is the
// color of the text around the formula.
type MathPath struct {
	Path  string
	Color string
}

// noColor is the color that the layout gives to an item outside a node of
// \color, so that the item takes the color of the text. No formula sets a
// color with an alpha of 0.
var noColor = okage.Color{}

// Layout lays out a formula of TeX at a size in pixels, in the display
// style, as Mermaid 12.3 asks KaTeX to (D36). A formula with no color
// gives one path. A formula with \color gives a path for each item, so
// that each keeps its color. An error of the parse wraps
// okage.ErrTeXMathSyntax, and MathCommand names the command of it.
func (f *MathFont) Layout(tex string, size float64) (*Formula, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	face, err := f.load()
	if err != nil {
		return nil, err
	}
	box, err := texmath.LayoutTeX(tex, face, size, texmath.Display, texmath.WithColor(noColor))
	if err != nil {
		return nil, err
	}
	out := &Formula{Width: box.Width, Height: box.Height, Depth: box.Depth}
	colored := false
	for _, it := range box.Items {
		if it.Color != noColor {
			colored = true
			break
		}
	}
	if !colored {
		if p := box.Path(); p != nil {
			out.Paths = []MathPath{{Path: svgPath(p)}}
		}
		return out, nil
	}
	for i, it := range box.Items {
		p := box.ItemPath(i)
		if p == nil {
			continue
		}
		c := ""
		if it.Color != noColor {
			c = hexColor(it.Color)
		}
		// Items of one color that follow each other share a path.
		if n := len(out.Paths); n > 0 && out.Paths[n-1].Color == c {
			out.Paths[n-1].Path += svgPath(p)
			continue
		}
		out.Paths = append(out.Paths, MathPath{Path: svgPath(p), Color: c})
	}
	return out, nil
}

// load parses the font once.
func (f *MathFont) load() (*gotext.Face, error) {
	f.once.Do(func() {
		ft, err := gotext.ParseTTF(bytes.NewReader(f.data))
		if err != nil {
			f.err = err
			return
		}
		f.face = ft
	})
	return f.face, f.err
}

// MathCommand returns the command of TeX that an error of Layout names,
// such as \phase, or empty when it names none.
func MathCommand(err error) string {
	if se, ok := errors.AsType[*texmath.SyntaxError](err); ok {
		return se.Command
	}
	return ""
}

// svgPath writes a path in the syntax of the d attribute of SVG, with two
// places after the point, so that the same formula writes the same bytes.
func svgPath(p *path.Path) string {
	var b strings.Builder
	pts := p.Points()
	i := 0
	cmd := func(c byte, n int) {
		b.WriteByte(c)
		for k := range n {
			if k > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(num(pts[i].X))
			b.WriteByte(',')
			b.WriteString(num(pts[i].Y))
			i++
		}
	}
	for _, v := range p.Verbs() {
		switch v {
		case path.PathVerbMove:
			cmd('M', 1)
		case path.PathVerbLine:
			cmd('L', 1)
		case path.PathVerbQuad:
			cmd('Q', 2)
		case path.PathVerbCubic:
			cmd('C', 3)
		case path.PathVerbClose:
			b.WriteByte('Z')
		}
	}
	return b.String()
}

// num writes a coordinate with at most two places after the point.
func num(v float32) string {
	s := strconv.FormatFloat(float64(v), 'f', 2, 32)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

// hexColor writes a color of the layout as #rrggbb, or as rgba() when it is
// not opaque.
func hexColor(c okage.Color) string {
	u := c.ToColorU8()
	if u.IsOpaque() {
		return "#" + hex2(u.Red()) + hex2(u.Green()) + hex2(u.Blue())
	}
	return "rgba(" + strconv.Itoa(int(u.Red())) + "," + strconv.Itoa(int(u.Green())) + "," +
		strconv.Itoa(int(u.Blue())) + "," + strconv.FormatFloat(float64(u.Alpha())/255, 'f', 3, 64) + ")"
}

// hex2 writes a byte as two hex digits.
func hex2(v uint8) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>4], digits[v&15]})
}

// mathRE matches a run of math of KaTeX in a label, as Mermaid 12.3 finds
// it: a text between two $$ on one line.
var mathRE = regexp.MustCompile(`\$\$(.*?)\$\$`)

// Run is a part of a line of a label: text, or the TeX of a formula.
type Run struct {
	Text string
	Math bool
}

// SplitMath splits a line of a label into runs of text and runs of math,
// where Mermaid 12.3 finds math: each text between two $$. The runs of
// math hold the TeX without the $$. A line with no math is one run of
// text.
func SplitMath(s string) []Run {
	locs := mathRE.FindAllStringSubmatchIndex(s, -1)
	if len(locs) == 0 {
		return []Run{{Text: s}}
	}
	var out []Run
	at := 0
	for _, l := range locs {
		if l[0] > at {
			out = append(out, Run{Text: s[at:l[0]]})
		}
		out = append(out, Run{Text: s[l[2]:l[3]], Math: true})
		at = l[1]
	}
	if at < len(s) {
		out = append(out, Run{Text: s[at:]})
	}
	return out
}

// HasMath reports whether a text holds math of KaTeX.
func HasMath(s string) bool {
	return mathRE.MatchString(s)
}

// Piece is a run of a line of a label laid out. X is its offset from the
// start of the line, and W its width. A piece of math holds its formula,
// whose baseline is Baseline below the middle of the line.
type Piece struct {
	Run

	X, W     float64
	Formula  *Formula
	Baseline float64
}

// MathLine is a line of a label with math, laid out as Mermaid 12.3 lays
// it out: a row of its runs, each centered on the middle of the line, with
// no wrap. Height is the larger of the height of a line of text, when the
// line has text, and the height and the depth of its tallest formula. So a
// line of math alone is as high as its formula (D36).
type MathLine struct {
	Pieces []Piece
	Width  float64
	Height float64
}

// LayMathLine lays out a line of a label with runs of math in a font, with
// a line of text lineHeight high. Mermaid draws each run as an item of a
// row, which drops the spaces at its ends, so a run of text loses them, and
// a run of spaces alone draws nothing. A thin space of 3/18 of the size of
// the font stands between a run of text and a run of math, so that the two
// do not touch (docs/STYLE.md). A run of text measures with m, and a run
// of math lays out in the font of math of m at the size of the font.
// A run of math that does not lay out gives its error, and the line is
// nil.
func LayMathLine(m Measurer, f Font, runs []Run, lineHeight float64) (*MathLine, error) {
	mf := MathOf(m)
	l := &MathLine{}
	for _, r := range runs {
		if !r.Math {
			r.Text = strings.TrimSpace(r.Text)
			if r.Text == "" {
				continue
			}
		}
		if n := len(l.Pieces); n > 0 && l.Pieces[n-1].Math != r.Math {
			l.Width += f.Size * MathGap
		}
		p := Piece{Run: r, X: l.Width}
		if r.Math {
			if mf == nil {
				return nil, ErrNoMathFont
			}
			fm, err := mf.Layout(r.Text, f.Size)
			if err != nil {
				return nil, err
			}
			p.Formula, p.W = fm, fm.Width
			p.Baseline = (fm.Height - fm.Depth) / 2
			l.Height = max(l.Height, fm.Height+fm.Depth)
		} else {
			p.W = m.Width(f, r.Text)
			l.Height = max(l.Height, lineHeight)
		}
		l.Width += p.W
		l.Pieces = append(l.Pieces, p)
	}
	return l, nil
}

// MathGap is the space between a run of text and a run of math, in ems of
// the font of the text: a thin space of TeX.
const MathGap = 3.0 / 18

// ErrNoMathFont is the error of a run of math with no font of math, which
// zuzu.WithMath gives.
var ErrNoMathFont = errors.New("no font of math")
