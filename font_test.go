package zuzu

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/excalifont"
	"github.com/xo/zuzu/text/humanist"
	"github.com/xo/zuzu/text/mono"
	"github.com/xo/zuzu/text/rounded"
	"github.com/xo/zuzu/text/serif"
	"github.com/xo/zuzu/theme"
)

// chartFonts are the four bundled fonts of WithFont (D58 part 1).
var chartFonts = []*text.Fallback{mono.Font(), humanist.Font(), serif.Font(), rounded.Font()}

// textFamilies returns the families of the texts of a scene, and the
// number of its texts.
func textFamilies(s *scene.Scene) (map[string]bool, int) {
	families, n := map[string]bool{}, 0
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Type == scene.Text {
				families[m.Items[i].Font] = true
				n++
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return families, n
}

// fontChart parses a source of testdata/look in the strict mode, with the
// options of the parse.
func fontChart(t *testing.T, name string, opts ...Option) *Chart {
	t.Helper()
	src, err := os.ReadFile("testdata/look/" + name)
	if err != nil {
		t.Fatal(err)
	}
	c, report, err := Parse(src, append([]Option{WithStrict(true)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("%s: the report of the parse is %v, want none", name, report)
	}
	return c
}

// TestWithFontNamesTheFont checks that a render with each bundled font
// gives each text the family of the font before IBM Plex Sans, and that
// the SVG names the family and embeds the font in WOFF2 (D43).
func TestWithFontNamesTheFont(t *testing.T) {
	t.Parallel()
	for _, f := range chartFonts {
		family := "'" + f.Family() + "', " + shape.DefaultFont
		for _, name := range []string{"bar.vl.json", "flowchart.mmd"} {
			c := fontChart(t, name)
			s, report, err := c.Render(t.Context(), WithLocation(time.UTC), WithFont(f))
			if err != nil {
				t.Fatal(err)
			}
			if len(report) != 0 {
				t.Errorf("%s %s: the report is %v, want none", f.Family(), name, report)
			}
			families, n := textFamilies(s)
			if n == 0 || len(families) != 1 || !families[family] {
				t.Errorf("%s %s: the texts have the families %v, want %s first", f.Family(), name, families, f.Family())
			}
			var b bytes.Buffer
			if _, err := c.SVG(t.Context(), &b, WithLocation(time.UTC), WithFont(f)); err != nil {
				t.Fatal(err)
			}
			svg := b.String()
			if !strings.Contains(svg, `font-family="`+family) {
				t.Errorf("%s %s: the SVG names no text in the family %s", f.Family(), name, family)
			}
			if !strings.Contains(svg, "@font-face{font-family:'"+f.Family()+"';src:url(data:font/woff2;base64,") {
				t.Errorf("%s %s: the SVG does not embed the font in WOFF2", f.Family(), name)
			}
		}
	}
}

// TestWithFontChangesTheLayout checks that the layout measures in the
// font: a chart is laid out apart from the chart in IBM Plex Sans, and a
// text with a limit is cut in the font.
func TestWithFontChangesTheLayout(t *testing.T) {
	t.Parallel()
	const src = `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 200, "height": 40,
		"marks": [{"type": "text", "name": "label", "encode": {"enter": {"x": {"value": 0}, "y": {"value": 20},
		"text": {"value": "A label that is far too long for its limit"}, "limit": {"value": 150}, "fontSize": {"value": 14}}}}]}`
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	plain, _, err := c.Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := plain.Root.Items[0].Items[0].Items[0]
	for _, f := range chartFonts {
		s, _, err := c.Render(t.Context(), WithFont(f))
		if err != nil {
			t.Fatal(err)
		}
		it := s.Root.Items[0].Items[0].Items[0]
		if it.Limit != 0 || len(it.Text) != 1 || !strings.HasSuffix(it.Text[0], text.Ellipsis) {
			t.Fatalf("%s: the text is %q with the limit %g, want it cut with an ellipsis and no limit", f.Family(), it.Text, it.Limit)
		}
		m := text.NewEstimate()
		m.SetMain(f)
		font := text.Font{Size: 14, Weight: 400}
		if w := m.Width(font, it.Text[0]); w > 150 {
			t.Errorf("%s: the text %q is %g wide in the font, more than its limit of 150", f.Family(), it.Text[0], w)
		}
		if strings.Join(it.Text, "") == strings.Join(want.Text, "") && it.Font == want.Font {
			t.Errorf("%s: the text draws as it draws in IBM Plex Sans", f.Family())
		}
	}
	// A sequence of Mermaid lays out when it compiles, so the font of the
	// parse changes its size.
	plainSeq, _, err := fontChart(t, "sequence.mmd").Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	monoSeq, _, err := fontChart(t, "sequence.mmd", WithFont(mono.Font())).Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if plainSeq.Width == monoSeq.Width {
		t.Errorf("the sequence diagram is %g wide in IBM Plex Sans and in IBM Plex Mono", plainSeq.Width)
	}
}

// TestWithFontImageMatchesSVG draws each font with the direct renderer
// and with okage from the SVG, and asks that at most 1 percent of the
// pixels differ, the gate of okage D31 (D11), and that the image differs
// from the image in IBM Plex Sans.
func TestWithFontImageMatchesSVG(t *testing.T) {
	t.Parallel()
	c := fontChart(t, "bar.vl.json")
	plain, _, err := c.Image(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range chartFonts {
		img, report, err := c.Image(t.Context(), WithLocation(time.UTC), WithFont(f))
		if err != nil {
			t.Fatal(err)
		}
		if len(report) != 0 {
			t.Errorf("%s: the report is %v, want none", f.Family(), report)
		}
		var b bytes.Buffer
		if _, err := c.SVG(t.Context(), &b, WithLocation(time.UTC), WithFont(f)); err != nil {
			t.Fatal(err)
		}
		r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
			okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family),
			okagesvg.WithFontFile(f.Data()))
		want, err := r.Render(b.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds() != want.Bounds() {
			t.Fatalf("%s: the size is %v, want %v", f.Family(), img.Bounds(), want.Bounds())
		}
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			for k := range 4 {
				if d := int(img.Pix[i+k]) - int(want.Pix[i+k]); d > 1 || d < -1 {
					n++
					break
				}
			}
		}
		if share := float64(n) / float64(len(img.Pix)/4); share > 0.01 {
			t.Errorf("%s: %.2f%% of the pixels differ, more than 1%%", f.Family(), 100*share)
		}
		if img.Bounds() == plain.Bounds() && bytes.Equal(img.Pix, plain.Pix) {
			t.Errorf("%s: the image is the image in IBM Plex Sans", f.Family())
		}
	}
}

// TestWithFontAndTheLook checks that the font of a look drawn by hand wins
// over the font of WithFont, that a look with no font of its own draws in
// the font of WithFont with no warning, and that the font of a render wins
// over the font of the parse.
func TestWithFontAndTheLook(t *testing.T) {
	t.Parallel()
	look := WithLook(theme.LookExcalidraw)
	cases := []struct {
		name   string
		parse  []Option
		render []RenderOption
		want   string
	}{
		{"the font of the look wins", nil, []RenderOption{look, WithFont(mono.Font()), lookFont}, excalifont.Family},
		{"a look with no font", nil, []RenderOption{look, WithFont(mono.Font())}, mono.Family},
		{"the font of the parse", []Option{WithFont(serif.Font())}, nil, serif.Family},
		{"the font of the render wins", []Option{WithFont(serif.Font())}, []RenderOption{WithFont(rounded.Font())}, rounded.Family},
	}
	for _, c := range cases {
		ch := fontChart(t, "bar.vl.json", c.parse...)
		s, report, err := ch.Render(t.Context(), append([]RenderOption{WithLocation(time.UTC)}, c.render...)...)
		if err != nil {
			t.Fatal(err)
		}
		for _, is := range report {
			if errors.Is(is, ErrNoLookFont) {
				t.Errorf("%s: the report warns %v", c.name, is)
			}
		}
		families, _ := textFamilies(s)
		if len(families) != 1 || !families["'"+c.want+"', "+shape.DefaultFont] {
			t.Errorf("%s: the texts have the families %v, want %s first", c.name, families, c.want)
		}
	}
}
