// Package icon reads icons in the JSON format of Iconify, and in SVG, for
// the nodes of a diagram (D31). An icon of one color that holds only
// shapes draws as a path, and another icon draws as an image, an SVG that
// Clean makes safe.
package icon

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Error is an error of the package icon.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package.
const (
	// ErrInvalid is the error of a pack, an icon or an SVG that does not
	// read.
	ErrInvalid Error = "invalid icon"
	// ErrMissing is the error of an icon that a pack does not hold.
	ErrMissing Error = "missing icon"
)

// Icon is an icon: its body, the inside of an SVG element, its view box,
// and the turns and the flips that draw it.
type Icon struct {
	Body                     string
	Left, Top, Width, Height float64
	// Rotate is the number of quarter turns, clockwise.
	Rotate int
	// HFlip and VFlip flip the icon across and down.
	HFlip, VFlip bool
	// Credit is the attribution that the license of the pack of the icon
	// asks for, or empty.
	Credit string
}

// Info is what a pack says about itself.
type Info struct {
	Name, Version, Author, License, SPDX, URL string
}

// Pack is a pack of icons in the JSON format of Iconify.
type Pack struct {
	Prefix string
	Info   Info
	// Credit is the attribution that the license of the pack asks for, or
	// empty. Each icon of the pack carries it.
	Credit  string
	width   float64
	height  float64
	left    float64
	top     float64
	icons   map[string]jsonIcon
	aliases map[string]jsonIcon
}

// jsonIcon is an icon or an alias of a pack.
type jsonIcon struct {
	Body   *string  `json:"body"`
	Parent string   `json:"parent"`
	Left   *float64 `json:"left"`
	Top    *float64 `json:"top"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`
	Rotate int      `json:"rotate"`
	HFlip  bool     `json:"hFlip"`
	VFlip  bool     `json:"vFlip"`
}

