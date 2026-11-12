package mermaid

import (
	"math"
	"strings"

	"github.com/xo/zuzu/text"
)

// A label of a sequence diagram wraps where Mermaid 12.3 wraps it (D32).
// Mermaid does not wrap each label at sequence.width. It wraps a message
// at the width between its two lifelines, a note at the width of the
// note, and the label of a frame at the width of the frame, each less
// wrapPadding on each side. These widths come from where Mermaid puts its
// participants, which is not where zuzu puts them. So seqWraps works out
// the places of the participants as Mermaid does, in the pixels of
// Mermaid, and from them the width at which each label wraps. scaleWrap
// then turns each width into the pixels of zuzu.

// seqMermaid holds the settings of Mermaid that its widths read, in the
// pixels of Mermaid. Each has the default of Mermaid, which can differ
// from the default of the layout of zuzu.
type seqMermaid struct {
	// width is sequence.width, the least width of a participant and the
	// least width at which a label wraps.
	width float64
	// actorMargin is sequence.actorMargin, the least space between two
	// participants.
	actorMargin float64
	// labelBoxWidth is sequence.labelBoxWidth, which Mermaid takes from
	// the width of a frame for each message in the frame.
	labelBoxWidth float64
}

// seqMermaidFrame is an open frame of the model of Mermaid: its extent,
// its width, and the events whose labels wrap at its width, which are its
// start and each section with a label.
type seqMermaidFrame struct {
	from, to, width float64
	events          []int
}

