package mermaid

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/text"
)

// seqConfig holds the settings of a sequence diagram, in the units of the
// look of zuzu. A setting that the source gives replaces its default.
type seqConfig struct {
	mirror, numbers, hideUnused, rightAngles, wrap bool

	actorMargin, messageMargin, noteMargin, boxMargin, boxTextMargin float64
	activationWidth, minWidth, minHeight, wrapPadding                float64
	labelBoxWidth, labelBoxHeight, marginX, marginY                  float64

	// wrapWidth is the width at which a message, a note and the label of
	// a box wrap, and actorWrap the width at which the label of a
	// participant wraps, both in the pixels of zuzu.
	wrapWidth, actorWrap float64

	messageAlign, noteAlign string

	actorFont, messageFont, noteFont text.Font
}

// seqRead are the settings of the config that a sequence diagram reads.
var seqRead = []string{
	"wrap", "sequence.wrap", "sequence.mirrorActors", "sequence.showSequenceNumbers",
	"sequence.hideUnusedParticipants", "sequence.rightAngles", "sequence.actorMargin",
	"sequence.messageMargin", "sequence.noteMargin", "sequence.boxMargin", "sequence.boxTextMargin",
	"sequence.activationWidth", "sequence.width", "sequence.height", "sequence.wrapPadding",
	"sequence.labelBoxWidth", "sequence.labelBoxHeight", "sequence.diagramMarginX",
	"sequence.diagramMarginY", "sequence.messageAlign", "sequence.noteAlign",
	"sequence.actorFontSize", "sequence.actorFontWeight", "sequence.messageFontSize",
	"sequence.messageFontWeight", "sequence.noteFontSize", "sequence.noteFontWeight",
}

// sequenceSettings reads the config of a sequence diagram. Each setting
// that it does not read is an issue.
func (c *chart) sequenceSettings() (seqConfig, error) {
	if err := c.unreadSettings(seqRead...); err != nil {
		return seqConfig{}, err
	}
	cfg := c.d.Config
	seq, _ := cfg["sequence"].(map[string]any)
	num := func(key string, def float64) float64 {
		if v, ok := configNumber(cfg, "sequence", key); ok && v >= 0 {
			return v
		}
		return def
	}
	flag := func(key string, def bool) bool {
		if v, ok := seq[key].(bool); ok {
			return v
		}
		return def
	}
	font := func(prefix string, size float64, weight int) text.Font {
		f := text.Font{Family: text.Family, Size: num(prefix+"FontSize", size), Weight: weight}
		switch v := seq[prefix+"FontWeight"].(type) {
		case float64:
			f.Weight = int(v)
		case string:
			f.Weight = text.Weight(v)
		}
		return f
	}
	s := seqConfig{
		mirror:          flag("mirrorActors", true),
		numbers:         flag("showSequenceNumbers", false),
		hideUnused:      flag("hideUnusedParticipants", false),
		rightAngles:     flag("rightAngles", false),
		actorMargin:     num("actorMargin", 32),
		messageMargin:   num("messageMargin", 36),
		noteMargin:      num("noteMargin", 10),
		boxMargin:       num("boxMargin", 10),
		boxTextMargin:   num("boxTextMargin", 5),
		activationWidth: math.Max(num("activationWidth", 10), 2),
		minWidth:        num("width", 56),
		minHeight:       num("height", 34),
		wrapPadding:     num("wrapPadding", 10),
		labelBoxWidth:   num("labelBoxWidth", 0),
		labelBoxHeight:  num("labelBoxHeight", 20),
		marginX:         num("diagramMarginX", 8),
		marginY:         num("diagramMarginY", 8),
		messageAlign:    "center",
		noteAlign:       "center",
		actorFont:       font("actor", 12, 600),
		messageFont:     font("message", 11, 400),
		noteFont:        font("note", 11, 400),
	}
	width := num("width", 150)
	s.wrapWidth = scaleWrap(width)
	s.actorWrap = scaleWrap(width - 2*s.wrapPadding)
	if w, ok := cfg["wrap"].(bool); ok {
		s.wrap = w
	}
	s.wrap = flag("wrap", s.wrap)
	for _, a := range []struct {
		key string
		to  *string
	}{{"messageAlign", &s.messageAlign}, {"noteAlign", &s.noteAlign}} {
		v, ok := seq[a.key].(string)
		if !ok {
			continue
		}
		switch v {
		case "left", "center", "right":
			*a.to = v
		default:
			if err := c.add("the setting sequence.%s %s", a.key, v); err != nil {
				return s, err
			}
		}
	}
	return s, nil
}

// seqLayout lays out a sequence diagram and writes its rows.
type seqLayout struct {
	s   *sequence
	cfg seqConfig
	m   *text.Estimate

	ps  []*seqParticipant
	idx map[string]int
	// cx is the x of the lifeline of each participant, and w the width of
	// its head.
	cx, w []float64
	// hh is the height of the head of every participant.
	hh float64

	rows map[string][]any
	// minX, maxX and maxY are the bounds of the rows.
	minX, maxX, maxY float64

	// acts are the open activations of each participant, by their start.
	acts map[string][]float64
	// frames are the open blocks.
	frames []*seqFrame
	// numbers reports whether the messages show their number, and number
	// and step are the next number and its step.
	numbers      bool
	number, step float64
}

// seqFrame is an open block, with the extent of what it holds.
type seqFrame struct {
	block, fill string
	text        seqText
	y0, x0, x1  float64
	sections    []seqPart
}

