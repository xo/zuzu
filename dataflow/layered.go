package dataflow

import (
	"fmt"
	"math"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
)

// layered lays out a graph in ranks (D31). It runs on the rows of the
// nodes, reads the edges and the clusters from the datasets that it names,
// and makes one row for each node, each edge and each cluster, which the
// field kind tells apart.
func layered(p *params, rows []Row) ([]Row, error) {
	str := func(name, def string) (*Accessor, error) {
		if _, ok := p.t.Param(name); !ok {
			return FieldAccessor(def), nil
		}
		return p.field(name)
	}
	edgesName, err := p.str("edges", "")
	if err != nil {
		return nil, err
	}
	if edgesName == "" {
		return nil, fmt.Errorf("the edges: %w", ErrParam)
	}
	edges, err := p.env.Data(edgesName)
	if err != nil {
		return nil, err
	}
	clustersName, err := p.str("clusters", "")
	if err != nil {
		return nil, err
	}
	var clusters []Row
	if clustersName != "" {
		if clusters, err = p.env.Data(clustersName); err != nil {
			return nil, err
		}
	}
	fields := map[string]*Accessor{}
	for name, def := range map[string]string{
		"key": "key", "cluster": "cluster", "source": "source", "target": "target", "minlen": "minlen",
		"weight": "weight", "rank": "rank", "width": "width", "height": "height", "label": "label", "shape": "shape",
	} {
		if fields[name], err = str(name, def); err != nil {
			return nil, err
		}
	}
	dir, err := p.str("direction", graph.TB)
	if err != nil {
		return nil, err
	}
	switch dir {
	case graph.TB, graph.BT, graph.LR, graph.RL:
	default:
		return nil, fmt.Errorf("the direction %q: %w", dir, ErrParam)
	}
	num := func(name string, def float64) float64 {
		if err != nil {
			return 0
		}
		var v float64
		v, err = p.number(name, def)
		return v
	}
	rankSep, nodeSep, pad := num("rankSep", 40), num("nodeSep", 24), num("clusterPadding", 12)
	fontSize, padding := num("fontSize", 11), num("padding", 8)
	// The space above and below the title of a cluster.
	titleTop, titleBottom := num("clusterTitleTop", 0), num("clusterTitleBottom", 0)
	if err != nil {
		return nil, err
	}
	breakWords, err := p.boolean("breakWords", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as("x", "y", "width", "height", "rank", "order")
	if err != nil {
		return nil, err
	}
	te, _ := p.env.(TextEnv)
	sizeAt := func(text, font string, size float64, weight string) (float64, float64) {
		if te == nil {
			return 0.6 * size * float64(len([]rune(text))), size
		}
		return te.TextSize(text, font, size, weight)
	}
	measureAt := func(v any, fontSize float64, weight string) (float64, float64) {
		if v == nil || expr.IsUndefined(v) {
			return 0, 0
		}
		s := expr.String(v)
		if s == "" {
			return 0, 0
		}
		var text any = s
		if strings.Contains(s, "\n") {
			var lines []any
			for l := range strings.SplitSeq(s, "\n") {
				lines = append(lines, l)
			}
			text = lines
		}
		if te == nil {
			n := float64(strings.Count(s, "\n") + 1)
			return 0.6 * fontSize * float64(len(s)) / n, fontSize * n
		}
		return te.TextSize(text, "", fontSize, weight)
	}
	// sizeOf returns the size of the font of a row, or the size of the
	// font of the layout.
	sizeOf := func(r Row, key string) float64 {
		if v := expr.Number(r[key]); v > 0 {
			return v
		}
		return fontSize
	}
	// weightOf returns the weight of the font of a row.
	weightOf := func(r Row) string {
		if w, ok := r["fontWeight"]; ok && w != nil {
			return expr.String(w)
		}
		return ""
	}
	// measureRow measures a text of a row in the size and the weight of
	// the font of the row, which a style of the source sets.
	measureRow := func(r Row, v any) (float64, float64) {
		return measureAt(v, sizeOf(r, "fontSize"), weightOf(r))
	}
	// wrapRow wraps a label of a row to a width, in the size and the
	// weight of the font of the row, or of the label of an edge. It returns
	// the label as lines that the measure reads, and the lines that the
	// text draws, or nil when the label does not wrap.
	wrapRow := func(v any, limit, size float64, weight string) (any, []any) {
		if !(limit > 0) || v == nil || expr.IsUndefined(v) {
			return v, nil
		}
		s := expr.String(v)
		if s == "" {
			return v, nil
		}
		ls := wrapText(s, limit, breakWords, func(l string) float64 {
			w, _ := sizeAt(l, "", size, weight)
			return w
		})
		out := make([]any, len(ls))
		for j, l := range ls {
			out[j] = l
		}
		return strings.Join(ls, "\n"), out
	}
	// other lays out a label that takes no space in the ranks: the label
	// of HTML in the field name+"HTML" of a row, or the text in
	// name+"Label". It returns the size of the label, and its layout for
	// HTML.
	other := func(r Row, name string, size float64) (float64, float64, *htmlLabel, error) {
		if src, ok := r[name+"HTML"].(string); ok && src != "" {
			h, err := layHTML(src, sizeAt, size)
			if err != nil {
				return 0, 0, nil, err
			}
			return h.lay.Width + 4, h.lay.Height + 2, h, nil
		}
		tw, th := measureAt(r[name+"Label"], size, "")
		if tw > 0 {
			return tw + 4, th + 2, nil, nil
		}
		return 0, 0, nil, nil
	}
	in := graph.LayoutInput{Direction: dir, RankSep: rankSep, NodeSep: nodeSep, ClusterPadding: pad}
	text := func(r Row, f *Accessor) string {
		v := f.Get(r)
		if v == nil || expr.IsUndefined(v) {
			return ""
		}
		return expr.String(v)
	}
	// A record lays its fields out across the ranks: side by side when the
	// ranks run down, and one under another when they run across. A record
	// with the field stack lays them one under another in every direction.
	across := dir == graph.TB || dir == graph.BT
	records := map[int]*record{}
	medias := map[int]*media{}
	htmls := map[int]*htmlLabel{}
	// wrapped holds the lines of each label that wraps, of a node or of
	// an edge, by its index.
	wrapped, edgeWrapped := map[int][]any{}, map[int][]any{}
	// xhtmls holds the xlabel of HTML of each node, and edgeHTMLs the
	// labels of HTML of each edge, by their names.
	xhtmls := map[int]*htmlLabel{}
	edgeHTMLs := map[int]map[string]*htmlLabel{}
	for i, r := range rows {
		if rec := readRecord(r, across); rec != nil {
			records[i] = rec
		}
	}
	for i, r := range rows {
		n := graph.LayoutNode{ID: text(r, fields["key"]), Cluster: text(r, fields["cluster"]), Rank: text(r, fields["rank"])}
		kind, ok := graph.ShapeOf(text(r, fields["shape"]))
		if !ok {
			kind = "rounded"
		}
		n.Shape = kind
		n.Width, n.Height = expr.Number(fields["width"].Get(r)), expr.Number(fields["height"].Get(r))
		h, ok, err := htmlOf(r, sizeAt, fontSize)
		if err != nil {
			return nil, fmt.Errorf("the label of HTML of node %s: %w", n.ID, err)
		}
		if ok {
			htmls[i] = h
			if !(n.Width > 0) || !(n.Height > 0) {
				n.Width, n.Height = h.size(kind, padding)
			}
		}
		if rec, ok := records[i]; ok {
			n.Width, n.Height = rec.size(func(v any, weight string) (float64, float64) {
				return measureAt(v, sizeOf(r, "fontSize"), weight)
			}, padding)
		}
		label := fields["label"].Get(r)
		if _, ok := records[i]; !ok && htmls[i] == nil {
			// A label wraps beside an icon in the line, so the icon
			// takes a part of the width.
			limit := expr.Number(r["wrap"])
			if m, ok := mediaOf(r, padding); ok && m.inline {
				limit = math.Max(limit-m.w-4, 1)
			}
			var ls []any
			if label, ls = wrapRow(label, limit, sizeOf(r, "fontSize"), weightOf(r)); ls != nil {
				wrapped[i] = ls
			}
		}
		if m, ok := mediaOf(r, padding); ok {
			medias[i] = m
			m.tw, m.th = measureRow(r, label)
			if m.inline {
				n.Width, n.Height = graph.NodeSize(kind, m.tw+m.w+4, math.Max(m.th, m.h), padding)
			} else {
				n.Shape = "rect"
				n.Width, n.Height = math.Max(m.fw, m.tw), m.fh
				if m.tw > 0 {
					n.Height += 4 + m.th
				}
			}
		}
		if !(n.Width > 0) || !(n.Height > 0) {
			tw, th := measureRow(r, label)
			if ms, ok := r["margin"].([]any); ok && len(ms) == 2 {
				// A margin of DOT is the space around the label in place
				// of the padding.
				n.Width, n.Height = graph.NodeSize(kind, tw+2*expr.Number(ms[0]), th+2*expr.Number(ms[1]), 0)
			} else {
				n.Width, n.Height = graph.NodeSize(kind, tw, th, padding)
			}
		}
		// A node of DOT with a width or a height and no fixedsize is at
		// least that size, and grows to fit its label.
		if v := expr.Number(r["minWidth"]); v > n.Width {
			n.Width = v
		}
		if v := expr.Number(r["minHeight"]); v > n.Height {
			n.Height = v
		}
		var xh *htmlLabel
		if n.XLabelWidth, n.XLabelHeight, xh, err = other(r, "x", sizeOf(r, "fontSize")); err != nil {
			return nil, fmt.Errorf("the xlabel of HTML of node %s: %w", n.ID, err)
		}
		if xh != nil {
			xhtmls[i] = xh
		}
		in.Nodes = append(in.Nodes, n)
	}
	index := map[string]int{}
	for i, n := range in.Nodes {
		index[n.ID] = i
	}
	// boxes holds the box of each port of each node, as x, y, width and
	// height from its center.
	boxes := map[int]map[string][4]float64{}
	for i, rec := range records {
		boxes[i] = rec.boxes(in.Nodes[i].Width, in.Nodes[i].Height)
	}
	for i, h := range htmls {
		boxes[i] = h.ports()
	}
	// port returns the place of a port of a node from its center, or nil,
	// and the direction out of the node when the port names a side. A port
	// is a name, a side such as e, or both, as p:e. A port names the center
	// of its box, and the edge leaves the border of the node on the line
	// from it. A side names a point of the box of the port, or of the node,
	// and the edge leaves it in the direction of the side.
	port := func(id string, v any) ([]float64, []float64) {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, nil
		}
		i, ok := index[id]
		if !ok {
			return nil, nil
		}
		n := in.Nodes[i]
		name, side, _ := strings.Cut(s, ":")
		b, ok := boxes[i][name]
		if !ok {
			if side != "" {
				return nil, nil
			}
			// A word that names no port can be a side of the node.
			side, b = name, [4]float64{-n.Width / 2, -n.Height / 2, n.Width, n.Height}
		}
		if side == "" {
			return []float64{b[0] + b[2]/2, b[1] + b[3]/2}, nil
		}
		at, dir, ok := compass(side, b)
		if !ok {
			return nil, nil
		}
		return at, dir
	}
	for i, r := range edges {
		e := graph.LayoutEdge{Source: text(r, fields["source"]), Target: text(r, fields["target"])}
		e.SourcePort, e.SourceSide = port(e.Source, r["sourcePort"])
		e.TargetPort, e.TargetSide = port(e.Target, r["targetPort"])
		if m := expr.Number(fields["minlen"].Get(r)); m > 0 {
			e.MinLen = int(m)
		}
		if w := expr.Number(fields["weight"].Get(r)); w > 0 {
			e.Weight = w
		}
		if c, ok := r["constraint"].(bool); ok && !c {
			e.NoConstraint = true
		}
		if c, ok := r["curve"].(string); ok {
			e.Curve = c
		}
		size, endSize := sizeOf(r, "fontSize"), sizeOf(r, "endFontSize")
		if _, ok := r["endFontSize"]; !ok {
			endSize = size
		}
		hs := map[string]*htmlLabel{}
		if src, ok := r["html"].(string); ok && src != "" {
			h, err := layHTML(src, sizeAt, size)
			if err != nil {
				return nil, fmt.Errorf("the label of HTML of the edge %s -> %s: %w", e.Source, e.Target, err)
			}
			hs["label"] = h
			e.LabelWidth, e.LabelHeight = h.lay.Width+8, h.lay.Height+4
		} else {
			label, ls := wrapRow(fields["label"].Get(r), expr.Number(r["wrap"]), size, "")
			if ls != nil {
				edgeWrapped[i] = ls
			}
			if lw, lh := measureAt(label, size, ""); lw > 0 {
				e.LabelWidth, e.LabelHeight = lw+8, lh+4
			}
		}
		for _, l := range []struct {
			name string
			size float64
			w, h *float64
		}{
			{"head", endSize, &e.HeadLabelWidth, &e.HeadLabelHeight},
			{"tail", endSize, &e.TailLabelWidth, &e.TailLabelHeight},
			{"x", size, &e.XLabelWidth, &e.XLabelHeight},
		} {
			var h *htmlLabel
			if *l.w, *l.h, h, err = other(r, l.name, l.size); err != nil {
				return nil, fmt.Errorf("the %slabel of HTML of the edge %s -> %s: %w", l.name, e.Source, e.Target, err)
			}
			if h != nil {
				hs[l.name] = h
			}
		}
		if len(hs) > 0 {
			edgeHTMLs[i] = hs
		}
		if d := expr.Number(r["labelDistance"]); d > 0 {
			e.LabelDistance, e.LabelAngle = d, expr.Number(r["labelAngle"])
		}
		in.Edges = append(in.Edges, e)
	}
	clusterHTMLs := map[int]*htmlLabel{}
	for i, r := range clusters {
		c := graph.LayoutCluster{ID: text(r, fields["key"]), Parent: expr.String(r["parent"])}
		if r["parent"] == nil {
			c.Parent = ""
		}
		if d, ok := r["direction"].(string); ok {
			c.Direction = d
		}
		tw, th := measureAt(fields["label"].Get(r), sizeOf(r, "fontSize"), "600")
		if src, ok := r["html"].(string); ok && src != "" {
			h, err := layHTML(src, sizeAt, sizeOf(r, "fontSize"))
			if err != nil {
				return nil, fmt.Errorf("the label of HTML of cluster %s: %w", c.ID, err)
			}
			clusterHTMLs[i] = h
			tw, th = h.lay.Width, h.lay.Height
		}
		if tw > 0 {
			c.TitleWidth, c.TitleHeight, c.TitleTop = tw, th+4+titleTop+titleBottom, titleTop
		}
		in.Clusters = append(in.Clusters, c)
	}
	l, err := graph.Lay(in)
	if err != nil {
		return nil, fmt.Errorf("laying out the graph: %w: %w", ErrParam, err)
	}
	out := make([]Row, 0, len(rows)+len(edges)+len(clusters))
	for i, r := range rows {
		o := clone(r)
		np := l.Nodes[i]
		o["kind"] = "node"
		if ls, ok := wrapped[i]; ok {
			o["lines"] = ls
		}
		o[as[0]], o[as[1]], o[as[2]], o[as[3]] = np.X, np.Y, np.Width, np.Height
		o[as[4]], o[as[5]] = float64(np.Rank), float64(np.Order)
		o["path"] = graph.ShapePath(in.Nodes[i].Shape, np.Width, np.Height)
		o["textX"], o["textY"] = np.X, np.Y+graph.TextOffset(in.Nodes[i].Shape)
		o["xLabelX"], o["xLabelY"] = np.XLabelX, np.XLabelY
		o["xLabelWidth"], o["xLabelHeight"] = in.Nodes[i].XLabelWidth, in.Nodes[i].XLabelHeight
		if m, ok := medias[i]; ok {
			m.place(o, np)
		}
		out = append(out, o)
		if h, ok := htmls[i]; ok {
			out = append(out, h.rows(r, np.X, np.Y, in.Nodes[i].Shape)...)
		}
		if h, ok := xhtmls[i]; ok {
			out = append(out, h.rows(linkRow(Row{"key": r["key"], "textColor": r["textColor"]}, r), np.XLabelX, np.XLabelY, "")...)
		}
		if rec, ok := records[i]; ok {
			var fs []Row
			o["path"], fs = rec.rows(r, np, in.Nodes[i].Shape, padding, fontSize)
			out = append(out, fs...)
		}
	}
	for i, r := range edges {
		o := clone(r)
		ep := l.Edges[i]
		o["kind"] = "edge"
		if ls, ok := edgeWrapped[i]; ok {
			o["lines"] = ls
		}
		o["path"] = ep.Path
		o["labelX"], o["labelY"] = ep.LabelX, ep.LabelY
		o["labelWidth"], o["labelHeight"] = in.Edges[i].LabelWidth, in.Edges[i].LabelHeight
		o["endX"], o["endY"], o["endAngle"] = ep.EndX, ep.EndY, ep.EndAngle
		o["startX"], o["startY"], o["startAngle"] = ep.StartX, ep.StartY, ep.StartAngle
		le := in.Edges[i]
		o["headLabelX"], o["headLabelY"], o["headLabelWidth"], o["headLabelHeight"] = ep.HeadLabelX, ep.HeadLabelY, le.HeadLabelWidth, le.HeadLabelHeight
		o["tailLabelX"], o["tailLabelY"], o["tailLabelWidth"], o["tailLabelHeight"] = ep.TailLabelX, ep.TailLabelY, le.TailLabelWidth, le.TailLabelHeight
		o["xLabelX"], o["xLabelY"], o["xLabelWidth"], o["xLabelHeight"] = ep.XLabelX, ep.XLabelY, le.XLabelWidth, le.XLabelHeight
		pts := make([]any, len(ep.Points))
		for j, pt := range ep.Points {
			pts[j] = []any{pt[0], pt[1]}
		}
		o["points"] = pts
		out = append(out, o)
		hs := edgeHTMLs[i]
		// The labels at the ends take the color of the ends, and the
		// others the color of the label of the edge.
		end := r["endColor"]
		if end == nil {
			end = r["textColor"]
		}
		for _, l := range []struct {
			name  string
			x, y  float64
			color any
		}{
			{"label", ep.LabelX, ep.LabelY, r["textColor"]}, {"head", ep.HeadLabelX, ep.HeadLabelY, end},
			{"tail", ep.TailLabelX, ep.TailLabelY, end}, {"x", ep.XLabelX, ep.XLabelY, r["textColor"]},
		} {
			if h, ok := hs[l.name]; ok {
				out = append(out, h.rows(linkRow(Row{"key": r["key"], "stroke": r["stroke"], "textColor": l.color}, r), l.x, l.y, "")...)
			}
		}
	}
	for i, r := range clusters {
		o := clone(r)
		cp := l.Clusters[i]
		o["kind"] = "cluster"
		o["x"], o["y"], o["width"], o["height"] = cp.X, cp.Y, cp.Width, cp.Height
		o["depth"], o["titleX"], o["titleY"] = float64(cp.Depth), cp.TitleX, cp.TitleY
		out = append(out, o)
		if h, ok := clusterHTMLs[i]; ok && cp.Width > 0 {
			out = append(out, h.rows(linkRow(Row{"key": r["key"], "stroke": r["stroke"], "textColor": r["textColor"]}, r), cp.TitleX+h.lay.Width/2, cp.TitleY+h.lay.Height/2, "")...)
		}
	}
	// A title of HTML sits above the layout, at its left.
	if src, err := p.str("titleHTML", ""); err != nil {
		return nil, err
	} else if src != "" {
		size, err := p.number("titleFontSize", fontSize)
		if err != nil {
			return nil, err
		}
		h, err := layHTML(src, sizeAt, size)
		if err != nil {
			return nil, fmt.Errorf("the title of HTML: %w", err)
		}
		out = append(out, h.rows(Row{"key": "title"}, h.lay.Width/2, -h.lay.Height/2-8, "")...)
	}
	return out, nil
}

