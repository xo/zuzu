package excalifont

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"strings"
	"testing"

	okagesvg "github.com/xo/okage/svg"
	gotext "github.com/xo/okage/typesetting/font"

	"github.com/xo/zuzu/text"
)

// missing are the characters of Latin-1 that Excalifont 1.000 does not
// have. A font of another version fails TestTheFontHasLatin1 until this
// list is updated.
const missing = "¤§¬²³µ¹"

// face parses the font with go-text, as text.Estimate does.
func face(t *testing.T) *gotext.Face {
	t.Helper()
	b := Font().Data()
	if len(b) < 4 || binary.BigEndian.Uint32(b) != 0x00010000 {
		t.Fatal("Data of the font is not a TrueType file")
	}
	f, err := gotext.ParseTTF(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("parsing the font: %v", err)
	}
	return f
}

// TestTheFontParses checks that the font decodes from WOFF2 into one
// TrueType face of Excalifont, of weight 400 and upright, which go-text and
// okage both read (D34).
func TestTheFontParses(t *testing.T) {
	t.Parallel()
	if got := Font().Family(); got != Family {
		t.Errorf("the family of the fallback is %q, want %q", got, Family)
	}
	d := face(t).Describe()
	if d.Family != Family || d.Aspect.Weight != gotext.WeightNormal || d.Aspect.Style != gotext.StyleNormal {
		t.Errorf("go-text describes the font as %+v, want %s, of weight 400 and upright", d, Family)
	}
	db := okagesvg.NewFontDatabase()
	ids := db.LoadFontData(Font().Data())
	if len(ids) != 1 {
		t.Fatalf("okage loaded %d faces, want 1", len(ids))
	}
	if got, _ := db.FaceName(ids[0], 1); got != Family {
		t.Errorf("okage reads the family %q, want %q", got, Family)
	}
}

// TestTheFontHoldsItsLicense checks that the license travels with the
// font, as the Open Font License asks (D16, D40): in LICENSE.txt beside
// it, and in the table name of the font, which a program that embeds the
// font holds.
func TestTheFontHoldsItsLicense(t *testing.T) {
	t.Parallel()
	const copyright = "Copyright (c) 2024 by Excalidraw."
	b, err := os.ReadFile("LICENSE.txt")
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.HasPrefix(s, copyright) || !strings.Contains(s, "SIL OPEN FONT LICENSE Version 1.1") {
		t.Error("LICENSE.txt is not the copyright of Excalifont and the Open Font License 1.1")
	}
	db := okagesvg.NewFontDatabase()
	ids := db.LoadFontData(Font().Data())
	if len(ids) != 1 {
		t.Fatalf("okage loaded %d faces, want 1", len(ids))
	}
	if got, _ := db.FaceName(ids[0], 0); !strings.HasPrefix(got, copyright) {
		t.Errorf("the font names the copyright %q, want %q", got, copyright)
	}
	if got, _ := db.FaceName(ids[0], 13); !strings.Contains(got, "SIL Open Font License, Version 1.1") {
		t.Error("the font does not name the SIL Open Font License 1.1")
	}
}

// TestTheReadmeNamesTheFile checks that README.md gives the SHA-256 of the
// embedded file, so that a reader can match it with its source.
func TestTheReadmeNamesTheFile(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if want := hex.EncodeToString(sum[:]); !strings.Contains(string(b), want) {
		t.Errorf("README.md does not give the SHA-256 %s of Excalifont-Regular.woff2", want)
	}
}

// TestTheFontHasLatin1 checks that the font has a glyph for each printable
// character of ASCII and of Latin-1, except those of missing, and for no
// character of missing.
func TestTheFontHasLatin1(t *testing.T) {
	t.Parallel()
	f := face(t)
	for r := rune(0x20); r <= 0xff; r++ {
		if r >= 0x7f && r < 0xa0 {
			continue
		}
		_, ok := f.NominalGlyph(r)
		if want := !strings.ContainsRune(missing, r); ok != want {
			t.Errorf("U+%04X %q: the font has a glyph %t, want %t", r, r, ok, want)
		}
	}
}

