package svg

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/emoji"
)

// TestWithFontEmbedsTheFontOfAText checks that WithFont embeds a font as a
// rule @font-face, with the bytes of its file in their own format, only when
// a text names its family (D43).
func TestWithFontEmbedsTheFontOfAText(t *testing.T) {
	t.Parallel()
	woff2 := []byte("wOF2 not a real font")
	sketch := text.NewFallback("Sketch", woff2)
	other := text.NewFallback("Other", []byte("\x00\x01\x00\x00 a font"))
	write := func(font string, opts ...Option) string {
		t.Helper()
		root := scene.Mark{Type: scene.Group, Items: []scene.Item{{Items: []scene.Mark{
			{Type: scene.Text, Items: []scene.Item{{X: 1, Y: 10, Text: scene.Lines{"a b"}, Font: font, FontSize: 11}}},
		}}}}
		var b bytes.Buffer
		if err := Write(&b, &scene.Scene{Width: 50, Height: 50, Root: root}, opts...); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	out := write("'Sketch', 'IBM Plex Sans', sans-serif", WithFont(sketch), WithFont(other), WithFont(nil))
	want := "<defs><style>@font-face{font-family:'Sketch';src:url(data:font/woff2;base64," +
		base64.StdEncoding.EncodeToString(woff2) + ") format('woff2')}</style></defs>"
	if !strings.Contains(out, want) {
		t.Errorf("no rule @font-face of Sketch in\n%s", out)
	}
	if strings.Contains(out, "'Other'") {
		t.Errorf("a font that no text names is embedded in\n%s", out)
	}
	if out := write("'IBM Plex Sans', sans-serif", WithFont(sketch)); strings.Contains(out, "@font-face") {
		t.Errorf("a font is embedded with no text that names it in\n%s", out)
	}
	if out := write("'Other'", WithFont(other)); !strings.Contains(out, "data:font/ttf;base64,") || !strings.Contains(out, "format('truetype')") {
		t.Errorf("a TrueType font is not embedded as truetype in\n%s", out)
	}
}

// TestWithFallbacksMeasuresTheLimitWithTheEmoji checks that a text with a
// limit measures its emoji in the font of WithFallbacks, as the layout
// measures it, and so is cut at the same place (D32). Without the font, an
// emoji measures as nothing, and the text fits.
func TestWithFallbacksMeasuresTheLimitWithTheEmoji(t *testing.T) {
	t.Parallel()
	const line, limit = "🚀🚀🚀🚀 go far", 40
	write := func(opts ...Option) string {
		t.Helper()
		root := scene.Mark{Type: scene.Group, Items: []scene.Item{{Items: []scene.Mark{
			{Type: scene.Text, Items: []scene.Item{{X: 1, Y: 10, Text: scene.Lines{line}, FontSize: 11, Limit: limit}}},
		}}}}
		var b bytes.Buffer
		if err := Write(&b, &scene.Scene{Width: 50, Height: 50, Root: root}, opts...); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if out := write(); !strings.Contains(out, ">"+line+"</text>") {
		t.Errorf("without the font of emoji, the text is cut in\n%s", out)
	}
	f := text.Font{Size: 11}
	want := text.Truncate(text.NewEstimate(emoji.Font()), f, line, limit)
	if want == line || !strings.HasSuffix(want, text.Ellipsis) {
		t.Fatalf("the layout cuts %q to %q, want a cut text", line, want)
	}
	out := write(WithFallbacks(emoji.Font()))
	if !strings.Contains(out, ">"+want+"</text>") {
		t.Errorf("with the font of emoji, the text is not %q in\n%s", want, out)
	}
	if !strings.Contains(out, "'"+text.EmojiFamily+"'") {
		t.Errorf("the text does not name the font of emoji in\n%s", out)
	}
}