// media is the icon or the image of a node, which the transform zuzu.icon
// sized: the size of the icon, the size of its form, and the size of the
// label.
type media struct {
	w, h, fw, fh, tw, th float64
	form, pos            string
	inline               bool
}

// mediaOf returns the media of a row of a node, or false for none.
func mediaOf(r Row, padding float64) (*media, bool) {
	w, h := expr.Number(r["mediaWidth"]), expr.Number(r["mediaHeight"])
	if !(w > 0) || !(h > 0) {
		return nil, false
	}
	m := &media{w: w, h: h, fw: w, fh: h, form: strOf(r["iconForm"]), pos: strOf(r["iconPos"])}
	m.inline, _ = r["iconInline"].(bool)
	switch m.form {
	case "square", "rounded":
		m.fw, m.fh = w+2*padding, h+2*padding
	case "circle":
		d := math.Hypot(w, h) + padding
		m.fw, m.fh = d, d
	default:
		m.form = ""
	}
	return m, true
}

// place writes the places of the media and the label of a node: mediaX
// and mediaY, the center of the icon, mediaLeft and mediaTop, its top
// left, textX and textY, the center of the label, and formPath, the path of
// the form around the icon.
func (m *media) place(o Row, np graph.NodePlace) {
	x, y := np.X, np.Y
	if m.inline {
		left := x - (m.w+4+m.tw)/2
		o["mediaX"], o["mediaY"] = left+m.w/2, y
		o["textX"] = left + m.w + 4 + m.tw/2
	} else {
		o["path"] = ""
		top := y - np.Height/2
		my, ty := top+m.fh/2, top+m.fh+4+m.th/2
		if m.pos == "t" && m.tw > 0 {
			ty, my = top+m.th/2, top+m.th+4+m.fh/2
		}
		o["mediaX"], o["mediaY"], o["textY"] = x, my, ty
		if m.form != "" {
			kind := map[string]string{"square": "rect", "rounded": "rounded", "circle": "circle"}[m.form]
			o["formPath"] = graph.ShapePath(kind, m.fw, m.fh)
		}
	}
	o["mediaLeft"] = expr.Number(o["mediaX"]) - m.w/2
	o["mediaTop"] = expr.Number(o["mediaY"]) - m.h/2
}

