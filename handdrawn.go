package zuzu

import (
	"encoding/json/v2"
	"math"

	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/internal/sketch"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// WithLook sets the look, such as theme.LookExcalidraw, which draws each
// mark as if by hand (D39, D40). It wins over the look that the spec asks
// for with zuzu.look in its config, such as look: handDrawn of a Mermaid
// diagram. A look that zuzu does not have is an issue, and draws in the
// standard look.
//
// It is an option of Parse too, because some kinds of Mermaid lay out
// their labels when they compile, and a look measures its text in its own
// font (D40 part 1). The chart keeps the look, and each render of it that
// gives no look of its own uses the look. When both give a look, the look
// of the render wins.
func WithLook(l theme.Look) SharedOption {
	return lookOption{look: l}
}

// lookOption is the option of WithLook.
type lookOption struct {
	look theme.Look
}

// applyParse satisfies Option.
func (l lookOption) applyParse(o *options) {
	o.look, o.hasLook = l.look, true
}

// applyRender satisfies RenderOption.
func (l lookOption) applyRender(o *renderOptions) {
	o.look, o.hasLook = l.look, true
}

// WithLookSeed sets the seed of a look drawn by hand, which picks the
// strokes of each shape (D40 part 6, D58 part 3). It wins over the seed
// that the spec asks for with zuzu.lookSeed in its config, such as
// handDrawnSeed of a Mermaid diagram. A seed of 0 is theme.SketchSeed. The
// standard look draws no strokes by hand, and the seed changes nothing in
// it.
//
// It is an option of Parse too, as WithLook is. The chart keeps the seed,
// and each render of it that gives no seed of its own uses the seed. When
// both give a seed, the seed of the render wins.
func WithLookSeed(seed int64) SharedOption {
	return lookSeedOption{seed: seed}
}

// lookSeedOption is the option of WithLookSeed.
type lookSeedOption struct {
	seed int64
}

// applyParse satisfies Option.
func (l lookSeedOption) applyParse(o *options) {
	o.seed, o.hasSeed = l.seed, true
}

// applyRender satisfies RenderOption.
func (l lookSeedOption) applyRender(o *renderOptions) {
	o.seed, o.hasSeed = l.seed, true
}

// WithLookFont gives a parse or a render the font of a look drawn by hand.
// The package text/excalifont gives the font of the look like Excalidraw,
// Excalifont:
//
//	font := zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())
//	c, report, err := zuzu.Parse(src, font)
//	img, report, err := c.Image(ctx, zuzu.WithLook(theme.LookExcalidraw), font)
//
// The look measures its text in the font, with IBM Plex Sans for the
// characters that the font does not have, so the layout makes room for it
// (D40 part 1). The SVG names its family first, and the PNG of Image draws
// with it. Without the font, the look draws its text in IBM Plex Sans, and
// the report warns of it with ErrNoLookFont, in the strict mode too. A
// program that does not import
// text/excalifont does not hold the font. The standard look draws in IBM
// Plex Sans, and takes no font.
//
// It is an option of Parse too, as WithLook is. The chart keeps the font,
// and each render of it that gives no font for the look uses the font.
// When both give a font, the font of the render wins.
func WithLookFont(l theme.Look, f *text.Fallback) SharedOption {
	return lookFontOption{look: l, f: f}
}

// lookFontOption is the option of WithLookFont.
type lookFontOption struct {
	look theme.Look
	f    *text.Fallback
}

// applyParse satisfies Option.
func (l lookFontOption) applyParse(o *options) {
	if o.lookFonts == nil {
		o.lookFonts = map[theme.Look]*text.Fallback{}
	}
	o.lookFonts[l.look] = l.f
}

// applyRender satisfies RenderOption.
func (l lookFontOption) applyRender(o *renderOptions) {
	if o.lookFonts == nil {
		o.lookFonts = map[theme.Look]*text.Fallback{}
	}
	o.lookFonts[l.look] = l.f
}

// lookFont returns the font of a look: the font of the render, or else
// that of the parse, or nil.
func (c *Chart) lookFont(o *renderOptions, l theme.Look) *text.Fallback {
	if f := o.lookFonts[l]; f != nil {
		return f
	}
	return c.opts.lookFonts[l]
}

// sketchLook sets the look of the render, its seed and its font. The look
// of the render wins over the look of the parse, which wins over
// zuzu.look in the config of the spec (D40 part 1). The seed follows the
// same order, over zuzu.lookSeed (D58 part 3). A look that zuzu does not
// have and a seed that is not a whole number are issues. The font of a
// look drawn by hand wins over the font of WithFont (D58). A look drawn by
// hand with neither font is a warning.
func (rt *runtime) sketchLook() error {
	rt.drawLook, rt.lookSeed = theme.LookStandard, theme.SketchSeed
	cfg := rt.chart.spec.Config
	var l theme.Look
	path := "config." + theme.ConfigLook
	switch {
	case rt.opts.hasLook:
		l, path = rt.opts.look, ""
	case rt.chart.opts.hasLook:
		l, path = rt.chart.opts.look, ""
	default:
		if raw, ok := cfg[theme.ConfigLook]; ok {
			var name string
			if err := json.Unmarshal(raw, &name); err != nil {
				if err := rt.issue(vega.ErrUnsupported, path, "the look %s", raw); err != nil {
					return err
				}
			}
			l = theme.Look(name)
		}
	}
	switch {
	case l == "":
	case !l.Known():
		if err := rt.issue(vega.ErrUnsupported, path, "the look %q, which zuzu does not have, drawn in the standard look", l); err != nil {
			return err
		}
	default:
		rt.drawLook = l
	}
	switch raw, ok := cfg[theme.ConfigLookSeed]; {
	case rt.opts.hasSeed:
		rt.lookSeed = rt.opts.seed
	case rt.chart.opts.hasSeed:
		rt.lookSeed = rt.chart.opts.seed
	case ok:
		var seed float64
		err := json.Unmarshal(raw, &seed)
		switch {
		case err != nil || seed != math.Trunc(seed) || math.Abs(seed) > 1<<53:
			if err := rt.issue(vega.ErrUnsupported, "config."+theme.ConfigLookSeed, "the seed %s, which is not a whole number", raw); err != nil {
				return err
			}
		case seed != 0:
			rt.lookSeed = int64(seed)
		}
	}
	rt.font = rt.chart.font(&rt.opts)
	if rt.drawLook.HandDrawn() {
		if f := rt.chart.lookFont(&rt.opts, rt.drawLook); f != nil {
			rt.font = f
		}
		if rt.font == nil {
			// The text still draws, in IBM Plex Sans, so this is a
			// warning and never an error (D40).
			rt.warnf(ErrNoLookFont, "", "the look %s draws its text in %s, because the render has no font of the look, which zuzu.WithLookFont gives", rt.drawLook, text.Family)
		}
	}
	if rt.font != nil {
		rt.measure.SetMain(rt.font)
	}
	return nil
}

// sketch draws the scene of a render in a look drawn by hand, after the
// layout and before any renderer (D40 part 7). In the standard look, it
// gives each text the font of WithFont (D58).
func (rt *runtime) sketch(s *scene.Scene) {
	var family string
	if rt.font != nil {
		family = "'" + rt.font.Family() + "', " + shape.DefaultFont
	}
	if !rt.drawLook.HandDrawn() {
		if family != "" {
			useFont(&s.Root, family, rt.measure)
		}
		return
	}
	o := sketch.Options{Seed: rt.lookSeed}
	if family != "" {
		o.Font = family
		o.Measure = rt.measure
	}
	sketch.Draw(s, o)
}
