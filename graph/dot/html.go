package dot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xo/okage/html"

	"github.com/xo/zuzu/graph"
)

// htmlMark starts the value of an attribute that came as a string of HTML,
// <...>, so that it is told apart from a string in quotes.
const htmlMark = "\x00"

// isHTML reports whether the value of an attribute is a string of HTML,
// and returns it with no mark.
func isHTML(v string) (string, bool) {
	return strings.CutPrefix(v, htmlMark)
}

// html reads a label of HTML with the package html of okage (D31), and
// adds an issue for each part that it does not read and each color that
// zuzu does not know in the color scheme. zuzu reads the gradientangle and
// the style radial of a table or a cell itself.
func (p *parser) html(src, scheme, path string) (*html.Label, error) {
	src, _, _ = graph.HTMLGradients(src)
	l, issues, err := html.Parse(src)
	if err != nil {
		if se, ok := errors.AsType[*html.SyntaxError](err); ok {
			return nil, fmt.Errorf("%s: a label of HTML at %s: %s: %w", path, se.Pos, se.Msg, ErrSyntax)
		}
		return nil, fmt.Errorf("%s: a label of HTML: %w: %w", path, ErrSyntax, err)
	}
	for _, is := range issues {
		if err := p.issues.Add(path, "a label of HTML at %s: %s", is.Pos, is.Msg); err != nil {
			return nil, err
		}
	}
	for _, v := range htmlColors(l) {
		// A fill of two colors is a gradient.
		colors, _, ok := graph.ColorList(v)
		if !ok {
			if err := p.issues.Add(path, "the weights of the colors %s", v); err != nil {
				return nil, err
			}
		}
		if len(colors) > 2 {
			if err := p.issues.Add(path, "the colors after the second of the gradient %s", v); err != nil {
				return nil, err
			}
			colors = colors[:2]
		}
		for _, c := range colors {
			if _, ok := graph.ColorIn(c, scheme); !ok {
				if err := p.issues.Add(path, "the color %s", c); err != nil {
					return nil, err
				}
			}
		}
	}
	return l, nil
}

// htmlColors returns the colors that a label of HTML names.
func htmlColors(l *html.Label) []string {
	var out []string
	add := func(v string) {
		if v != "" {
			out = append(out, v)
		}
	}
	lines := func(ls []html.Line) {
		for _, ln := range ls {
			for _, s := range ln.Spans {
				add(s.Font.Color)
			}
		}
	}
	var table func(t *html.Table)
	table = func(t *html.Table) {
		add(t.BgColor)
		add(t.Color)
		for _, r := range t.Rows {
			for _, c := range r.Cells {
				add(c.BgColor)
				add(c.Color)
				lines(c.Lines)
				if c.Table != nil {
					table(c.Table)
				}
			}
		}
	}
	lines(l.Lines)
	if l.Table != nil {
		table(l.Table)
	}
	return out
}

// htmlText returns the text of a label of HTML: its lines, and the lines of
// the cells of a table one under another.
func htmlText(l *html.Label) string {
	var out []string
	lines := func(ls []html.Line) {
		for _, ln := range ls {
			var b strings.Builder
			for _, s := range ln.Spans {
				b.WriteString(s.Text)
			}
			out = append(out, strings.TrimSpace(b.String()))
		}
	}
	var table func(t *html.Table)
	table = func(t *html.Table) {
		for _, r := range t.Rows {
			for _, c := range r.Cells {
				lines(c.Lines)
				if c.Table != nil {
					table(c.Table)
				}
			}
		}
	}
	lines(l.Lines)
	if l.Table != nil {
		table(l.Table)
	}
	return strings.Join(out, "\n")
}

// label returns the text of a label of an edge, a cluster, the graph or an
// xlabel, and the source of its HTML when it is a label of HTML. The
// label of HTML draws through the package html of okage, as the label of
// a node does, and its text is the text of the label. A color of the
// label of HTML reads in the color scheme.
func (p *parser) label(v, id, scheme, path string) (string, string, error) {
	src, ok := isHTML(v)
	if !ok {
		return label(v, id), "", nil
	}
	l, err := p.html(src, scheme, path)
	if err != nil {
		return "", "", err
	}
	return htmlText(l), graph.HTMLInScheme(src, scheme), nil
}

