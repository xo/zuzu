package zuzu

import (
	"slices"
	"strings"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// WithEmoji gives a render a font for the emoji, so that an emoji draws in
// color (D32). The package text/emoji gives the font, Noto Color Emoji:
//
//	c.Image(ctx, zuzu.WithEmoji(emoji.Font()))
//
// The text measures with the font, and the PNG of Image draws with it. The
// SVG names its family for a text with an emoji, and a viewer draws it in
// the color font that it has. Without the font, an emoji draws nothing,
// as a character that no font has. A program that does not import
// text/emoji does not hold the font.
func WithEmoji(f *text.Fallback) RenderOption {
	return func(o *renderOptions) {
		if f != nil {
			o.fallbacks = append(o.fallbacks, f)
		}
	}
}

// missingGlyphs takes out of each text of a scene the characters that no
// font of the render has, which draw nothing, and reports each character
// once (D32). In the strict mode, the first such character is an error.
func (rt *runtime) missingGlyphs(m *scene.Mark, seen map[rune]bool) error {
	for i := range m.Items {
		it := &m.Items[i]
		if m.Type == scene.Group {
			for j := range it.Items {
				if err := rt.missingGlyphs(&it.Items[j], seen); err != nil {
					return err
				}
			}
			continue
		}
		if m.Type != scene.Text {
			continue
		}
		for j, line := range it.Text {
			miss := rt.measure.Missing(line)
			if len(miss) == 0 {
				continue
			}
			for _, r := range miss {
				if seen[r] {
					continue
				}
				seen[r] = true
				if err := rt.issue(ErrNoGlyph, m.Name, "the character %q, U+%04X, which no font of the render has, draws nothing", r, r); err != nil {
					return err
				}
			}
			it.Text[j] = strings.Map(func(r rune) rune {
				if slices.Contains(miss, r) {
					return -1
				}
				return r
			}, line)
		}
	}
	return nil
}
