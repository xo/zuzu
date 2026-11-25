package humanist

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"strings"
	"testing"

	okagesvg "github.com/xo/okage/svg"
	gotext "github.com/xo/okage/typesetting/font"

	"github.com/xo/zuzu/text"
)

// copyright is the first line of LICENSE.txt.
const copyright = "Copyright 2020 The Inter Project Authors (https://github.com/rsms/inter)"

// faces are the weight and the style of each face of the collection, in
// its order (D34).
var faces = []struct {
	weight int
	italic bool
}{{400, false}, {500, false}, {600, false}, {400, true}, {500, true}, {600, true}}

// parse parses each face of the font with go-text, as text.Estimate does.
func parse(t *testing.T) []*gotext.Face {
	t.Helper()
	b := Font().Data()
	if !bytes.HasPrefix(b, []byte("ttcf")) {
		t.Fatal("Data of the font is not a TrueType collection")
	}
	fs, err := gotext.ParseTTC(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("parsing the font: %v", err)
	}
	return fs
}

// advance returns the sum of the advances of the glyphs of s in a face, at
// a size.
func advance(f *gotext.Face, s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		gid, _ := f.NominalGlyph(r)
		w += float64(f.HorizontalAdvance(gid)) / float64(f.Upem()) * size
	}
	return w
}

// TestTheFontParses checks that the font decodes from WOFF2 into a
// TrueType collection of six faces of the family, with the weights 400,
// 500 and 600, each upright and italic, which go-text and okage both read
// (D34).
func TestTheFontParses(t *testing.T) {
	t.Parallel()
	if got := Font().Family(); got != Family {
		t.Errorf("the family of the fallback is %q, want %q", got, Family)
	}
	fs := parse(t)
	if len(fs) != len(faces) {
		t.Fatalf("the collection has %d faces, want %d", len(fs), len(faces))
	}
	for i, f := range fs {
		d := f.Describe()
		want := faces[i]
		if d.Family != Family || int(d.Aspect.Weight) != want.weight || (d.Aspect.Style == gotext.StyleItalic) != want.italic {
			t.Errorf("go-text describes the face %d as %+v, want %s, of weight %d, italic %t", i, d, Family, want.weight, want.italic)
		}
	}
	db := okagesvg.NewFontDatabase()
	ids := db.LoadFontData(Font().Data())
	if len(ids) != len(faces) {
		t.Fatalf("okage loaded %d faces, want %d", len(ids), len(faces))
	}
	for i, id := range ids {
		// The family of the upright and italic faces of weight 400 is in
		// the entry 1 of the table name, and that of the other faces is in
		// the entry 16.
		name, _ := db.FaceName(id, 16)
		if name == "" {
			name, _ = db.FaceName(id, 1)
		}
		if name != Family {
			t.Errorf("okage reads the family %q in the face %d, want %q", name, i, Family)
		}
	}
}

// TestTheFontHoldsItsLicense checks that the license travels with the
// font, as the Open Font License asks (D16): in LICENSE.txt beside it, and
// in the table name of each face, which a program that embeds the font
// holds.
func TestTheFontHoldsItsLicense(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("LICENSE.txt")
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.HasPrefix(s, copyright) || !strings.Contains(s, "SIL OPEN FONT LICENSE Version 1.1") {
		t.Error("LICENSE.txt is not the copyright of the font and the Open Font License 1.1")
	}
	db := okagesvg.NewFontDatabase()
	for i, id := range db.LoadFontData(Font().Data()) {
		if got, _ := db.FaceName(id, 13); !strings.Contains(got, "SIL Open Font License, Version 1.1") {
			t.Errorf("the face %d does not name the SIL Open Font License 1.1", i)
		}
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
		t.Errorf("README.md does not give the SHA-256 %s of the embedded file", want)
	}
}

// TestEstimateMeasuresEachFace checks that an Estimate with the font as its
// main font measures each weight and style in its own face, a weight
// between two faces in the nearest one, and a text in another width than
// IBM Plex Sans gives it (D58 part 1).
func TestEstimateMeasuresEachFace(t *testing.T) {
	t.Parallel()
	const s, size = "Hello, world 123", 20
	fs := parse(t)
	plain, main := text.NewEstimate(), text.NewEstimate()
	main.SetMain(Font())
	for i, want := range faces {
		for _, weight := range []int{want.weight, want.weight + 40} {
			font := text.Font{Family: "any family", Size: size, Weight: weight, Italic: want.italic}
			if got, want := main.Width(font, s), advance(fs[i], s, size); math.Abs(got-want) > 1e-9 {
				t.Errorf("the weight %d, italic %t, measures %q as %g, want %g", weight, font.Italic, s, got, want)
			}
		}
	}
	font := text.Font{Size: size, Weight: 400}
	if got, plex := main.Width(font, s), plain.Width(font, s); math.Abs(got-plex) < 1 {
		t.Errorf("%q measures %.2f in %s and %.2f in IBM Plex Sans, want them to differ", s, got, Family, plex)
	}
}

// TestOkageDrawsTheFont checks that a renderer of okage that loads the
// font shapes a text in it at each weight and style: the width of the text
// is the sum of the advances of its glyphs in that face within 2 percent,
// which leaves room for the kerning.
func TestOkageDrawsTheFont(t *testing.T) {
	t.Parallel()
	const s, size = "Hello, world 123", 16
	fs := parse(t)
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithFontFile(Font().Data()), okagesvg.WithSansSerifFamily(text.Family))
	for i, want := range faces {
		style := okagesvg.TextStyle{Family: "'" + Family + "', sans-serif", Size: size, Weight: uint16(want.weight)}
		if want.italic {
			style.Style = okagesvg.FontStyleItalic
		}
		run, err := r.ShapeText(s, style)
		if err != nil {
			t.Fatal(err)
		}
		if got, sum := float64(run.Width()), advance(fs[i], s, size); math.Abs(got-sum) > 0.02*sum {
			t.Errorf("okage draws %q at the weight %d, italic %t, %.2f wide, and its advances sum to %.2f", s, want.weight, want.italic, got, sum)
		}
	}
}