// seqWraps returns the width in the pixels of zuzu at which each message,
// note and label of a frame wraps, by the index of its event.
func (l *seqLayout) seqWraps() map[int]float64 {
	cfg, mc := l.cfg, l.cfg.mermaid
	kind := l.s.c.d.Kind
	ratio := func(label string) float64 { return scaleWrap(kind, label, 1) }
	// width returns the width in the pixels of Mermaid of a text, wrapped
	// at a width in the pixels of Mermaid when it wraps.
	width := func(t seqText, f text.Font, label string, wrap float64) float64 {
		w, _ := l.size(f, l.lines(t, f, wrap*ratio(label)))
		return w / ratio(label)
	}
	n := len(l.ps)
	wp := cfg.wrapPadding
	// The width of each participant, as calculateActorMargins of Mermaid
	// works it out.
	ws := make([]float64, n)
	for i, p := range l.ps {
		ws[i] = mc.width
		if !l.wraps(p.label) {
			ws[i] = math.Max(mc.width, width(p.label, cfg.actorFont, "", 0)+2*wp)
		}
	}
	// The widest message or note between each participant and the next,
	// as getMaxMessageWidthPerActor of Mermaid works it out.
	most := make([]float64, n)
	grow := func(i int, w float64) {
		if i >= 0 && i < n {
			most[i] = math.Max(most[i], w)
		}
	}
	for _, e := range l.s.events {
		i, ok1 := l.idx[e.from]
		j, ok2 := l.idx[e.to]
		if !ok1 || !ok2 || e.kind != "message" && e.kind != "note" {
			continue
		}
		f, label := cfg.messageFont, labelMessage
		if e.kind == "note" {
			f, label = cfg.noteFont, ""
		}
		w := width(e.text, f, label, mc.width-2*wp) + 2*wp
		switch {
		case e.kind == "message" && i == j+1:
			grow(j, w)
		case e.kind == "message" && i == j-1:
			grow(i, w)
		case e.kind == "message" && i == j:
			grow(i, w/2)
		case e.kind == "message":
		case e.place == "right" && j < n-1:
			grow(i, w)
		case e.place == "left" && j > 0:
			grow(j-1, w)
		case e.place == "over":
			if j > 0 {
				grow(j-1, w/2)
			}
			if j < n-1 {
				grow(i, w/2)
			}
		}
	}
	// The left edge and the center of each participant, as
	// addActorRenderingData of Mermaid puts them.
	xs, cs := make([]float64, n), make([]float64, n)
	for i := range n {
		if i > 0 {
			margin := math.Max(most[i-1]+mc.actorMargin-ws[i-1]/2-ws[i]/2, mc.actorMargin)
			xs[i] = xs[i-1] + ws[i-1] + margin
		}
		cs[i] = xs[i] + ws[i]/2
	}
	out := map[int]float64{}
	acts := map[int][][2]float64{}
	aw := cfg.activationWidth
	activate := func(i int) {
		x := cs[i] + float64(len(acts[i])-1)*aw/2
		acts[i] = append(acts[i], [2]float64{x, x + aw})
	}
	deactivate := func(i int) {
		if len(acts[i]) > 0 {
			acts[i] = acts[i][:len(acts[i])-1]
		}
	}
	bounds := func(i int) (float64, float64) {
		left, right := cs[i]-1, cs[i]+1
		for _, a := range acts[i] {
			left, right = math.Min(left, a[0]), math.Max(right, a[1])
		}
		return left, right
	}
	var frames []*seqMermaidFrame
	for k, e := range l.s.events {
		switch e.kind {
		case "activate", "deactivate":
			if i, ok := l.idx[e.from]; ok {
				if e.kind == "activate" {
					activate(i)
				} else {
					deactivate(i)
				}
			}
		case "start":
			if e.block != "rect" {
				frames = append(frames, &seqMermaidFrame{from: math.Inf(1), to: math.Inf(-1), events: []int{k}})
			}
		case "section":
			if len(frames) > 0 && len(e.text.lines) > 0 {
				frames[len(frames)-1].events = append(frames[len(frames)-1].events, k)
			}
		case "end":
			if e.block != "rect" && len(frames) > 0 {
				fr := frames[len(frames)-1]
				frames = frames[:len(frames)-1]
				if fr.width != 0 {
					for _, ev := range fr.events {
						out[ev] = (fr.width - 2*wp) * ratio(labelMessage)
					}
				}
			}
		case "message":
			i, ok1 := l.idx[e.from]
			j, ok2 := l.idx[e.to]
			if !ok1 || !ok2 {
				continue
			}
			startx, stopx := l.seqMessageEnds(e, i, j, bounds)
			bounded := math.Abs(startx - stopx)
			out[k] = math.Max(bounded+2*wp, mc.width) * ratio(labelMessage)
			// buildMessageModel of Mermaid gives the message this width,
			// which the frames around it read.
			msgW := math.Max(bounded+2*wp, mc.width)
			if !l.wraps(e.text) {
				msgW = math.Max(msgW, width(e.text, cfg.messageFont, labelMessage, 0)+2*wp)
			}
			for _, fr := range frames {
				if startx == stopx {
					fr.from = min(xs[i]-msgW/2, xs[i]-ws[i]/2, fr.from)
					fr.to = max(xs[j]+msgW/2, xs[j]+ws[i]/2, fr.to)
					fr.width = math.Max(fr.width, math.Abs(fr.to-fr.from)) - mc.labelBoxWidth
				} else {
					fr.from = math.Min(startx, fr.from)
					fr.to = math.Max(stopx, fr.to)
					fr.width = math.Max(fr.width, msgW) - mc.labelBoxWidth
				}
			}
			// Mermaid activates the receiver of + and deactivates the
			// sender of - after the message.
			if e.activate {
				activate(j)
			}
			if e.deactivate {
				deactivate(i)
			}
		case "note":
			i, ok1 := l.idx[e.from]
			j, ok2 := l.idx[e.to]
			if !ok1 || !ok2 {
				continue
			}
			startx, w := l.seqNoteModel(e, i, j, xs, cs, ws, width)
			out[k] = (w - 2*wp) * ratio("")
			for _, fr := range frames {
				fr.from = math.Min(fr.from, startx)
				fr.to = math.Max(fr.to, startx+w)
				fr.width = math.Max(fr.width, math.Abs(fr.from-fr.to)) - mc.labelBoxWidth
			}
		}
	}
	return out
}

