package icon

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// dropped are the elements that Clean drops, with all that they hold.
var dropped = map[string]bool{"script": true, "foreignObject": true, "iframe": true, "embed": true, "object": true, "audio": true, "video": true}

// urlRE matches a url() of CSS, and importRE an @import of CSS.
var (
	urlRE    = regexp.MustCompile(`(?i)url\(\s*(['"]?)([^'")]*)(['"]?)\s*\)`)
	importRE = regexp.MustCompile(`(?i)@import[^;]*;?`)
)

// Clean makes an SVG document safe to draw and to write into another: it
// drops scripts, foreignObject and the like, the handlers of events, and
// every reference outside the document, such as an href, a url() or an
// @import. A reference inside the document, #id, and a URL of data stay.
func Clean(src []byte) ([]byte, error) {
	d := xml.NewDecoder(strings.NewReader(string(src)))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	var b strings.Builder
	skip := 0
	style := false
	for {
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading an SVG: %w: %w", ErrInvalid, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 || dropped[t.Name.Local] {
				skip++
				continue
			}
			style = t.Name.Local == "style"
			b.WriteString("<" + qname(t.Name))
			for _, a := range t.Attr {
				v, ok := cleanAttr(a)
				if !ok {
					continue
				}
				b.WriteString(" " + qname(a.Name) + `="` + escape(v) + `"`)
			}
			b.WriteString(">")
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			style = false
			b.WriteString("</" + qname(t.Name) + ">")
		case xml.CharData:
			if skip > 0 {
				continue
			}
			s := string(t)
			if style {
				s = cleanCSS(s)
			}
			b.WriteString(escapeText(s))
		case xml.ProcInst, xml.Directive, xml.Comment:
			// A declaration, a doctype or a comment draws nothing.
		}
	}
	return []byte(b.String()), nil
}

// cleanAttr cleans an attribute, or returns false to drop it.
func cleanAttr(a xml.Attr) (string, bool) {
	name := strings.ToLower(a.Name.Local)
	switch {
	case strings.HasPrefix(name, "on"):
		return "", false
	case name == "href" || name == "src":
		v := strings.TrimSpace(a.Value)
		if strings.HasPrefix(v, "#") || strings.HasPrefix(v, "data:image/") {
			return v, true
		}
		return "", false
	case name == "style":
		return cleanCSS(a.Value), true
	}
	if strings.Contains(strings.ToLower(a.Value), "url(") {
		return cleanCSS(a.Value), true
	}
	return a.Value, true
}

// cleanCSS drops each @import of a text of CSS, and each url() that names
// something outside the document.
func cleanCSS(s string) string {
	s = importRE.ReplaceAllString(s, "")
	return urlRE.ReplaceAllStringFunc(s, func(m string) string {
		u := strings.TrimSpace(urlRE.FindStringSubmatch(m)[2])
		if strings.HasPrefix(u, "#") || strings.HasPrefix(u, "data:image/") {
			return m
		}
		return "none"
	})
}

// qname writes a name with its prefix.
func qname(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

// escape escapes the value of an attribute.
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// escapeText escapes text.
func escapeText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// FromSVG reads an icon from an SVG document: its view box, or its width
// and its height, and the inside of its root, cleaned.
func FromSVG(src []byte) (*Icon, error) {
	clean, err := Clean(src)
	if err != nil {
		return nil, err
	}
	s := string(clean)
	i := strings.Index(s, "<svg")
	if i < 0 {
		return nil, fmt.Errorf("an SVG with no svg element: %w", ErrInvalid)
	}
	end := strings.Index(s[i:], ">")
	last := strings.LastIndex(s, "</svg>")
	if end < 0 || last < i+end {
		return nil, fmt.Errorf("an SVG with no end: %w", ErrInvalid)
	}
	d := xml.NewDecoder(strings.NewReader(s[i : i+end+1]))
	d.Strict = false
	tok, err := d.RawToken()
	if err != nil {
		return nil, fmt.Errorf("reading an SVG: %w: %w", ErrInvalid, err)
	}
	root, ok := tok.(xml.StartElement)
	if !ok {
		return nil, fmt.Errorf("an SVG with no svg element: %w", ErrInvalid)
	}
	ic := &Icon{Body: s[i+end+1 : last]}
	var w, h float64
	for _, a := range root.Attr {
		switch a.Name.Local {
		case "viewBox":
			f := strings.FieldsFunc(a.Value, func(r rune) bool { return r == ',' || r == ' ' })
			if len(f) == 4 {
				var v [4]float64
				for k := range f {
					v[k], _ = strconv.ParseFloat(f[k], 64)
				}
				ic.Left, ic.Top, ic.Width, ic.Height = v[0], v[1], v[2], v[3]
			}
		case "width":
			w, _ = strconv.ParseFloat(strings.TrimSuffix(a.Value, "px"), 64)
		case "height":
			h, _ = strconv.ParseFloat(strings.TrimSuffix(a.Value, "px"), 64)
		}
	}
	if ic.Width <= 0 || ic.Height <= 0 {
		ic.Width, ic.Height = w, h
	}
	if ic.Width <= 0 || ic.Height <= 0 {
		return nil, fmt.Errorf("an SVG with no size: %w", ErrInvalid)
	}
	return ic, nil
}
