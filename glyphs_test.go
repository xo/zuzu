package zuzu

import (
	"bytes"
	"context"
	"errors"
	"image"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/emoji"
)

// emojiSrc is a flowchart with emoji and a star, which no font has.
const emojiSrc = "flowchart LR\n  A[\"Deploy 🚀\"] --> B[\"Ship 🇯🇵 ★ 👨‍👩‍👧\"]\n  B --> C[\"★ again 🚀\"]\n"

// TestMissingGlyphs checks that a character that no font of the render has
// draws nothing, and that the report names it once, or that it is
// ErrNoGlyph in the strict mode (D32).
func TestMissingGlyphs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range []struct {
		name string
		opts []RenderOption
		want []string
	}{
		{"no emoji", nil, []string{"🚀", "🇯", "🇵", "★", "👨", "👩", "👧"}},
		{"emoji", []RenderOption{WithEmoji(emoji.Font())}, []string{"★"}},
	} {
		chart, _, err := Parse([]byte(emojiSrc))
		if err != nil {
			t.Fatal(err)
		}
		s, report, err := chart.Render(ctx, c.opts...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []string
		for _, is := range report {
			if !errors.Is(is, ErrNoGlyph) {
				t.Errorf("%s: the issue %v, want ErrNoGlyph", c.name, is)
			}
			for _, w := range c.want {
				if strings.Contains(is.Message, "'"+w+"'") {
					got = append(got, w)
				}
			}
		}
		if len(report) != len(c.want) || len(got) != len(c.want) {
			t.Errorf("%s: the report %v, want one issue for each of %v", c.name, report, c.want)
		}
		for _, l := range texts(&s.Root) {
			if strings.Contains(l, "★") {
				t.Errorf("%s: the text %q holds a star, which no font has", c.name, l)
			}
		}
		strict, _, err := Parse([]byte(emojiSrc), WithStrict(true))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := strict.Render(ctx, c.opts...); !errors.Is(err, ErrNoGlyph) {
			t.Errorf("%s: in the strict mode, the error %v, want ErrNoGlyph", c.name, err)
		}
	}
}

// texts returns the lines of the text items of a mark and its groups.
func texts(m *scene.Mark) []string {
	var out []string
	for i := range m.Items {
		if m.Type == scene.Text {
			out = append(out, m.Items[i].Text...)
		}
		for j := range m.Items[i].Items {
			out = append(out, texts(&m.Items[i].Items[j])...)
		}
	}
	return out
}

// TestEmoji checks that an emoji measures with the font of WithEmoji, so
// that its node is wider, that the SVG names the family of the font, and
// that the PNG draws the emoji in color.
func TestEmoji(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := []byte("flowchart LR\n  A[\"go 🚀🚀🚀\"]\n")
	chart, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	plain, _, err := chart.Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	color, report, err := chart.Render(ctx, WithEmoji(emoji.Font()))
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v, want none", report, err)
	}
	// Without the font, the emoji measure as nothing. With it, three emoji
	// of Noto Color Emoji at 11 pixels are about 41 pixels wide, and the
	// least width of the node takes a part of that.
	if d := color.Width - plain.Width; d < 25 {
		t.Errorf("the emoji make the diagram %.1f wider, want 25 or more", d)
	}
	var b bytes.Buffer
	if _, err := chart.SVG(ctx, &b, WithEmoji(emoji.Font())); err != nil {
		t.Fatal(err)
	}
	if want := `font-family="'IBM Plex Sans', '` + text.EmojiFamily + `', sans-serif"`; !strings.Contains(b.String(), want) {
		t.Errorf("the SVG does not name %s:\n%s", want, b.String())
	}
	img, _, err := chart.Image(ctx, WithEmoji(emoji.Font()))
	if err != nil {
		t.Fatal(err)
	}
	// The rocket of Noto Color Emoji is red, blue and gray. The theme draws
	// no red in a node.
	if !hasRed(img) {
		t.Error("the PNG has no red, so the emoji did not draw in color")
	}
	img, _, err = chart.Image(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hasRed(img) {
		t.Error("without the font, the PNG has red")
	}
}

// hasRed reports whether an image has a pixel that is clearly red.
func hasRed(img *image.RGBA) bool {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
		if r > 180 && g < 100 && b < 100 {
			return true
		}
	}
	return false
}
