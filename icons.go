package zuzu

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // An image of a node can be a JPEG (D31).
	_ "image/png"  // An image of a node can be a PNG (D31).
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/xo/zuzu/icon"
)

// IconLoader gives the icons that no pack of WithIcons holds (D31). zuzu
// asks it once for each icon of a render, by its prefix and its name, and
// it gives the bytes from wherever it chooses. zuzu itself reads no file
// and makes no request.
type IconLoader interface {
	LoadIcon(ctx context.Context, req IconRequest) (io.ReadCloser, error)
}

// IconRequest names an icon: aws and lambda, for aws:lambda.
type IconRequest struct {
	Prefix, Name string
}

// IconLoaderFunc makes an IconLoader of a function.
type IconLoaderFunc func(context.Context, IconRequest) (io.ReadCloser, error)

// LoadIcon calls the function.
func (f IconLoaderFunc) LoadIcon(ctx context.Context, req IconRequest) (io.ReadCloser, error) {
	return f(ctx, req)
}

// FSIconLoader reads an icon from fsys: the SVG <prefix>/<name>.svg, or
// else the pack of Iconify <prefix>.json. The caller chooses fsys, so zuzu
// still reads only what the caller allows.
func FSIconLoader(fsys fs.FS) IconLoader {
	return IconLoaderFunc(func(_ context.Context, req IconRequest) (io.ReadCloser, error) {
		if !fs.ValidPath(req.Prefix) || !fs.ValidPath(req.Name) || strings.Contains(req.Name, "/") {
			return nil, fmt.Errorf("the icon %s:%s: %w", req.Prefix, req.Name, icon.ErrMissing)
		}
		f, err := fsys.Open(path.Join(req.Prefix, req.Name+".svg"))
		if err == nil {
			return f, nil
		}
		f, err2 := fsys.Open(req.Prefix + ".json")
		if err2 != nil {
			return nil, errors.Join(err, err2)
		}
		return f, nil
	})
}

// WithIcons gives a pack of icons to a render, under a name that replaces
// the prefix of the pack, as Mermaid does (D31). A diagram names an icon of
// the pack as name:icon.
func WithIcons(name string, p *icon.Pack) RenderOption {
	return func(o *renderOptions) {
		if o.icons == nil {
			o.icons = map[string]*icon.Pack{}
		}
		o.icons[name] = p
	}
}

// WithIconLoader sets how a render loads the icons that no pack holds.
func WithIconLoader(l IconLoader) RenderOption {
	return func(o *renderOptions) { o.iconLoader = l }
}

// iconEntry is an icon of a render, or the error that loading it gave.
type iconEntry struct {
	ic  *icon.Icon
	err error
}

// Icon returns an icon by its name, pack:name: from a pack of WithIcons,
// from the built-in icons for a name with no pack, or from the icon loader.
// A render loads each icon once.
func (e *dataEnv) Icon(name string) (*icon.Icon, error) {
	rt := e.s.rt
	if en, ok := rt.icons[name]; ok {
		return en.ic, en.err
	}
	ic, err := rt.loadIcon(name)
	if rt.icons == nil {
		rt.icons = map[string]iconEntry{}
	}
	rt.icons[name] = iconEntry{ic, err}
	if err == nil && ic.Credit != "" && !strings.Contains(strings.Join(rt.credits, "\n"), ic.Credit) {
		rt.credits = append(rt.credits, ic.Credit)
	}
	return ic, err
}

// loadIcon finds an icon.
func (rt *runtime) loadIcon(name string) (*icon.Icon, error) {
	prefix, local, ok := strings.Cut(name, ":")
	if !ok {
		if ic, ok := icon.Builtin(name); ok {
			return ic, nil
		}
		return nil, fmt.Errorf("the icon %q: %w", name, icon.ErrMissing)
	}
	if p, ok := rt.opts.icons[prefix]; ok {
		if ic, ok := p.Icon(local); ok {
			return ic, nil
		}
	}
	if rt.opts.iconLoader == nil {
		return nil, fmt.Errorf("the icon %q: %w", name, icon.ErrMissing)
	}
	r, err := rt.opts.iconLoader.LoadIcon(rt.ctx, IconRequest{Prefix: prefix, Name: local})
	if err != nil {
		return nil, fmt.Errorf("the icon %q: %w", name, err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("the icon %q: %w", name, err)
	}
	return icon.Decode(b, local)
}

// Image loads an image of a node by its url, with the loader of data, as a
// URL of data with its size. zuzu reads a PNG, a JPEG and an SVG, and
// cleans an SVG first (D31).
func (e *dataEnv) Image(url string) (string, float64, float64, error) {
	rt := e.s.rt
	if rt.opts.loader == nil {
		return "", 0, 0, fmt.Errorf("the image %q: %w", url, ErrNoLoader)
	}
	r, err := rt.opts.loader.Load(rt.ctx, LoadRequest{URL: url, Format: "image"})
	if err != nil {
		return "", 0, 0, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", 0, 0, fmt.Errorf("reading %q: %w", url, err)
	}
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '<' {
		ic, err := icon.FromSVG(t)
		if err != nil {
			return "", 0, 0, err
		}
		doc, err := ic.SVG("", ic.Width, ic.Height)
		if err != nil {
			return "", 0, 0, err
		}
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(doc), ic.Width, ic.Height, nil
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return "", 0, 0, fmt.Errorf("reading the image %q: %w", url, err)
	}
	return "data:image/" + kind + ";base64," + base64.StdEncoding.EncodeToString(b), float64(cfg.Width), float64(cfg.Height), nil
}

// IconColor returns the color of an icon of one color: the fill of the
// style zuzu.icon of the config.
func (e *dataEnv) IconColor() string {
	if st, ok := e.s.rt.config["style"].(map[string]any); ok {
		if m, ok := st["zuzu.icon"].(map[string]any); ok {
			if c, ok := m["fill"].(string); ok {
				return c
			}
		}
	}
	return "currentColor"
}

// Issue handles a feature that does not draw: an error in the strict mode,
// and an issue of the report in the loose mode.
func (e *dataEnv) Issue(err error, path, format string, args ...any) error {
	return e.s.rt.issue(err, path, format, args...)
}
