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
// zuzu does not know.
func (p *parser) html(src, path string) (*html.Label, error) {
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
	for _, c := range htmlColors(l) {
		if _, ok := graph.Color(c); !ok {
			if err := p.issues.Add(path, "the color %s", c); err != nil {
				return nil, err
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
// a node does, and its text is the text of the label.
func (p *parser) label(v, id, path string) (string, string, error) {
	src, ok := isHTML(v)
	if !ok {
		return label(v, id), "", nil
	}
	l, err := p.html(src, path)
	if err != nil {
		return "", "", err
	}
	return htmlText(l), src, nil
}