// wrapText breaks each line of a text at its spaces into lines that are no
// wider than limit, as Mermaid wraps a label. width measures a line. A line
// that fits stays as it is. A word that is wider than limit takes a line of
// its own, or, with breakWords, breaks between two letters into pieces
// that fit, with at least one letter in each.
func wrapText(s string, limit float64, breakWords bool, width func(string) float64) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if width(line) <= limit {
			out = append(out, line)
			continue
		}
		cur := ""
		for w := range strings.FieldsSeq(line) {
			next := w
			if cur != "" {
				next = cur + " " + w
			}
			if width(next) <= limit {
				cur = next
				continue
			}
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			if width(w) <= limit {
				cur = w
				continue
			}
			if !breakWords {
				out = append(out, w)
				continue
			}
			rs := []rune(w)
			for len(rs) > 0 {
				k := 1
				for k < len(rs) && width(string(rs[:k+1])) <= limit {
					k++
				}
				if k == len(rs) {
					cur = string(rs)
					break
				}
				out = append(out, string(rs[:k]))
				rs = rs[k:]
			}
		}
		if cur != "" {
			out = append(out, cur)
		}
	}
	return out
}

// linkRow copies the link and the tooltip of src into dst, and returns dst.
func linkRow(dst, src Row) Row {
	copyLink(dst, src)
	return dst
}
