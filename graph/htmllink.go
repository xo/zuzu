package graph

import (
	"cmp"
	"html"
	"strings"
)

// HTMLLink is the link and the tooltip of a table or a cell of a label of
// HTML of DOT, or empty (D32).
type HTMLLink struct {
	Href, Tooltip string
}

// HTMLLinks reads the attributes href, URL, tooltip and title of each
// <table> and each <td> of a label of HTML, which the package html of
// okage does not read. href wins over URL, and tooltip over title, as in
// Graphviz. It returns the source without them, and the links of the
// tables and of the cells, each in the order of the source. Without fill,
// each attribute turns into spaces, so that each other part keeps its line
// and its column. With fill, a table or a cell with a link and no bgcolor
// takes the bgcolor transparent in the place of its first attribute of a
// link, so that okage gives it a box, which takes the link.
func HTMLLinks(src string, fill bool) (string, []HTMLLink, []HTMLLink) {
	type edit struct {
		from, to int
		text     string
	}
	var edits []edit
	var tables, cells []HTMLLink
	var attrs map[string]string
	var spans []edit
	bg := false
	attr := func(a htmlAttr) {
		if a.tag != "table" && a.tag != "td" {
			return
		}
		switch a.name {
		case "href", "url", "tooltip", "title":
			if attrs == nil {
				attrs = map[string]string{}
			}
			attrs[a.name] = html.UnescapeString(src[a.vs:a.ve])
			spans = append(spans, edit{a.start, a.ve + 1, strings.Repeat(" ", a.ve+1-a.start)})
		case "bgcolor":
			bg = true
		}
	}
	tag := func(name string) {
		if name == "table" || name == "td" {
			l := HTMLLink{Href: cmp.Or(attrs["href"], attrs["url"]), Tooltip: cmp.Or(attrs["tooltip"], attrs["title"])}
			if fill && !bg && (l.Href != "" || l.Tooltip != "") && len(spans) > 0 {
				spans[0].text = `bgcolor="transparent"`
			}
			edits = append(edits, spans...)
			if name == "table" {
				tables = append(tables, l)
			} else {
				cells = append(cells, l)
			}
		}
		attrs, spans, bg = nil, nil, false
	}
	scanHTML(src, attr, tag)
	if len(edits) == 0 {
		return src, tables, cells
	}
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(src[at:e.from])
		b.WriteString(e.text)
		at = e.to
	}
	b.WriteString(src[at:])
	return b.String(), tables, cells
}

// htmlLinks reports whether a table or a cell of a label of HTML of a graph
// has a link or a tooltip, so that the marks of the graph encode them.
func htmlLinks(g *Graph) bool {
	srcs := []string{g.TitleHTML}
	for _, n := range g.Nodes {
		srcs = append(srcs, n.HTML, n.XHTML)
		for _, f := range n.Fields {
			srcs = append(srcs, f.Text)
		}
	}
	for _, e := range g.Edges {
		srcs = append(srcs, e.HTML, e.HeadHTML, e.TailHTML, e.XHTML)
	}
	for _, c := range g.Clusters {
		srcs = append(srcs, c.HTML)
	}
	for _, src := range srcs {
		if !strings.Contains(src, "<") {
			continue
		}
		_, tables, cells := HTMLLinks(src, false)
		for _, l := range append(tables, cells...) {
			if l != (HTMLLink{}) {
				return true
			}
		}
	}
	return false
}
