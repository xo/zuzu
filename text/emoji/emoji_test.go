package emoji

import (
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"testing"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/text"
)

// TestWidth checks that an Estimate with the font measures an emoji as
// okage draws it, with the bundled font and this one, so that a label with
// an emoji fits (D32).
func TestWidth(t *testing.T) {
	t.Parallel()
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithFontFile(Font().Data()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
	m := text.NewEstimate(Font())
	f := text.Font{Family: text.Family, Size: 14, Weight: 400}
	// The raster package shapes each piece of Split on its own, and the
	// pieces follow one another.
	for _, s := range []string{
		"😀", "a😀b", "🐶 dog", "👍🏽", "👨‍👩‍👧", "🇯🇵", "🇯🇵🇫🇷", "1️⃣", "❤️", "Café ☕",
		"Flags 🇯🇵 🇫🇷 and a", "a family 👨‍👩‍👧 and 😀", "key 1️⃣ 2", "tones 👍🏽👍🏿 end",
	} {
		want := 0.0
		for _, piece := range m.Split(s) {
			run, err := r.ShapeText(piece, okagesvg.TextStyle{Family: text.Family, Size: float32(f.Size), Weight: 400})
			if err != nil && strings.TrimSpace(piece) != "" {
				t.Fatalf("%q: the piece %q: %v", s, piece, err)
			}
			if err != nil {
				want += m.Width(f, piece)
				continue
			}
			want += float64(run.Width())
		}
		if got := m.Width(f, s); math.Abs(got-want) > 0.5 {
			t.Errorf("%q measures %.2f, and okage draws it %.2f wide", s, got, want)
		}
		if miss := m.Missing(s); len(miss) != 0 {
			t.Errorf("%q misses %q", s, miss)
		}
	}
	for s, want := range map[string][]string{
		"Flags 🇯🇵 🇫🇷 and a": {"Flags ", "🇯🇵", " ", "🇫🇷", " and a"},
		"a 👨‍👩‍👧 b":         {"a ", "👨‍👩‍👧", " b"},
		"key 1️⃣ 👍🏽":        {"key ", "1️⃣", " ", "👍🏽"},
		"plain":             {"plain"},
	} {
		if got := m.Split(s); !slices.Equal(got, want) {
			t.Errorf("Split(%q) = %q, want %q", s, got, want)
		}
	}
	// No font has a star, so it is missing and measures as nothing.
	if miss := m.Missing("a★b"); len(miss) != 1 || miss[0] != '★' {
		t.Errorf("the missing characters of a★b are %q, want ★", miss)
	}
	if got, want := m.Width(f, "a★b"), m.Width(f, "ab"); got != want {
		t.Errorf("a★b measures %.2f, want %.2f as ab", got, want)
	}
	// Without the font, an emoji is missing and measures as nothing.
	plain := text.NewEstimate()
	if miss := plain.Missing("a😀b😀"); len(miss) != 1 || miss[0] != '😀' {
		t.Errorf("without the font, the missing characters are %q, want 😀", miss)
	}
	if got, want := plain.Width(f, "a😀b"), plain.Width(f, "ab"); got != want {
		t.Errorf("without the font, a😀b measures %.2f, want %.2f as ab", got, want)
	}
}

// TestFallbackReadsAWebFont checks that a fallback decodes a font in a web
// format (D34): the font of this package, in WOFF 1.0, measures as the
// TrueType file does.
func TestFallbackReadsAWebFont(t *testing.T) {
	t.Parallel()
	web := text.NewFallback(Family, woff(t, data))
	if b := web.Data(); len(b) < 4 || binary.BigEndian.Uint32(b) != 0x00010000 {
		t.Fatalf("Data of the font in WOFF does not start as a TrueType file")
	}
	f := text.Font{Family: text.Family, Size: 14, Weight: 400}
	want, got := text.NewEstimate(Font()), text.NewEstimate(web)
	for _, s := range []string{"😀", "a👍🏽b", "🇯🇵"} {
		if w, g := want.Width(f, s), got.Width(f, s); g != w {
			t.Errorf("%q measures %.2f in WOFF, want %.2f", s, g, w)
		}
	}
}

// woff wraps a TrueType file in WOFF 1.0, with each table stored and not
// compressed, which WOFF allows.
func woff(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	be := binary.BigEndian
	n := int(be.Uint16(sfnt[4:]))
	pad := func(n int) int { return (n + 3) &^ 3 }
	size, total := 44+20*n, 12+16*n
	for i := range n {
		l := int(be.Uint32(sfnt[12+16*i+12:]))
		size, total = size+pad(l), total+pad(l)
	}
	out := make([]byte, 44+20*n, size)
	copy(out, "wOFF")
	be.PutUint32(out[4:], be.Uint32(sfnt))
	be.PutUint32(out[8:], uint32(size))
	be.PutUint16(out[12:], uint16(n))
	be.PutUint32(out[16:], uint32(total))
	for i := range n {
		rec, dir := sfnt[12+16*i:], out[44+20*i:]
		off, l := be.Uint32(rec[8:]), be.Uint32(rec[12:])
		copy(dir, rec[:4])
		be.PutUint32(dir[4:], uint32(len(out)))
		be.PutUint32(dir[8:], l)
		be.PutUint32(dir[12:], l)
		copy(dir[16:], rec[4:8])
		out = append(out, sfnt[off:off+l]...)
		out = append(out, make([]byte, pad(int(l))-int(l))...)
	}
	if len(out) != size {
		t.Fatalf("the WOFF file holds %d bytes, want %d", len(out), size)
	}
	return out
}