// jsonPack is a pack of icons.
type jsonPack struct {
	Prefix  string              `json:"prefix"`
	Icons   map[string]jsonIcon `json:"icons"`
	Aliases map[string]jsonIcon `json:"aliases"`
	Width   *float64            `json:"width"`
	Height  *float64            `json:"height"`
	Left    float64             `json:"left"`
	Top     float64             `json:"top"`
	Info    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Author  struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"author"`
		License struct {
			Title string `json:"title"`
			SPDX  string `json:"spdx"`
		} `json:"license"`
	} `json:"info"`
}

// Read reads a pack of icons in the JSON format of Iconify.
func Read(b []byte) (*Pack, error) {
	var jp jsonPack
	if err := json.Unmarshal(b, &jp); err != nil {
		return nil, fmt.Errorf("reading a pack of icons: %w: %w", ErrInvalid, err)
	}
	if jp.Icons == nil {
		return nil, fmt.Errorf("a pack with no icons: %w", ErrInvalid)
	}
	p := &Pack{
		Prefix: jp.Prefix, icons: jp.Icons, aliases: jp.Aliases, width: 16, height: 16, left: jp.Left, top: jp.Top,
		Info: Info{
			Name: jp.Info.Name, Version: jp.Info.Version, Author: jp.Info.Author.Name, URL: jp.Info.Author.URL,
			License: jp.Info.License.Title, SPDX: jp.Info.License.SPDX,
		},
	}
	if jp.Width != nil {
		p.width = *jp.Width
	}
	if jp.Height != nil {
		p.height = *jp.Height
	}
	if jp.Width != nil && jp.Height == nil {
		p.height = p.width
	}
	if jp.Height != nil && jp.Width == nil {
		p.width = p.height
	}
	return p, nil
}

// Names returns the names of the icons and the aliases of a pack, in order.
func (p *Pack) Names() []string {
	names := slices.Collect(maps.Keys(p.icons))
	names = slices.AppendSeq(names, maps.Keys(p.aliases))
	slices.Sort(names)
	return names
}

// Icon returns an icon of a pack by its name or the name of an alias. An
// alias adds its turns and its flips to those of its parent, and its view
// box replaces that of its parent.
func (p *Pack) Icon(name string) (*Icon, bool) {
	var chain []jsonIcon
	for range 8 {
		if ji, ok := p.icons[name]; ok {
			chain = append(chain, ji)
			break
		}
		ji, ok := p.aliases[name]
		if !ok || ji.Parent == "" {
			return nil, false
		}
		chain = append(chain, ji)
		name = ji.Parent
	}
	root := chain[len(chain)-1]
	if root.Body == nil {
		return nil, false
	}
	ic := &Icon{Body: *root.Body, Left: p.left, Top: p.top, Width: p.width, Height: p.height, Credit: p.Credit}
	for _, ji := range slices.Backward(chain) {
		if ji.Left != nil {
			ic.Left = *ji.Left
		}
		if ji.Top != nil {
			ic.Top = *ji.Top
		}
		if ji.Width != nil {
			ic.Width = *ji.Width
		}
		if ji.Height != nil {
			ic.Height = *ji.Height
		}
		ic.Rotate = (ic.Rotate + ji.Rotate) % 4
		ic.HFlip = ic.HFlip != ji.HFlip
		ic.VFlip = ic.VFlip != ji.VFlip
	}
	return ic, true
}

// Decode reads the bytes of one icon: an SVG document, an icon of Iconify,
// or a pack of Iconify that holds the icon of the name.
func Decode(b []byte, name string) (*Icon, error) {
	t := bytes.TrimSpace(b)
	if len(t) == 0 {
		return nil, fmt.Errorf("an empty icon: %w", ErrInvalid)
	}
	if t[0] == '<' {
		return FromSVG(t)
	}
	var probe map[string]any
	if err := json.Unmarshal(t, &probe); err != nil {
		return nil, fmt.Errorf("reading an icon: %w: %w", ErrInvalid, err)
	}
	if _, ok := probe["icons"]; ok {
		p, err := Read(t)
		if err != nil {
			return nil, err
		}
		ic, ok := p.Icon(name)
		if !ok {
			return nil, fmt.Errorf("the icon %q: %w", name, ErrMissing)
		}
		return ic, nil
	}
	var ji jsonIcon
	if err := json.Unmarshal(t, &ji); err != nil || ji.Body == nil {
		return nil, fmt.Errorf("reading an icon: %w", ErrInvalid)
	}
	p := &Pack{width: 16, height: 16, icons: map[string]jsonIcon{name: ji}}
	ic, _ := p.Icon(name)
	return ic, nil
}

// Render returns the view box and the body of an icon with its turns and
// its flips applied, as Iconify draws an icon with no change of its own:
// a flip and a turn wrap the body in a group with a transform, and a
// quarter turn swaps the sides of the view box.
func (ic *Icon) Render() ([4]float64, string) {
	left, top, w, h := ic.Left, ic.Top, ic.Width, ic.Height
	body := ic.Body
	var ts []string
	rot := ic.Rotate
	switch {
	case ic.HFlip && ic.VFlip:
		rot += 2
	case ic.HFlip:
		ts = append(ts, "translate("+num(w+left)+" "+num(-top)+")", "scale(-1 1)")
		left, top = 0, 0
	case ic.VFlip:
		ts = append(ts, "translate("+num(-left)+" "+num(h+top)+")", "scale(1 -1)")
		left, top = 0, 0
	}
	rot = ((rot % 4) + 4) % 4
	switch rot {
	case 1:
		c := h/2 + top
		ts = append([]string{"rotate(90 " + num(c) + " " + num(c) + ")"}, ts...)
	case 2:
		ts = append([]string{"rotate(180 " + num(w/2+left) + " " + num(h/2+top) + ")"}, ts...)
	case 3:
		c := w/2 + left
		ts = append([]string{"rotate(-90 " + num(c) + " " + num(c) + ")"}, ts...)
	}
	if rot%2 == 1 {
		left, top = top, left
		w, h = h, w
	}
	if len(ts) > 0 {
		defs, content := splitDefs(body)
		body = `<g transform="` + strings.Join(ts, " ") + `">` + content + "</g>"
		if defs != "" {
			body = "<defs>" + defs + "</defs>" + body
		}
	}
	return [4]float64{left, top, w, h}, body
}

// splitDefs takes the definitions out of a body, so that a transform of
// the body leaves them as they are.
func splitDefs(body string) (string, string) {
	var defs, content strings.Builder
	for {
		i := strings.Index(body, "<defs")
		if i < 0 {
			break
		}
		open := strings.Index(body[i:], ">")
		if open < 0 {
			break
		}
		if body[i+open-1] == '/' {
			content.WriteString(body[:i])
			body = body[i+open+1:]
			continue
		}
		end := strings.Index(body[i:], "</defs>")
		if end < 0 {
			break
		}
		content.WriteString(body[:i])
		defs.WriteString(body[i+open+1 : i+end])
		body = body[i+end+len("</defs>"):]
	}
	content.WriteString(body)
	return defs.String(), content.String()
}

// SVG returns the icon as an SVG document of a size, with currentColor in
// a color, cleaned as Clean cleans it.
func (ic *Icon) SVG(color string, w, h float64) ([]byte, error) {
	vb, body := ic.Render()
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"`)
	fmt.Fprintf(&b, ` width="%s" height="%s" viewBox="%s %s %s %s"`, num(w), num(h), num(vb[0]), num(vb[1]), num(vb[2]), num(vb[3]))
	if color != "" {
		fmt.Fprintf(&b, ` color="%s"`, escape(color))
	}
	b.WriteString(">")
	b.WriteString(body)
	b.WriteString("</svg>")
	return Clean([]byte(b.String()))
}

// num writes a number as JavaScript does for the numbers of an icon.
func num(v float64) string {
	if v == 0 {
		// JavaScript writes -0 as 0.
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