// seqPart is a section of a block: its y and its text.
type seqPart struct {
	y    float64
	text seqText
}

// newSeqLayout lays out a sequence diagram.
func newSeqLayout(s *sequence, cfg seqConfig) *seqLayout {
	l := &seqLayout{
		s: s, cfg: cfg, m: text.NewEstimate(), idx: map[string]int{}, rows: map[string][]any{},
		minX: math.Inf(1), maxX: math.Inf(-1), acts: map[string][]float64{},
	}
	for _, p := range s.participants {
		if cfg.hideUnused && !p.used {
			continue
		}
		l.idx[p.id] = len(l.ps)
		l.ps = append(l.ps, p)
	}
	for _, p := range l.ps {
		w, h := l.headSize(p)
		l.w = append(l.w, w)
		l.hh = math.Max(l.hh, h)
	}
	l.place()
	l.draw()
	return l
}

// lh returns the height of a line of text in a font.
func lh(f text.Font) float64 {
	return math.Round(f.Size*13) / 10
}

// lines returns the lines of a text, wrapped to a width when the text or
// the diagram asks for it.
func (l *seqLayout) lines(t seqText, f text.Font, width float64) []string {
	wrap := l.cfg.wrap
	if t.wrap != nil {
		wrap = *t.wrap
	}
	if !wrap {
		return t.lines
	}
	var out []string
	for _, line := range t.lines {
		cur := ""
		for word := range strings.FieldsSeq(line) {
			next := word
			if cur != "" {
				next = cur + " " + word
			}
			if cur != "" && l.m.Width(f, next) > width {
				out = append(out, cur)
				next = word
			}
			cur = next
		}
		out = append(out, cur)
	}
	return out
}

// size returns the width and the height of lines of text.
func (l *seqLayout) size(f text.Font, lines []string) (float64, float64) {
	w := 0.0
	for _, s := range lines {
		w = math.Max(w, l.m.Width(f, s))
	}
	return math.Ceil(w), float64(len(lines)) * lh(f)
}

// icon reports whether a kind of participant draws as an icon over its
// label, rather than as a box around it.
func icon(kind string) bool {
	switch kind {
	case "actor", "boundary", "control", "entity":
		return true
	}
	return false
}

// iconHeight is the height of the icon of a participant.
const iconHeight = 28

// headSize returns the width and the height of the head of a participant.
func (l *seqLayout) headSize(p *seqParticipant) (float64, float64) {
	lines := l.lines(p.label, l.cfg.actorFont, l.cfg.actorWrap)
	tw, th := l.size(l.cfg.actorFont, lines)
	if icon(p.kind) {
		return math.Max(tw+8, 40), iconHeight + 4 + th
	}
	w := math.Max(tw+28, l.cfg.minWidth)
	h := math.Max(th+16, l.cfg.minHeight)
	switch p.kind {
	case "queue":
		w += 16
	case "database":
		h += 12
	case "collections":
		w += 6
		h += 6
	}
	return w, h
}