// htmlRecord reads the label of HTML of a record, as Graphviz does: the
// braces and the bars outside the tags split the fields, and the text of
// each field is HTML. A field draws its lines in their fonts, at the left
// when each of its lines ends with <br align="left"/>. A table in a field,
// a line in more than one font, a font with a color or a size, and a line
// at the right are issues. A face gives the field its weight and its style.
func (p *parser) htmlRecord(src, scheme, path string) ([]graph.Field, error) {
	var out []graph.Field
	for _, part := range splitRecord(src) {
		if strings.TrimSpace(part) == "" {
			continue
		}
		l, err := p.html(part, scheme, path)
		if err != nil {
			return nil, err
		}
		if l.Table != nil {
			if err := p.issues.Add(path, "a table in a field of a record"); err != nil {
				return nil, err
			}
			out = append(out, graph.Field{Text: htmlText(l)})
			continue
		}
		f, err := p.htmlField(l.Lines, path)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// htmlField returns the field of a record that the lines of a label of
// HTML draw.
func (p *parser) htmlField(lines []html.Line, path string) (graph.Field, error) {
	var f graph.Field
	var texts []string
	fonts, left, other := false, 0, 0
	for _, ln := range lines {
		var b strings.Builder
		var font graph.Font
		mixed, first := false, true
		for _, s := range ln.Spans {
			b.WriteString(s.Text)
			if strings.TrimSpace(s.Text) == "" {
				continue
			}
			ft := graph.Font{Bold: s.Font.Bold, Italic: s.Font.Italic, Underline: s.Font.Underline}
			x := s.Font
			// A face gives a weight and a style, such as Helvetica-Bold,
			// and zuzu draws the rest of its name in IBM Plex Sans (D32).
			// A field draws bold or not.
			weight, style := graph.FontFace(x.Face)
			switch weight {
			case "":
			case graph.BoldWeight:
				ft.Bold = true
			default:
				if err := p.issues.Add(path, "the weight %s of the face %s in a field of a record", weight, x.Face); err != nil {
					return f, err
				}
			}
			ft.Italic = ft.Italic || style != ""
			if first {
				font, first = ft, false
			} else if ft != font {
				mixed = true
			}
			if x.Color != "" || x.Size != 0 || x.Overline || x.Strike || x.Sub || x.Sup {
				if err := p.issues.Add(path, "a font with a color or a size, or a line over or through its text, in a field of a record"); err != nil {
					return f, err
				}
			}
		}
		if mixed {
			if err := p.issues.Add(path, "a line in more than one font in a field of a record"); err != nil {
				return f, err
			}
		}
		texts = append(texts, strings.TrimSpace(b.String()))
		f.Fonts = append(f.Fonts, font)
		fonts = fonts || font != graph.Font{}
		switch ln.Align {
		case html.AlignLeft:
			left++
		case html.AlignRight:
			if err := p.issues.Add(path, "a line at the right in a field of a record"); err != nil {
				return f, err
			}
			other++
		default:
			other++
		}
	}
	switch {
	case left > 0 && other == 0:
		f.Align = "l"
	case left > 0:
		if err := p.issues.Add(path, "lines at the left and at the center in a field of a record"); err != nil {
			return f, err
		}
	}
	if !fonts {
		f.Fonts = nil
	}
	f.Text = strings.Join(texts, "\n")
	return f, nil
}

// splitRecord splits the label of HTML of a record into the sources of its
// fields, flat, as record does: a brace or a bar outside a tag ends a
// field. \{, \} and \| are text.
func splitRecord(src string) []string {
	var out []string
	var b strings.Builder
	inTag := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inTag:
			inTag = c != '>'
		case c == '<':
			inTag = true
		case c == '\\' && i+1 < len(src) && strings.IndexByte("{}|", src[i+1]) >= 0:
			i++
			c = src[i]
		case c == '{' || c == '}' || c == '|':
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	return append(out, b.String())
}

// wholeGroup reports whether the label of a record is one group in braces,
// {a|b}. Its fields lie in the other direction, as in Graphviz.
func wholeGroup(v string) bool {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "{") || !strings.HasSuffix(v, "}") || strings.HasSuffix(v, `\}`) {
		return false
	}
	depth := 0
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 && i < len(v)-1 {
				return false
			}
		}
	}
	return depth == 0
}
