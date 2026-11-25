package zuzu

import (
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// WithFont gives a parse or a render the font of the chart, such as IBM
// Plex Mono of the package text/mono (D35 part 11, D58 part 1):
//
//	font := zuzu.WithFont(mono.Font())
//	c, report, err := zuzu.Parse(src, font)
//	img, report, err := c.Image(ctx, font)
//
// The chart measures each text in the font, with IBM Plex Sans for the
// characters that the font does not have, so the layout makes room for it.
// A font with several faces measures and draws each weight and style in
// its own face. Each text names the family of the font first, before IBM
// Plex Sans, over the font that the spec gives. The SVG embeds the font in
// a rule @font-face (D43), and the PNG of Image draws with it. A program
// that does not import the package of a font does not hold the font.
//
// In a look drawn by hand, the font of WithLookFont wins over it. A look
// with no font of its own draws in the font of WithFont, and the report
// then gives no ErrNoLookFont.
//
// It is an option of Parse too, as WithLookFont is, because some kinds of
// Mermaid lay out their labels when they compile. The chart keeps the
// font, and each render of it that gives no font uses the font. When both
// give a font, the font of the render wins. A nil font gives no font.
func WithFont(f *text.Fallback) SharedOption {
	return fontOption{f: f}
}

// fontOption is the option of WithFont.
type fontOption struct {
	f *text.Fallback
}

// applyParse satisfies Option.
func (f fontOption) applyParse(o *options) {
	o.font = f.f
}

// applyRender satisfies RenderOption.
func (f fontOption) applyRender(o *renderOptions) {
	o.font = f.f
}

// font returns the font of WithFont: the font of the render, or else that
// of the parse, or nil.
func (c *Chart) font(o *renderOptions) *text.Fallback {
	if o.font != nil {
		return o.font
	}
	return c.opts.font
}

// useFont gives each text of a mark and of the marks under it a family,
// and cuts the text to its limit in the font that m measures, so that each
// renderer draws the text that the layout measured, as the pass of a look
// drawn by hand does (D40). The scene of a render is its own, so useFont
// changes it in place.
func useFont(mk *scene.Mark, family string, m text.Measurer) {
	for i := range mk.Items {
		it := &mk.Items[i]
		if mk.Type == scene.Text {
			it.Font = family
			if it.Limit > 0 {
				it.Text = shape.TextOf(it, m).Lines
				it.Limit = 0
			}
		}
		for j := range it.Items {
			useFont(&it.Items[j], family, m)
		}
	}
}