// place works out the x of each lifeline: each two lifelines are far
// enough apart for the heads, the messages and the notes between them.
func (l *seqLayout) place() {
	type need struct {
		i, j int
		d    float64
	}
	n := len(l.ps)
	var needs []need
	for i := range n - 1 {
		d := l.w[i]/2 + l.w[i+1]/2 + l.cfg.actorMargin
		if l.ps[i].box != l.ps[i+1].box {
			d += 2 * l.cfg.boxMargin
		}
		needs = append(needs, need{i, i + 1, d})
	}
	// A box is as wide as its title. A box of one participant grows to the
	// right, so the next participant moves.
	for b, box := range l.s.boxes {
		first, last := -1, -1
		for i, p := range l.ps {
			if p.box == b {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		tw, _ := l.size(l.boxFont(), box.title.lines)
		if first < 0 {
			continue
		}
		extra := tw + 20 - (l.w[first]/2 + l.w[last]/2 + 2*l.cfg.boxMargin)
		switch {
		case extra <= 0:
		case first < last:
			needs = append(needs, need{first, last, tw + 20 - l.w[first]/2 - l.w[last]/2 - 2*l.cfg.boxMargin})
		case last+1 < n:
			needs = append(needs, need{last, last + 1, tw + 20 + l.cfg.actorMargin + (l.w[last+1]-l.w[last])/2})
		}
	}
	aw := l.cfg.activationWidth
	numbers := l.cfg.numbers
	for k, e := range l.s.events {
		switch e.kind {
		case "autonumber":
			numbers = e.visible || l.cfg.numbers
		case "message":
			i, ok1 := l.idx[e.from]
			j, ok2 := l.idx[e.to]
			if !ok1 || !ok2 {
				continue
			}
			tw, _ := l.size(l.cfg.messageFont, l.lines(e.text, l.cfg.messageFont, l.cfg.wrapWidth))
			if i == j {
				if i+1 < n {
					needs = append(needs, need{i, i + 1, math.Max(tw+16, 44) + 2*aw})
				}
				continue
			}
			d := tw + 32 + 2*aw
			if numbers {
				d += 12
			}
			if l.ps[j].created == k {
				d += l.w[j] / 2
			}
			if l.ps[i].destroyed == k {
				d += l.w[i] / 2
			}
			if l.ps[j].destroyed == k {
				d += l.w[j] / 2
			}
			needs = append(needs, need{min(i, j), max(i, j), d})
		case "note":
			i, ok1 := l.idx[e.from]
			j, ok2 := l.idx[e.to]
			if !ok1 || !ok2 {
				continue
			}
			nw, _ := l.noteSize(e)
			switch {
			case e.place == "right" && i+1 < n:
				needs = append(needs, need{i, i + 1, nw + 24 + aw})
			case e.place == "left" && i > 0:
				needs = append(needs, need{i - 1, i, nw + 24 + aw})
			case e.place == "over" && i == j:
				if i > 0 {
					needs = append(needs, need{i - 1, i, nw/2 + 12})
				}
				if i+1 < n {
					needs = append(needs, need{i, i + 1, nw/2 + 12})
				}
			case e.place == "over":
				needs = append(needs, need{min(i, j), max(i, j), nw - 48})
			}
		}
	}
	slices.SortStableFunc(needs, func(a, b need) int {
		if a.j != b.j {
			return a.j - b.j
		}
		return b.i - a.i
	})
	l.cx = make([]float64, n)
	for i := range l.cx {
		if i > 0 {
			l.cx[i] = l.cx[i-1]
		}
	}
	for _, nd := range needs {
		if gap := l.cx[nd.j] - l.cx[nd.i]; gap < nd.d {
			for k := nd.j; k < n; k++ {
				l.cx[k] += nd.d - gap
			}
		}
	}
}

// noteSize returns the size of a note.
func (l *seqLayout) noteSize(e seqEvent) (float64, float64) {
	tw, th := l.size(l.cfg.noteFont, l.lines(e.text, l.cfg.noteFont, l.cfg.wrapWidth))
	return math.Max(tw+2*l.cfg.noteMargin, 40), th + 1.4*l.cfg.noteMargin
}

// add appends a row to a set, and grows the bounds by a box.
func (l *seqLayout) add(set string, row map[string]any, x0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.extent(x0, x1)
	l.maxY = math.Max(l.maxY, y1)
}

// extent grows the bounds, and the blocks that are open, by a span of x.
func (l *seqLayout) extent(x0, x1 float64) {
	l.minX, l.maxX = math.Min(l.minX, x0), math.Max(l.maxX, x1)
	for _, f := range l.frames {
		f.x0, f.x1 = math.Min(f.x0, x0), math.Max(f.x1, x1)
	}
}

// lineRows returns the lines of a text as the values of a row.
func lineRows(lines []string) []any {
	out := make([]any, len(lines))
	for i, s := range lines {
		out[i] = s
	}
	return out
}

// textRow adds a row of text to a set: its lines at a place, with an
// align and a baseline.
func (l *seqLayout) textRow(set, kind string, lines []string, f text.Font, x, y float64, align, baseline string) {
	if len(lines) == 0 {
		return
	}
	w, h := l.size(f, lines)
	x0 := x - w/2
	switch align {
	case "left":
		x0 = x
	case "right":
		x0 = x - w
	}
	row := map[string]any{"kind": kind, "x": x, "y": y, "lines": lineRows(lines), "align": align, "baseline": baseline, "text": strings.Join(lines, " ")}
	l.add(set, row, x0, x0+w, y+h)
}

// head draws the head of a participant: its shape and its label, in a band
// from top of the height of the heads. It returns the y where the
// lifeline starts below it.
func (l *seqLayout) head(i int, top float64, part string) float64 {
	p := l.ps[i]
	x, w, hh := l.cx[i], l.w[i], l.hh
	f := l.cfg.actorFont
	lines := l.lines(p.label, f, l.cfg.actorWrap)
	_, th := l.size(f, lines)
	if icon(p.kind) {
		h := iconHeight + 4 + th
		y0 := top + (hh-h)/2
		cy := y0 + iconHeight/2
		l.add("icons", map[string]any{"kind": part, "key": p.id, "x": x, "y": cy, "path": iconPath(p.kind)}, x-18, x+18, cy+iconHeight/2)
		l.textRow("actorLabels", part, lines, f, x, y0+iconHeight+4, "center", "top")
		return y0 + h + 2
	}
	cy := top + hh/2
	tx := x
	path := graph.ShapePath("rounded", w, hh)
	switch p.kind {
	case "database":
		path = graph.ShapePath("cylinder", w, hh)
		cy2 := cy + math.Min(hh*0.12, 10)/2
		l.textRow("actorLabels", part, lines, f, tx, cy2, "center", "middle")
		l.add("shapes", map[string]any{"kind": part, "key": p.id, "x": x, "y": cy, "path": path}, x-w/2, x+w/2, top+hh)
		return top + hh
	case "queue":
		path = queuePath(w, hh)
		tx -= 4
	case "collections":
		back := shape.RoundRect(-w/2+6, -hh/2, w-6, hh-6, 4, 4, 4, 4)
		front := shape.RoundRect(-w/2, -hh/2+6, w-6, hh-6, 4, 4, 4, 4)
		l.add("shapes", map[string]any{"kind": part, "key": p.id, "x": x, "y": cy, "path": back}, x-w/2, x+w/2, top+hh)
		l.add("shapeBacks", map[string]any{"kind": part, "key": p.id, "x": x, "y": cy, "path": front}, x-w/2, x+w/2, top+hh)
		l.textRow("actorLabels", part, lines, f, tx-3, cy+3, "center", "middle")
		return top + hh
	}
	l.add("shapes", map[string]any{"kind": part, "key": p.id, "x": x, "y": top + hh/2, "path": path}, x-w/2, x+w/2, top+hh)
	l.textRow("actorLabels", part, lines, f, tx, cy, "center", "middle")
	return top + hh
}

// queuePath returns the path of a queue: a cylinder on its side.
func queuePath(w, h float64) string {
	x, y := w/2, h/2
	rx := math.Min(w*0.08, 8)
	a := func(sweep string, px, py float64) string {
		return "A" + pathNum(rx) + "," + pathNum(y) + " 0 0," + sweep + " " + pathPoint(px, py)
	}
	return "M" + pathPoint(-x+rx, -y) + "H" + pathNum(x-rx) + a("1", x-rx, y) + "H" + pathNum(-x+rx) + a("1", -x+rx, -y) + "Z" +
		"M" + pathPoint(x-rx, -y) + a("0", x-rx, y)
}

// iconPath returns the path of the icon of a participant, around its
// center.
func iconPath(kind string) string {
	circle := func(cx, cy, r float64) string {
		return "M" + pathPoint(cx+r, cy) + "A" + pathNum(r) + "," + pathNum(r) + " 0 1,1 " + pathPoint(cx-r, cy) +
			"A" + pathNum(r) + "," + pathNum(r) + " 0 1,1 " + pathPoint(cx+r, cy) + "Z"
	}
	switch kind {
	case "boundary":
		return circle(4, 0, 10) + "M-6,0H-14M-14,-10V10"
	case "control":
		return circle(0, 2, 10) + "M4,-12L-1,-8L4,-4"
	case "entity":
		return circle(0, -2, 10) + "M-10,12H10"
	}
	return circle(0, -8, 5) + "M0,-3V5M-9,0H9M0,5L-7,14M0,5L7,14"
}

// band returns the x of the side of the head of a participant that faces
// a place.
func (l *seqLayout) band(i int, toward float64) float64 {
	half := l.w[i] / 2
	if icon(l.ps[i].kind) {
		half = 14
	}
	if toward < l.cx[i] {
		return l.cx[i] - half
	}
	return l.cx[i] + half
}

// attach returns the x where a message meets the lifeline of a
// participant, at the side of its top activation that faces a place.
func (l *seqLayout) attach(id string, toward float64) float64 {
	i := l.idx[id]
	n := len(l.acts[id])
	if n == 0 {
		return l.cx[i]
	}
	aw := l.cfg.activationWidth
	left := l.cx[i] - aw/2 + float64(n-1)*aw/2
	if toward < l.cx[i] {
		return left
	}
	return left + aw
}

// activate opens an activation of a participant at a y.
func (l *seqLayout) activate(id string, y float64) {
	l.acts[id] = append(l.acts[id], y)
}

// deactivate closes the top activation of a participant at a y, and draws
// it.
func (l *seqLayout) deactivate(id string, y float64) {
	stack := l.acts[id]
	if len(stack) == 0 {
		return
	}
	y0 := stack[len(stack)-1]
	l.acts[id] = stack[:len(stack)-1]
	aw := l.cfg.activationWidth
	x := l.cx[l.idx[id]] - aw/2 + float64(len(stack)-1)*aw/2
	l.add("activations", map[string]any{"kind": "activation", "key": id, "x": x, "x2": x + aw, "y": y0, "y2": math.Max(y, y0+8)}, x, x+aw, y)
}

// gap is the space above a message, a note or a block, so that two
// messages of one line are messageMargin apart.
func (l *seqLayout) gap() float64 {
	return math.Max(l.cfg.messageMargin-lh(l.cfg.messageFont)-4, 4)
}

// draw lays out the events down the page and writes the rows.
func (l *seqLayout) draw() {
	top := 0.0
	if len(l.s.boxes) > 0 {
		top = l.cfg.boxMargin
		for _, b := range l.s.boxes {
			if len(b.title.lines) > 0 {
				_, th := l.size(l.boxFont(), b.title.lines)
				top = math.Max(top, l.cfg.boxMargin+th+8)
			}
		}
	}
	start := make([]float64, len(l.ps))
	end := make([]float64, len(l.ps))
	for i, p := range l.ps {
		if p.created < 0 {
			start[i] = l.head(i, top, "head")
		}
		end[i] = -1
	}
	y := top + l.hh
	l.numbers, l.number, l.step = l.cfg.numbers, 1, 1
	for k, e := range l.s.events {
		switch e.kind {
		case "autonumber":
			if e.start > 0 {
				l.number = e.start
			}
			if e.step > 0 {
				l.step = e.step
			}
			l.numbers = e.visible || l.cfg.numbers
		case "message":
			y = l.message(k, e, y, start, end)
		case "note":
			y = l.note(e, y)
		case "activate":
			if _, ok := l.idx[e.from]; ok {
				l.activate(e.from, y)
			}
		case "deactivate":
			if _, ok := l.idx[e.from]; ok {
				l.deactivate(e.from, y)
			}
		case "start":
			y += l.gap() * 0.6
			f := &seqFrame{block: e.block, fill: e.fill, text: e.text, y0: y, x0: math.Inf(1), x1: math.Inf(-1)}
			l.frames = append(l.frames, f)
			if e.block == "rect" {
				y += 2
			} else {
				_, th := l.size(l.cfg.messageFont, l.lines(e.text, l.cfg.messageFont, l.cfg.wrapWidth))
				y += math.Max(l.cfg.labelBoxHeight, th+2*l.cfg.boxTextMargin)
			}
		case "section":
			y += l.gap() * 0.6
			f := l.frames[len(l.frames)-1]
			f.sections = append(f.sections, seqPart{y: y, text: e.text})
			_, th := l.size(l.cfg.messageFont, l.lines(e.text, l.cfg.messageFont, l.cfg.wrapWidth))
			y += math.Max(th+2*l.cfg.boxTextMargin, 6)
		case "end":
			y += l.gap() * 0.6
			l.closeFrame(y)
		}
		l.maxY = math.Max(l.maxY, y)
	}
	y += l.gap() + 8
	for _, p := range l.ps {
		for len(l.acts[p.id]) > 0 {
			l.deactivate(p.id, y)
		}
	}
	bottom := y
	for i, p := range l.ps {
		if p.destroyed >= 0 {
			continue
		}
		end[i] = y
		if l.cfg.mirror {
			l.head(i, y, "foot")
			bottom = y + l.hh
		}
	}
	for i, p := range l.ps {
		l.add("lifelines", map[string]any{"kind": "lifeline", "key": p.id, "x": l.cx[i], "y": start[i], "y2": end[i]}, l.cx[i], l.cx[i], end[i])
	}
	l.boxes(top, bottom)
}

// boxFont is the font of the title of a box.
func (l *seqLayout) boxFont() text.Font {
	return text.Font{Family: text.Family, Size: 11, Weight: 600}
}

// boxes draws the boxes of participants, from the top of the diagram to its
// bottom.
func (l *seqLayout) boxes(top, bottom float64) {
	f := l.boxFont()
	for b, box := range l.s.boxes {
		x0, x1 := math.Inf(1), math.Inf(-1)
		for i, p := range l.ps {
			if p.box == b {
				x0 = math.Min(x0, l.cx[i]-l.w[i]/2-l.cfg.boxMargin)
				x1 = math.Max(x1, l.cx[i]+l.w[i]/2+l.cfg.boxMargin)
			}
		}
		if math.IsInf(x0, 1) {
			continue
		}
		tw, _ := l.size(f, box.title.lines)
		x1 = math.Max(x1, x0+tw+20)
		row := map[string]any{"kind": "box", "x": x0, "x2": x1, "y": 0, "y2": bottom + l.cfg.boxMargin}
		set := "boxes"
		if box.fill != "" {
			row["fill"], row["opacity"] = sourceFill(box.fill)
			set = "boxesFilled"
		}
		l.add(set, row, x0, x1, bottom+l.cfg.boxMargin)
		l.textRow("boxLabels", "box", box.title.lines, f, x0+10, top-6-float64(len(box.title.lines)-1)*lh(f), "left", "bottom")
	}
}

// sourceFill returns a color that the source gives to a box or a rect, as
// an opaque color and an opacity: its own opacity, or 0.25 for an opaque
// color, so that the text on it reads in both modes.
func sourceFill(s string) (string, float64) {
	c, _ := color.Parse(s)
	a := c.A
	if a >= 1 {
		a = 0.25
	}
	c.A = 1
	return c.Hex(), a
}

// message draws a message at a y, and returns the y below it.
func (l *seqLayout) message(k int, e seqEvent, y float64, start, end []float64) float64 {
	i, ok1 := l.idx[e.from]
	j, ok2 := l.idx[e.to]
	defer func() { l.number = math.Round((l.number+l.step)*100) / 100 }()
	if !ok1 || !ok2 {
		return y
	}
	f := l.cfg.messageFont
	lines := l.lines(e.text, f, l.cfg.wrapWidth)
	_, th := l.size(f, lines)
	lineY := y + l.gap() + th + 4
	if len(lines) == 0 {
		lineY = y + l.gap() + 4
	}
	created := l.ps[j].created == k
	destroyFrom, destroyTo := l.ps[i].destroyed == k, l.ps[j].destroyed == k
	if created || destroyFrom || destroyTo {
		lineY = math.Max(lineY, y+8+l.hh/2)
	}
	if i == j {
		return l.self(e, lines, lineY)
	}
	if e.activate {
		l.activate(e.to, lineY)
	}
	fx, tx := l.attach(e.from, l.cx[j]), l.attach(e.to, l.cx[i])
	switch {
	case e.centralFrom:
		fx = l.cx[i]
	case destroyFrom:
		fx = l.band(i, l.cx[j])
	}
	switch {
	case e.centralTo:
		tx = l.cx[j]
	case created || destroyTo:
		tx = l.band(j, l.cx[i])
	}
	dir := 1.0
	if tx < fx {
		dir = -1
	}
	// The ends of the line: a dot of a central connection takes 3.5, and a
	// filled head covers the end of the line.
	x0, x1 := fx, tx
	if e.centralFrom {
		l.add("dots", map[string]any{"kind": "dot", "x": fx, "y": lineY, "path": dotPath()}, fx-4, fx+4, lineY+4)
		x0 += 3.5 * dir
	}
	if e.centralTo {
		l.add("dots", map[string]any{"kind": "dot", "x": tx, "y": lineY, "path": dotPath()}, tx-4, tx+4, lineY+4)
		x1 -= 3.5 * dir
	}
	l.end(e.arrow.head, x1, lineY, dir)
	l.end(e.arrow.tail, x0, lineY, -dir)
	set := "messages"
	if e.arrow.dotted {
		set = "messagesDotted"
	}
	l.add(set, map[string]any{
		"kind": "message", "from": e.from, "to": e.to, "text": strings.Join(lines, " "),
		"x": x0, "y": lineY, "path": "M0,0H" + pathNum(x1-x0),
	}, math.Min(x0, x1), math.Max(x0, x1), lineY)
	if l.numbers {
		l.badge(fx, lineY)
	}
	lx, align := (fx+tx)/2, "center"
	switch l.cfg.messageAlign {
	case "left":
		lx, align = math.Min(fx, tx)+12, "left"
	case "right":
		lx, align = math.Max(fx, tx)-12, "right"
	}
	l.textRow("messageLabels", "message", lines, f, lx, lineY-5, align, "bottom")
	if e.deactivate {
		l.deactivate(e.from, lineY)
	}
	next := lineY
	if created {
		start[j] = l.head(j, lineY-l.hh/2, "head")
		next = lineY + l.hh/2
	}
	for _, d := range []struct {
		yes bool
		at  int
	}{{destroyFrom, i}, {destroyTo, j}} {
		if !d.yes {
			continue
		}
		for len(l.acts[l.ps[d.at].id]) > 0 {
			l.deactivate(l.ps[d.at].id, lineY)
		}
		end[d.at] = lineY - l.hh/2
		l.head(d.at, lineY-l.hh/2, "foot")
		next = lineY + l.hh/2
	}
	return next
}

// self draws a message from a participant to itself: a loop at the right
// of its lifeline. It returns the y below it.
func (l *seqLayout) self(e seqEvent, lines []string, lineY float64) float64 {
	cx := l.cx[l.idx[e.from]]
	sx := l.attach(e.from, cx+1)
	if e.activate {
		l.activate(e.to, lineY+18)
	}
	endY := lineY + 18
	ex := l.attach(e.to, cx+1)
	if e.centralFrom {
		sx = cx
	}
	if e.centralTo {
		ex = cx
	}
	reach := math.Max(sx, ex) + 30
	path := "M0,0C" + pathPoint(reach-sx+6, 0) + " " + pathPoint(reach-sx+6, endY-lineY) + " " + pathPoint(ex-sx, endY-lineY)
	if l.cfg.rightAngles {
		path = "M0,0H" + pathNum(reach-sx) + "V" + pathNum(endY-lineY) + "H" + pathNum(ex-sx)
	}
	set := "messages"
	if e.arrow.dotted {
		set = "messagesDotted"
	}
	l.add(set, map[string]any{
		"kind": "message", "from": e.from, "to": e.to, "text": strings.Join(lines, " "),
		"x": sx, "y": lineY, "path": path,
	}, sx, reach+6, endY)
	l.end(e.arrow.head, ex, endY, -1)
	l.end(e.arrow.tail, sx, lineY, -1)
	if e.centralFrom {
		l.add("dots", map[string]any{"kind": "dot", "x": sx, "y": lineY, "path": dotPath()}, sx-4, sx+4, lineY+4)
	}
	if e.centralTo {
		l.add("dots", map[string]any{"kind": "dot", "x": ex, "y": endY, "path": dotPath()}, ex-4, ex+4, endY+4)
	}
	if l.numbers {
		l.badge(sx, lineY)
	}
	lx := sx + 4
	if l.numbers {
		lx += 10
	}
	l.textRow("messageLabels", "message", lines, l.cfg.messageFont, lx, lineY-5, "left", "bottom")
	if e.deactivate {
		l.deactivate(e.from, endY)
	}
	return endY
}

// headLength and headWidth are the size of an arrowhead.
const headLength, headWidth = 8.0, 3.2

// end draws the mark at an end of a message, with its tip at x, y and
// pointing in dir, 1 to the right and -1 to the left.
func (l *seqLayout) end(kind seqEnd, x, y, dir float64) {
	b := -headLength * dir
	// up is the side of the half heads: above the line.
	up := -headWidth
	var set, path string
	switch kind {
	case endNone:
		return
	case endFilled:
		set, path = "heads", "M0,0L"+pathPoint(b, -headWidth)+"L"+pathPoint(b, headWidth)+"Z"
	case endOpen:
		set, path = "openHeads", "M"+pathPoint(b, -headWidth)+"L0,0L"+pathPoint(b, headWidth)
	case endCross:
		s := 4.0
		x -= s * dir
		set, path = "openHeads", "M"+pathPoint(-s, -s)+"L"+pathPoint(s, s)+"M"+pathPoint(-s, s)+"L"+pathPoint(s, -s)
	case endHalfTop:
		set, path = "heads", "M0,0L"+pathPoint(b, up)+"L"+pathPoint(b, 0)+"Z"
	case endHalfBottom:
		set, path = "heads", "M0,0L"+pathPoint(b, -up)+"L"+pathPoint(b, 0)+"Z"
	case endStickTop:
		set, path = "openHeads", "M0,0L"+pathPoint(b, up)
	case endStickBottom:
		set, path = "openHeads", "M0,0L"+pathPoint(b, -up)
	}
	l.add(set, map[string]any{"kind": "head", "x": x, "y": y, "path": path}, x-headLength, x+headLength, y+headLength)
}

// dotPath returns the path of the dot of a central connection.
func dotPath() string {
	return "M3.5,0A3.5,3.5 0 1,1 -3.5,0A3.5,3.5 0 1,1 3.5,0Z"
}

// badge draws the number of a message in a circle at a place.
func (l *seqLayout) badge(x, y float64) {
	n := strconv.FormatFloat(l.number, 'f', -1, 64)
	f := text.Font{Family: text.Family, Size: 9, Weight: 600}
	w, _ := l.size(f, []string{n})
	r := math.Max(8, w/2+4)
	l.add("badges", map[string]any{"kind": "number", "x": x, "y": y, "size": 4 * r * r, "text": n}, x-r, x+r, y+r)
}

// note draws a note at a y, and returns the y below it.
func (l *seqLayout) note(e seqEvent, y float64) float64 {
	i, ok1 := l.idx[e.from]
	j, ok2 := l.idx[e.to]
	if !ok1 || !ok2 {
		return y
	}
	f := l.cfg.noteFont
	lines := l.lines(e.text, f, l.cfg.wrapWidth)
	nw, nh := l.noteSize(e)
	top := y + l.gap()*0.6
	var x0 float64
	switch {
	case e.place == "right":
		x0 = l.attach(e.from, math.Inf(1)) + 8
	case e.place == "left":
		x0 = l.attach(e.from, math.Inf(-1)) - 8 - nw
	case i == j:
		x0 = l.cx[i] - nw/2
	default:
		a, b := math.Min(l.cx[i], l.cx[j]), math.Max(l.cx[i], l.cx[j])
		nw = math.Max(nw, b-a+48)
		x0 = (a+b)/2 - nw/2
	}
	l.add("notes", map[string]any{"kind": "note", "x": x0, "x2": x0 + nw, "y": top, "y2": top + nh}, x0, x0+nw, top+nh)
	tx := x0 + nw/2
	switch l.cfg.noteAlign {
	case "left":
		tx = x0 + l.cfg.noteMargin
	case "right":
		tx = x0 + nw - l.cfg.noteMargin
	}
	l.textRow("noteLabels", "note", lines, f, tx, top+nh/2-float64(len(lines)-1)*lh(f)/2, l.cfg.noteAlign, "middle")
	return top + nh
}

// closeFrame closes the innermost block at a y and draws it.
func (l *seqLayout) closeFrame(y float64) {
	f := l.frames[len(l.frames)-1]
	l.frames = l.frames[:len(l.frames)-1]
	pad := l.cfg.boxMargin
	// An empty block spans the lifelines.
	if math.IsInf(f.x0, 1) {
		f.x0, f.x1 = 0, 0
		if len(l.ps) > 0 {
			f.x0, f.x1 = l.cx[0], l.cx[len(l.ps)-1]
		}
	}
	x0, x1 := f.x0-pad, f.x1+pad
	if f.block == "rect" {
		row := map[string]any{"kind": "rect", "x": x0, "x2": x1, "y": f.y0, "y2": y}
		set := "rects"
		if f.fill != "" {
			row["fill"], row["opacity"] = sourceFill(f.fill)
			set = "rectsFilled"
		}
		l.add(set, row, x0, x1, y)
		return
	}
	mf := l.cfg.messageFont
	kf := text.Font{Family: text.Family, Size: mf.Size, Weight: 600}
	word := f.block
	if word == "par_over" {
		word = "par"
	}
	kw, _ := l.size(kf, []string{word})
	tabW := math.Max(kw+16, l.cfg.labelBoxWidth)
	tabH := l.cfg.labelBoxHeight
	cond := l.lines(f.text, mf, l.cfg.wrapWidth)
	cw, _ := l.size(mf, bracket(cond))
	need := tabW + 12 + cw + 12
	for _, s := range f.sections {
		sw, _ := l.size(mf, bracket(l.lines(s.text, mf, l.cfg.wrapWidth)))
		need = math.Max(need, sw+24)
	}
	x1 = math.Max(x1, x0+need)
	l.add("frames", map[string]any{"kind": "block", "block": word, "x": x0, "x2": x1, "y": f.y0, "y2": y}, x0, x1, y)
	l.add("tabs", map[string]any{"kind": "tab", "x": x0, "x2": x0 + tabW, "y": f.y0, "y2": f.y0 + tabH}, x0, x0+tabW, f.y0+tabH)
	l.textRow("tabLabels", "block", []string{word}, kf, x0+tabW/2, f.y0+tabH/2, "center", "middle")
	l.textRow("condLabels", "condition", bracket(cond), mf, x0+tabW+10, f.y0+l.cfg.boxTextMargin, "left", "top")
	for _, s := range f.sections {
		l.add("dividers", map[string]any{"kind": "divider", "x": x0, "x2": x1, "y": s.y}, x0, x1, s.y)
		l.textRow("condLabels", "condition", bracket(l.lines(s.text, mf, l.cfg.wrapWidth)), mf, x0+10, s.y+l.cfg.boxTextMargin, "left", "top")
	}
}

// bracket puts the lines of the condition of a block in brackets.
func bracket(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := slices.Clone(lines)
	out[0] = "[" + out[0]
	out[len(out)-1] += "]"
	return out
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31). The
// x of each row moves so that the diagram starts at 0.
func (l *seqLayout) spec() map[string]any {
	if math.IsInf(l.minX, 1) {
		l.minX, l.maxX = 0, 0
	}
	dx := -l.minX
	var data []any
	for _, set := range slices.Sorted(maps.Keys(l.rows)) {
		rows := l.rows[set]
		for _, r := range rows {
			m, _ := r.(map[string]any)
			for _, k := range []string{"x", "x2"} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round((v+dx)*100) / 100
				}
			}
			for _, k := range []string{"y", "y2"} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round(v*100) / 100
				}
			}
		}
		data = append(data, map[string]any{"name": set, "values": rows})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	color := map[string]any{"scale": "color", "field": "key"}
	var marks []any
	mark := func(name, typ, set, style string, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": name, "from": map[string]any{"data": set},
			"style": []any{style}, "encode": map[string]any{"update": enc},
		})
	}
	box := func(enc map[string]any) map[string]any {
		enc["x"], enc["x2"], enc["y"], enc["y2"] = field("x"), field("x2"), field("y"), field("y2")
		return enc
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = field("x"), field("y"), field("path")
		return enc
	}
	texts := func(set, style string, f text.Font) {
		mark(set, "text", set, style, map[string]any{
			"x": field("x"), "y": field("y"), "text": field("lines"), "align": field("align"), "baseline": field("baseline"),
			"fontSize": value(f.Size), "fontWeight": value(f.Weight), "lineHeight": value(lh(f)),
		})
	}
	cluster := graph.StyleCluster
	mark("boxes", "rect", "boxes", cluster, box(map[string]any{}))
	mark("boxesFilled", "rect", "boxesFilled", cluster, box(map[string]any{"fill": field("fill"), "fillOpacity": field("opacity")}))
	mark("rects", "rect", "rects", cluster, box(map[string]any{"strokeOpacity": value(0)}))
	mark("rectsFilled", "rect", "rectsFilled", cluster, box(map[string]any{
		"fill": field("fill"), "fillOpacity": field("opacity"), "strokeOpacity": value(0),
	}))
	texts("boxLabels", graph.StyleClusterTitle, l.boxFont())
	mark("frames", "rect", "frames", cluster, box(map[string]any{"fillOpacity": value(0)}))
	mark("dividers", "rule", "dividers", graph.StyleGuide, map[string]any{
		"x": field("x"), "x2": field("x2"), "y": field("y"), "strokeDash": value([]any{4, 3}),
	})
	mark("lifelines", "rule", "lifelines", graph.StyleGuide, map[string]any{
		"x": field("x"), "y": field("y"), "y2": field("y2"), "strokeWidth": value(1),
	})
	mark("activationBacks", "rect", "activations", graph.StyleEdgeLabelBox, box(map[string]any{
		"fillOpacity": value(1), "cornerRadius": value(2),
	}))
	mark("activations", "rect", "activations", graph.StyleNode, box(map[string]any{
		"fill": color, "stroke": color, "fillOpacity": value(0.3), "strokeWidth": value(1), "cornerRadius": value(2),
	}))
	mark("messages", "path", "messages", graph.StyleEdge, path(map[string]any{"strokeWidth": value(1.5)}))
	mark("messagesDotted", "path", "messagesDotted", graph.StyleEdge, path(map[string]any{
		"strokeWidth": value(1.5), "strokeDash": value([]any{4, 3}),
	}))
	mark("heads", "path", "heads", graph.StyleArrow, path(map[string]any{}))
	mark("openHeads", "path", "openHeads", graph.StyleEdge, path(map[string]any{"strokeWidth": value(1.5)}))
	mark("dots", "path", "dots", graph.StyleArrow, path(map[string]any{}))
	mark("notes", "rect", "notes", cluster, box(map[string]any{"fillOpacity": value(1)}))
	mark("tabs", "rect", "tabs", cluster, box(map[string]any{"fillOpacity": value(1), "cornerRadius": value(4)}))
	mark("shapes", "path", "shapes", graph.StyleNode, path(map[string]any{"fill": color, "stroke": color}))
	mark("shapeBacks", "path", "shapeBacks", graph.StyleEdgeLabelBox, path(map[string]any{"fillOpacity": value(1)}))
	mark("shapeFronts", "path", "shapeBacks", graph.StyleNode, path(map[string]any{"fill": color, "stroke": color}))
	mark("icons", "path", "icons", graph.StyleNode, path(map[string]any{"fill": color, "stroke": color, "strokeWidth": value(1.5)}))
	circle := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["size"], enc["shape"] = field("x"), field("y"), field("size"), value("circle")
		return enc
	}
	mark("badgeBacks", "symbol", "badges", graph.StyleEdgeLabelBox, circle(map[string]any{"fillOpacity": value(1), "strokeWidth": value(0)}))
	mark("badges", "symbol", "badges", graph.StyleNode, circle(map[string]any{}))
	mark("numbers", "text", "badges", graph.StyleNodeLabel, map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "align": value("center"), "baseline": value("middle"),
		"fontSize": value(9), "fontWeight": value(600),
	})
	texts("tabLabels", graph.StyleClusterTitle, text.Font{Size: l.cfg.messageFont.Size, Weight: 600})
	texts("condLabels", graph.StyleEdgeLabel, l.cfg.messageFont)
	texts("messageLabels", graph.StyleNodeLabel, l.cfg.messageFont)
	texts("noteLabels", graph.StyleNodeLabel, l.cfg.noteFont)
	texts("actorLabels", graph.StyleNodeLabel, l.cfg.actorFont)
	var domain []any
	for _, p := range l.ps {
		domain = append(domain, p.id)
	}
	return map[string]any{
		"width":    math.Ceil(l.maxX - l.minX),
		"height":   math.Ceil(l.maxY),
		"autosize": "pad",
		"padding": map[string]any{
			"left": l.cfg.marginX, "right": l.cfg.marginX, "top": l.cfg.marginY, "bottom": l.cfg.marginY,
		},
		"data":   data,
		"scales": []any{map[string]any{"name": "color", "type": "ordinal", "domain": domain, "range": "category"}},
		"marks":  marks,
	}
}