// seqMessageEnds returns the x of the start and of the end of a message
// from participant i to participant j in the model of Mermaid, as
// buildMessageModel of Mermaid works them out. bounds returns the left and
// the right of the lifeline of a participant and its activations.
func (l *seqLayout) seqMessageEnds(e seqEvent, i, j int, bounds func(int) (float64, float64)) (float64, float64) {
	fromLeft, fromRight := bounds(i)
	toLeft, toRight := bounds(j)
	right := fromLeft <= toLeft
	startx, stopx := fromLeft, toRight
	if right {
		startx, stopx = fromRight, toLeft
	}
	if i == j {
		return startx, startx
	}
	// in moves an x toward the start of the message.
	in := func(v float64) float64 {
		if right {
			return -v
		}
		return v
	}
	if e.activate && toRight-toLeft <= 2 {
		stopx += in(l.cfg.activationWidth/2 - 1)
	}
	switch e.arrow.head {
	case endFilled, endOpen, endCross, endHalfTop, endHalfBottom:
		stopx += in(3)
	}
	switch e.arrow.tail {
	case endFilled, endHalfTop, endHalfBottom:
		startx -= in(3)
	}
	return startx, stopx
}

// seqNoteModel returns the x of the left of a note and its width in the
// model of Mermaid, as buildNoteModel of Mermaid works them out. xs, cs
// and ws are the left edges, the centers and the widths of the
// participants, and width measures a text in the pixels of Mermaid.
func (l *seqLayout) seqNoteModel(e seqEvent, i, j int, xs, cs, ws []float64, width func(seqText, text.Font, string, float64) float64) (float64, float64) {
	cfg, mc := l.cfg, l.cfg.mermaid
	wrap := l.wraps(e.text)
	tw := width(e.text, cfg.noteFont, "", mc.width)
	switch {
	case e.place == "right":
		w := math.Max(ws[i]/2+ws[j]/2, tw+2*cfg.noteMargin)
		if wrap {
			w = math.Max(mc.width, tw)
		}
		return xs[i] + (ws[i]+mc.actorMargin)/2, w
	case e.place == "left":
		w := math.Max(ws[i]/2+ws[j]/2, tw+2*cfg.noteMargin)
		if wrap {
			w = math.Max(mc.width, tw+2*cfg.noteMargin)
		}
		return xs[i] - w + (ws[i]-mc.actorMargin)/2, w
	case i == j:
		w := math.Max(mc.width, ws[i])
		if !wrap {
			tw = width(e.text, cfg.noteFont, "", w)
			w = math.Max(w, tw+2*cfg.noteMargin)
		}
		return xs[i] + (ws[i]-w)/2, w
	}
	w := math.Abs(cs[i]-cs[j]) + mc.actorMargin
	return math.Min(cs[i], cs[j]) - mc.actorMargin/2, w
}

// wraps reports whether a text wraps: by its own wrap: or nowrap:, or by
// the setting of the diagram.
func (l *seqLayout) wraps(t seqText) bool {
	if t.wrap != nil {
		return *t.wrap
	}
	return l.cfg.wrap
}

// wrapOf returns the width in the pixels of zuzu at which the label of an
// event wraps, or def when the model of Mermaid gives none.
func (l *seqLayout) wrapOf(k int, def float64) float64 {
	if w, ok := l.wrap[k]; ok {
		return w
	}
	return def
}

// frameLines returns the lines of the label of a frame, wrapped at a
// width as Mermaid wraps it: always, with its brackets, which the caller
// puts back with bracket. A label with a break in it does not wrap.
func (l *seqLayout) frameLines(t seqText, f text.Font, width float64) []string {
	if len(t.lines) != 1 || width <= 0 {
		return t.lines
	}
	yes := true
	lines := l.lines(seqText{lines: []string{"[" + t.lines[0] + "]"}, wrap: &yes}, f, width)
	lines[0] = strings.TrimPrefix(lines[0], "[")
	lines[len(lines)-1] = strings.TrimSuffix(lines[len(lines)-1], "]")
	return lines
}
