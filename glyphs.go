package zuzu

import (
	"slices"
	"strings"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// WithEmoji gives a parse or a render a font for the emoji, so that an
// emoji draws in color (D32). The package text/emoji gives the font, Noto
// Color Emoji:
//
//	c, report, err := zuzu.Parse(src, zuzu.WithEmoji(emoji.Font()))
//	img, report, err := c.Image(ctx, zuzu.WithEmoji(emoji.Font()))
//
// The text measures with the font, and the PNG of Image draws with it. The
// SVG names its family for a text with an emoji, and a viewer draws it in
// the color font that it has. Without the font, an emoji draws nothing,
// as a character that no font has. A program that does not import
// text/emoji does not hold the font.
//
// It is an option of Parse too, because some kinds of Mermaid, such as a
// sequence, a pie or a mindmap, lay out their labels when they compile
// (D38). The chart keeps the font, and each render of it that gives no font
// of its own uses the font. When both give a font, the font of the render
// wins.
func WithEmoji(f *text.Fallback) SharedOption {
	return emojiOption{f: f}
}

// emojiOption is the option of WithEmoji.
type emojiOption struct {
	f *text.Fallback
}

// applyParse satisfies Option.
func (e emojiOption) applyParse(o *options) {
	if e.f != nil {
		o.emoji = append(o.emoji, e.f)
	}
}

// applyRender satisfies RenderOption.
func (e emojiOption) applyRender(o *renderOptions) {
	if e.f != nil {
		o.fallbacks = append(o.fallbacks, e.f)
	}
}

// fallbacks returns the fonts of WithEmoji of a render, or those of the
// parse when the render gives none (D38).
func (c *Chart) fallbacks(o *renderOptions) []*text.Fallback {
	if len(o.fallbacks) > 0 {
		return o.fallbacks
	}
	return c.opts.emoji
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