// TestTheMetricsAreSane checks the em, the ascender and the descender of
// the font, and that each printable character of ASCII has an outline and
// an advance of a letter.
func TestTheMetricsAreSane(t *testing.T) {
	t.Parallel()
	f := face(t)
	if got := f.Upem(); got != 1000 {
		t.Errorf("the em is %d units, want 1000", got)
	}
	ext, ok := f.FontHExtents()
	if !ok || ext.Ascender != 886 || ext.Descender != -374 {
		t.Errorf("the extents are %+v, want an ascender of 886 and a descender of -374", ext)
	}
	for r := rune(0x21); r < 0x7f; r++ {
		gid, _ := f.NominalGlyph(r)
		adv := f.HorizontalAdvance(gid)
		if adv < 100 || adv > 1500 {
			t.Errorf("%q advances %g units, want from 100 to 1500", r, adv)
		}
		g, ok := f.GlyphExtents(gid)
		if !ok || g.Width <= 0 || g.Height >= 0 {
			t.Errorf("%q has the extents %+v, want an outline", r, g)
		}
	}
	gid, _ := f.NominalGlyph(' ')
	if adv := f.HorizontalAdvance(gid); adv <= 0 || adv > 600 {
		t.Errorf("the space advances %g units, want more than 0 and at most 600", adv)
	}
	x, _ := f.NominalGlyph('x')
	h, _ := f.NominalGlyph('H')
	xe, _ := f.GlyphExtents(x)
	he, _ := f.GlyphExtents(h)
	if xe.YBearing >= he.YBearing {
		t.Errorf("x is %g units high and H %g, want x lower than H", xe.YBearing, he.YBearing)
	}
}

// TestOkageDrawsTheFont checks that a renderer of okage that loads the
// font shapes a text in it: the width of the text is the sum of the
// advances of its glyphs within 2 percent, which leaves room for the
// kerning, and is not the width of IBM Plex Sans.
func TestOkageDrawsTheFont(t *testing.T) {
	t.Parallel()
	const s, size = "Hello, sketchy world 123", 16
	f := face(t)
	var sum float64
	for _, r := range s {
		gid, _ := f.NominalGlyph(r)
		sum += float64(f.HorizontalAdvance(gid)) / float64(f.Upem()) * size
	}
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFile(Font().Data()),
		okagesvg.WithSansSerifFamily(Family), okagesvg.WithFontFamily(Family))
	run, err := r.ShapeText(s, okagesvg.TextStyle{Family: Family, Size: size, Weight: 400})
	if err != nil {
		t.Fatal(err)
	}
	if got := float64(run.Width()); math.Abs(got-sum) > 0.02*sum {
		t.Errorf("okage draws %q %.2f wide, and its advances sum to %.2f", s, got, sum)
	}
	plex := text.NewEstimate().Width(text.Font{Family: text.Family, Size: size, Weight: 400}, s)
	if math.Abs(sum-plex) < 1 {
		t.Errorf("%q measures %.2f in Excalifont and %.2f in IBM Plex Sans, want them to differ", s, sum, plex)
	}
	t.Logf("%q at %d px: Excalifont %.2f, okage %.2f, IBM Plex Sans %.2f", s, size, sum, run.Width(), plex)
}

// TestEstimateMeasuresInTheMainFont checks that an Estimate with the font as
// its main font measures each character that the font has in it, at each
// weight, and each other character in IBM Plex Sans, which then is not
// missing (D40 part 1).
func TestEstimateMeasuresInTheMainFont(t *testing.T) {
	t.Parallel()
	f := face(t)
	plain, main := text.NewEstimate(), text.NewEstimate()
	main.SetMain(Font())
	advance := func(s string, size float64) float64 {
		w := 0.0
		for _, r := range s {
			gid, _ := f.NominalGlyph(r)
			w += float64(f.HorizontalAdvance(gid)) / float64(f.Upem()) * size
		}
		return w
	}
	for _, weight := range []int{400, 700} {
		font := text.Font{Family: "any family", Size: 20, Weight: weight}
		const s = "Hand drawn"
		if got, want := main.Width(font, s), advance(s, 20); math.Abs(got-want) > 1e-9 {
			t.Errorf("the weight %d measures %q as %g, want %g", weight, s, got, want)
		}
		if main.Width(font, s) == plain.Width(font, s) {
			t.Errorf("the weight %d measures %q as IBM Plex Sans does", weight, s)
		}
	}
	// The font does not have the section sign, so it measures in IBM Plex
	// Sans.
	font := text.Font{Size: 20, Weight: 400}
	if got, want := main.Width(font, "§"), plain.Width(font, "§"); got != want || got == 0 {
		t.Errorf("§ measures %g, want the %g of IBM Plex Sans", got, want)
	}
	if m := main.Missing("Hand §"); len(m) != 0 {
		t.Errorf("the missing characters are %q, want none", m)
	}
}
