package dataflow

import (
	"encoding/base64"
	"fmt"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/icon"
)

// IconEnv is an Env that gives the icons and the images of the nodes of a
// diagram (D31). The transform zuzu.icon needs it.
type IconEnv interface {
	// Icon returns an icon by its name, pack:name, or an error for an icon
	// that no pack and no loader gives.
	Icon(name string) (*icon.Icon, error)
	// Image returns an image by its url, as a URL of data, with its size.
	Image(url string) (string, float64, float64, error)
	// IconColor returns the color of an icon of one color.
	IconColor() string
	// Issue handles a feature that does not draw: an error in the strict
	// mode, and an issue of the report in the loose mode.
	Issue(err error, path, format string, args ...any) error
}

// iconOf resolves the icon or the image of each row of a node: an icon of
// one color as a path that fits its size, centered at 0, 0, and another
// icon or an image as a URL of data. A missing icon or image draws as a box
// with a question mark (D31). The fields that it writes are mediaWidth,
// mediaHeight, iconKind, which is fill, stroke or image, iconPath,
// iconStrokeWidth, iconURL and mediaAspect.
func iconOf(p *params, rows []Row) ([]Row, error) {
	env, _ := p.env.(IconEnv)
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		o := clone(r)
		out = append(out, o)
		name, url := strOf(r["icon"]), strOf(r["image"])
		if name == "" && url == "" {
			continue
		}
		size := expr.Number(r["iconSize"])
		if !(size > 0) {
			size = 48
		}
		var ic *icon.Icon
		if url != "" {
			w, h := expr.Number(r["imageWidth"]), expr.Number(r["imageHeight"])
			data, nw, nh, err := loadImage(env, url)
			if err == nil {
				switch {
				case w > 0 && h > 0:
				case w > 0:
					h = w * nh / nw
				case h > 0:
					w = h * nw / nh
				default:
					w, h = nw, nh
				}
				aspect, _ := r["imageAspect"].(bool)
				o["iconKind"], o["iconURL"], o["mediaWidth"], o["mediaHeight"], o["mediaAspect"] = "image", data, w, h, aspect
				continue
			}
			if err := issue(env, err, "an image that does not load, drawn as a box with a question mark"); err != nil {
				return nil, err
			}
			ic = icon.Missing()
		} else {
			var err error
			if env != nil {
				ic, err = env.Icon(name)
			} else {
				err = fmt.Errorf("the icon %q: %w", name, icon.ErrMissing)
			}
			if err != nil {
				if err := issue(env, err, "an icon that does not load, drawn as a box with a question mark"); err != nil {
					return nil, err
				}
				ic = icon.Missing()
			}
		}
		vb, _ := ic.Render()
		k := size / max(vb[2], vb[3])
		o["mediaWidth"], o["mediaHeight"] = vb[2]*k, vb[3]*k
		if s, ok := ic.Shape(); ok {
			path, sw, err := s.Scale(vb, size)
			if err == nil {
				o["iconKind"], o["iconPath"], o["iconStrokeWidth"] = "fill", path, sw
				if s.Stroke {
					o["iconKind"] = "stroke"
				}
				continue
			}
		}
		color := "currentColor"
		if env != nil {
			color = env.IconColor()
		}
		doc, err := ic.SVG(color, vb[2]*k, vb[3]*k)
		if err != nil {
			if err := issue(env, err, "the icon %q, which does not draw", name); err != nil {
				return nil, err
			}
			continue
		}
		o["iconKind"], o["iconURL"], o["mediaAspect"] = "image", "data:image/svg+xml;base64,"+base64.StdEncoding.EncodeToString(doc), true
	}
	return out, nil
}

// loadImage loads an image through the environment.
func loadImage(env IconEnv, url string) (string, float64, float64, error) {
	if env == nil {
		return "", 0, 0, fmt.Errorf("the image %q: %w", url, ErrParam)
	}
	data, w, h, err := env.Image(url)
	if err == nil && (!(w > 0) || !(h > 0)) {
		err = fmt.Errorf("the image %q has no size: %w", url, ErrParam)
	}
	return data, w, h, err
}

// issue reports an icon or an image that does not draw.
func issue(env IconEnv, err error, format string, args ...any) error {
	if env == nil {
		return nil
	}
	return env.Issue(err, "transform zuzu.icon", format, args...)
}

// strOf returns a value as a string, or empty for null or undefined.
func strOf(v any) string {
	if v == nil || expr.IsUndefined(v) {
		return ""
	}
	return expr.String(v)
}
