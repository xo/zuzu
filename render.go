package zuzu

import (
	"math"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
)

// padding is the space around a chart.
type padding struct {
	top, right, bottom, left float64
}

// readPadding reads the padding signal: a number, or an object with top,
// right, bottom and left.
func readPadding(v any) padding {
	switch x := v.(type) {
	case float64:
		return padding{x, x, x, x}
	case map[string]any:
		return padding{number(x["top"], 0), number(x["right"], 0), number(x["bottom"], 0), number(x["left"], 0)}
	}
	return padding{}
}

// autosize reads the type of the autosize signal: pad, fit, none and so
// on. The default is pad.
func autosize(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if t, ok := x["type"].(string); ok {
			return t
		}
	}
	return "pad"
}

// containsPadding reports whether the autosize signal puts the padding
// inside the width and the height, with "contains": "padding".
func containsPadding(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m["contains"] == "padding"
}

// render runs the spec and builds the scene graph.
func (rt *runtime) render() (*scene.Scene, error) {
	s := rt.root
	wv, err := s.signal("width")
	if err != nil {
		return nil, err
	}
	hv, err := s.signal("height")
	if err != nil {
		return nil, err
	}
	pv, err := s.signal("padding")
	if err != nil {
		return nil, err
	}
	w, h, pad := number(wv, 0), number(hv, 0), readPadding(pv)
	item := &scene.Item{Width: w, Height: h}
	s.item = item
	// dx and dy are the x and the y that the encode of the spec gives the
	// root group. They move its content inside the padding, as in Vega.
	var dx, dy float64
	if len(s.spec.Encode) > 0 {
		st := &itemState{it: item, set: map[string]bool{}}
		if err := s.encodeItem(s.spec.Encode, st, map[string]any{}, ""); err != nil {
			return nil, err
		}
		badLink := false
		if err := s.linkIssue(item, "", &badLink); err != nil {
			return nil, err
		}
		dx, dy = item.X, item.Y
	}
	item.X, item.Y = pad.left+dx, pad.top+dy
	// Every dataset of the top level runs, whether a mark draws it or not,
	// as in Vega, so that its errors show.
	for _, d := range s.spec.Data {
		if _, err := s.dataset(d.Name); err != nil {
			return nil, err
		}
	}
	marks, err := s.buildScope("")
	if err != nil {
		return nil, err
	}
	item.Items = marks
	cw, ch := w+pad.left+pad.right, h+pad.top+pad.bottom
	av, err := s.signal("autosize")
	if err != nil {
		return nil, err
	}
	kind := autosize(av)
	if kind != "none" {
		// The canvas grows to hold the guides, and the plot area moves
		// inside it, as the autosize pad of Vega does.
		b := layout.Empty()
		st := rt.style()
		for i := range marks {
			if marks[i].Clip {
				// A mark that clips stays inside the plot area.
				continue
			}
			m := st.MarkBounds(&marks[i])
			if !m.IsEmpty() {
				b.X1, b.Y1 = math.Min(b.X1, m.X1+dx), math.Min(b.Y1, m.Y1+dy)
				b.X2, b.Y2 = math.Max(b.X2, m.X2+dx), math.Max(b.Y2, m.Y2+dy)
			}
		}
		if !b.IsEmpty() {
			x1, y1 := math.Min(0, b.X1), math.Min(0, b.Y1)
			x2, y2 := math.Max(w, b.X2), math.Max(h, b.Y2)
			item.X, item.Y = pad.left-x1+dx, pad.top-y1+dy
			cw, ch = math.Ceil(x2-x1+pad.left+pad.right), math.Ceil(y2-y1+pad.top+pad.bottom)
			// The autosize fit keeps the size of the spec for the whole
			// chart, so the plot area gives up what the guides take. With
			// "contains": "padding", the size of the spec holds the
			// padding too, for fit and for pad.
			padW, padH := 0.0, 0.0
			if containsPadding(av) {
				padW, padH = pad.left+pad.right, pad.top+pad.bottom
			}
			if kind == "fit" || kind == "fit-x" {
				rt.fitW = w - (x2 - x1 - w) - padW
				rt.wantW = w + pad.left + pad.right - padW
			} else if padW > 0 {
				rt.fitW = w - padW
			}
			if kind == "fit" || kind == "fit-y" {
				rt.fitH = h - (y2 - y1 - h) - padH
				rt.wantH = h + pad.top + pad.bottom - padH
			} else if padH > 0 {
				rt.fitH = h - padH
			}
		}
	} else if containsPadding(av) {
		// With the autosize none, the size of the spec holds the padding
		// too, and the canvas does not grow.
		rt.fitW = w - pad.left - pad.right
		rt.fitH = h - pad.top - pad.bottom
	}
	sc := &scene.Scene{
		Width:  cw,
		Height: ch,
		Root:   scene.Mark{Type: scene.Group, Role: "frame", Name: "root", Items: []scene.Item{*item}},
		// The licenses of the icons that the scene draws ask for these
		// credits (D31).
		Credits: rt.credits,
	}
	if err := rt.missingGlyphs(&sc.Root, map[rune]bool{}); err != nil {
		return nil, err
	}
	bg, err := s.signal("background")
	if err != nil {
		return nil, err
	}
	switch c, ok := bg.(string); {
	case rt.opts.hasBackground:
		if rt.opts.background != "transparent" {
			sc.Background = scene.Paint{Color: rt.opts.background}
		}
	case ok && c != "":
		sc.Background = scene.Paint{Color: c}
	case bg == nil || expr.IsUndefined(bg):
		// The background of the config of the spec goes over the theme.
		sc.Background = scene.Paint{Color: rt.theme.Background}
		if v, ok := rt.config["background"].(string); ok && v != "" {
			sc.Background = scene.Paint{Color: v}
		}
	}
	return sc, nil
}
