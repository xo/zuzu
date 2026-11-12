package mermaid

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// The steps of the layout of a git graph, in pixels, as Mermaid 12.3 lays
// it out: the step from one commit to the next, the offset of a commit in
// its step, the start of a vertical graph, the space between two lanes,
// and the extra space for labels that turn.
const (
	gitStep      = 40.0
	gitOffset    = 10.0
	gitStart     = 30.0
	gitLaneSpace = 50.0
	gitTurnSpace = 40.0
	// gitR is the radius of a commit in the look of zuzu.
	gitR = 7.0
)

// gitBreak matches a break in the name of a branch: a new line, the two
// characters \n, or <br>.
var gitBreak = regexp.MustCompile(`(?i)\\n|\n|<br\s*/?>`)

// gitPoint is the center of a commit.
type gitPoint struct{ x, y float64 }

// gitLayout lays out a git graph and writes its rows.
type gitLayout struct {
	g        *gitGraph
	m        *text.Estimate
	vertical bool
	branches []gitBranch
	// lane is the place of the lane of each branch across the graph.
	lane map[string]float64
	// labels are the lines of the name of each branch, and labelW their
	// width.
	labels map[string][]string
	labelW map[string]float64
	pos    map[string]gitPoint
	// maxPos is the end of the lanes along the graph.
	maxPos float64
	// lanes are the places across the graph that a line already takes,
	// which a link that goes around commits keeps away from.
	lanes []float64

	rows                   map[string][]any
	minX, minY, maxX, maxY float64
}

// The fonts of the labels of a git graph.
var (
	gitBranchFont = text.Font{Family: text.Family, Size: 11, Weight: 600}
	gitLabelFont  = text.Font{Family: text.Family, Size: 10, Weight: 400}
)

// newGitLayout lays out a git graph.
func newGitLayout(g *gitGraph) *gitLayout {
	l := &gitLayout{
		g: g, m: text.NewEstimate(), vertical: g.dir != "LR", branches: g.sortedBranches(),
		lane: map[string]float64{}, labels: map[string][]string{}, labelW: map[string]float64{},
		pos: map[string]gitPoint{}, rows: map[string][]any{},
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	p := 0.0
	for _, b := range l.branches {
		var lines []string
		for _, s := range gitBreak.Split(b.name, -1) {
			lines = append(lines, strings.TrimSpace(s))
		}
		w := 0.0
		for _, s := range lines {
			w = math.Max(w, l.m.Width(gitBranchFont, s))
		}
		l.labels[b.name], l.labelW[b.name] = lines, math.Ceil(w)
		l.lane[b.name] = p
		p += gitLaneSpace
		if g.cfg.rotate {
			p += gitTurnSpace
		}
		if l.vertical {
			p += l.labelW[b.name] / 2
		}
	}
	l.place()
	l.draw()
	return l
}

// sorted returns the commits in the order of their numbers.
func (l *gitLayout) sorted() []*gitCommit {
	out := make([]*gitCommit, 0, len(l.g.ids))
	for _, id := range l.g.ids {
		out = append(out, l.g.commits[id])
	}
	slices.SortFunc(out, func(a, b *gitCommit) int { return a.seq - b.seq })
	return out
}

// spine returns the place across the graph of the line of a branch. In a
// graph from left to right, it is 2 pixels above the place of the lane, as
// in Mermaid.
func (l *gitLayout) spine(branch string) float64 {
	if l.vertical {
		return l.lane[branch]
	}
	return l.lane[branch] - 2
}

// along returns the place of a commit along the graph.
func (l *gitLayout) along(p gitPoint) float64 {
	if l.vertical {
		return p.y
	}
	return p.x
}

// at returns the center of a commit at a place along the graph.
func (l *gitLayout) at(c *gitCommit, along float64) gitPoint {
	if l.vertical {
		return gitPoint{l.lane[c.branch], along}
	}
	return gitPoint{along, l.spine(c.branch)}
}

// closest returns the parent of a commit that is the furthest along the
// graph: the lowest in a graph from bottom to top, and else the highest.
// Of two at the same place, the last wins. It reports false when no
// parent has a place.
func (l *gitLayout) closest(c *gitCommit) (gitPoint, bool) {
	var best gitPoint
	found := false
	for _, p := range c.parents {
		pp, ok := l.pos[p]
		if !ok {
			continue
		}
		v := l.along(pp)
		switch {
		case l.g.dir == "BT" && (!found || v <= best.y):
			best, found = pp, true
		case l.g.dir != "BT" && v >= 0 && (!found || v >= l.along(best)):
			best, found = pp, true
		}
	}
	return best, found
}

// place works out the center of each commit. Without parallel commits,
// each commit takes the next step along the graph in the order of the
// source. With them, a commit takes the step after its closest parent, so
// that commits on two branches can share a step.
func (l *gitLayout) place() {
	commits := l.sorted()
	bt, parallel := l.g.dir == "BT", l.g.cfg.parallel
	if bt && parallel {
		l.placeBT(commits)
	}
	if bt {
		slices.Reverse(commits)
	}
	pos := 0.0
	if l.vertical {
		pos = gitStart
	}
	for _, c := range commits {
		if parallel {
			pos = l.parallelPos(c)
		}
		along := pos + gitOffset
		if bt && parallel {
			along = pos
		}
		l.pos[c.id] = l.at(c, along)
		if bt && parallel {
			pos += gitStep
		} else {
			pos += gitStep + gitOffset
		}
		l.maxPos = math.Max(l.maxPos, pos)
	}
}

// parallelPos returns the step of a commit with parallel commits.
func (l *gitLayout) parallelPos(c *gitCommit) float64 {
	switch l.g.dir {
	case "BT":
		return l.pos[c.id].y - gitStep
	case "TB":
		if len(c.parents) == 0 {
			return gitStart
		}
		if p, ok := l.closest(c); ok {
			return p.y + gitStep
		}
	default:
		if p, ok := l.closest(c); ok {
			return p.x + gitStep
		}
	}
	return 0
}

// placeBT places the commits of a graph from bottom to top with parallel
// commits, before the steps of place. Each root goes below the lowest
// commit, and each other commit one step above its highest parent.
func (l *gitLayout) placeBT(commits []*gitCommit) {
	cur, low := gitStart, gitStart
	var roots []*gitCommit
	for _, c := range commits {
		if len(c.parents) > 0 {
			if p, ok := l.closest(c); ok {
				cur = p.y + gitStep
			}
			low = math.Max(low, cur)
		} else {
			roots = append(roots, c)
		}
		l.pos[c.id] = gitPoint{l.lane[c.branch], cur + gitOffset}
	}
	for _, c := range roots {
		l.pos[c.id] = gitPoint{l.lane[c.branch], low + gitStart}
	}
	for _, c := range commits {
		if len(c.parents) == 0 {
			continue
		}
		p, _ := l.closest(c)
		cur = p.y - gitStep
		l.pos[c.id] = gitPoint{l.lane[c.branch], cur - gitOffset}
	}
}

// add appends a row to a set, and grows the bounds by a box.
func (l *gitLayout) add(set string, row map[string]any, x0, y0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.grow(x0, y0)
	l.grow(x1, y1)
}

// grow grows the bounds by a point.
func (l *gitLayout) grow(x, y float64) {
	l.minX, l.maxX = math.Min(l.minX, x), math.Max(l.maxX, x)
	l.minY, l.maxY = math.Min(l.minY, y), math.Max(l.maxY, y)
}

// draw writes the rows of the lanes, the labels of the branches, the
// links, the commits, their labels and their tags.
func (l *gitLayout) draw() {
	if l.g.cfg.showBranches {
		for _, b := range l.branches {
			l.branch(b.name)
		}
	}
	for _, id := range l.g.ids {
		c := l.g.commits[id]
		for _, p := range c.parents {
			if pc, ok := l.g.commits[p]; ok {
				l.link(pc, c)
			}
		}
	}
	for _, c := range l.sorted() {
		l.commit(c)
	}
}

// branch draws the lane of a branch and its label.
func (l *gitLayout) branch(name string) {
	s := l.spine(name)
	l.lanes = append(l.lanes, s)
	lines := l.labels[name]
	w, h := l.labelW[name]+18, float64(len(lines))*lh(gitBranchFont)+8
	var x0, y0 float64
	switch l.g.dir {
	case "TB":
		l.add("lanes", map[string]any{"key": name, "x": s, "y": gitStart, "x2": s, "y2": l.maxPos}, s, gitStart, s, l.maxPos)
		x0, y0 = s-w/2, 0
	case "BT":
		l.add("lanes", map[string]any{"key": name, "x": s, "y": gitStart, "x2": s, "y2": l.maxPos}, s, gitStart, s, l.maxPos)
		x0, y0 = s-w/2, l.maxPos+4
	default:
		l.add("lanes", map[string]any{"key": name, "x": 0.0, "y": s, "x2": l.maxPos, "y2": s}, 0, s, l.maxPos, s)
		right := -5.0
		if l.g.cfg.rotate {
			right -= 30
		}
		x0, y0 = right-w, s-h/2
	}
	l.add("branchBoxes", map[string]any{"key": name, "x": x0, "y": y0, "x2": x0 + w, "y2": y0 + h}, x0, y0, x0+w, y0+h)
	l.add("branchLabels", map[string]any{
		"key": name, "x": x0 + w/2, "y": y0 + h/2, "lines": lineRows(lines), "text": strings.Join(lines, " "),
	}, x0, y0, x0+w, y0+h)
}

// findLane returns a place between two places for the middle of a link
// that goes around commits. It keeps 10 pixels away from each place that a
// line takes, and moves toward the first place when it cannot, as Mermaid
// does.
func (l *gitLayout) findLane(a, b float64, depth int) float64 {
	c := a + math.Abs(a-b)/2
	if depth > 5 {
		return c
	}
	if !slices.ContainsFunc(l.lanes, func(v float64) bool { return math.Abs(v-c) < 10 }) {
		l.lanes = append(l.lanes, c)
		return c
	}
	return l.findLane(a, b-math.Abs(a-b)/5, depth+1)
}

// reroute reports whether a link from a commit to its child goes around
// the commits between them: a commit between the two in the source is on
// the branch that the link runs along.
func (l *gitLayout) reroute(a, b *gitCommit, p1, p2 gitPoint) bool {
	furthest := p1.y < p2.y
	if l.vertical {
		furthest = p1.x < p2.x
	}
	branch := a.branch
	if furthest {
		branch = b.branch
	}
	for _, c := range l.g.commits {
		if c.seq > a.seq && c.seq < b.seq && c.branch == branch {
			return true
		}
	}
	return false
}

// gitPath writes a path of moves, lines and arcs.
type gitPath struct {
	b strings.Builder
}

func gitNum(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func (p *gitPath) move(x, y float64) *gitPath {
	p.b.WriteString("M" + gitNum(x) + "," + gitNum(y))
	return p
}

func (p *gitPath) line(x, y float64) *gitPath {
	p.b.WriteString("L" + gitNum(x) + "," + gitNum(y))
	return p
}

// arc draws a quarter of a circle of a radius to a point, clockwise when
// sweep is 1.
func (p *gitPath) arc(r float64, sweep int, x, y float64) *gitPath {
	p.b.WriteString("A" + gitNum(r) + "," + gitNum(r) + " 0 0 " + strconv.Itoa(sweep) + " " + gitNum(x) + "," + gitNum(y))
	return p
}

// link draws the link from a commit to its child, in the color of a
// branch. The link turns once with an arc of 20, or, when it goes around
// commits, runs in a lane of its own and turns twice with arcs of 10.
func (l *gitLayout) link(a, b *gitCommit) {
	p1, p2 := l.pos[a.id], l.pos[b.id]
	key := b.branch
	// The link to a merge from the branch that it merges takes the color
	// of that branch.
	second := b.typ == gitMerge && len(b.parents) > 0 && a.id != b.parents[0]
	if second {
		key = a.branch
	}
	p := &gitPath{}
	p.move(p1.x, p1.y)
	if l.reroute(a, b, p1, p2) {
		const r = 10.0
		var lineY, lineX float64
		if p1.y < p2.y {
			lineY = l.findLane(p1.y, p2.y, 0)
		} else {
			lineY = l.findLane(p2.y, p1.y, 0)
		}
		if p1.x < p2.x {
			lineX = l.findLane(p1.x, p2.x, 0)
		} else {
			lineX = l.findLane(p2.x, p1.x, 0)
		}
		switch l.g.dir {
		case "TB":
			if p1.x < p2.x {
				p.line(lineX-r, p1.y).arc(r, 1, lineX, p1.y+r).line(lineX, p2.y-r).arc(r, 0, lineX+r, p2.y)
			} else {
				key = a.branch
				p.line(lineX+r, p1.y).arc(r, 0, lineX, p1.y+r).line(lineX, p2.y-r).arc(r, 1, lineX-r, p2.y)
			}
		case "BT":
			if p1.x < p2.x {
				p.line(lineX-r, p1.y).arc(r, 0, lineX, p1.y-r).line(lineX, p2.y+r).arc(r, 1, lineX+r, p2.y)
			} else {
				key = a.branch
				p.line(lineX+r, p1.y).arc(r, 1, lineX, p1.y-r).line(lineX, p2.y+r).arc(r, 0, lineX-r, p2.y)
			}
		default:
			if p1.y < p2.y {
				p.line(p1.x, lineY-r).arc(r, 0, p1.x+r, lineY).line(p2.x-r, lineY).arc(r, 1, p2.x, lineY+r)
			} else {
				key = a.branch
				p.line(p1.x, lineY+r).arc(r, 1, p1.x+r, lineY).line(p2.x-r, lineY).arc(r, 0, p2.x, lineY-r)
			}
		}
		l.grow(lineX, lineY)
	} else {
		const r = 20.0
		switch {
		case l.g.dir == "TB" && p1.x < p2.x && second:
			p.line(p1.x, p2.y-r).arc(r, 0, p1.x+r, p2.y)
		case l.g.dir == "TB" && p1.x < p2.x:
			p.line(p2.x-r, p1.y).arc(r, 1, p2.x, p1.y+r)
		case l.g.dir == "TB" && p1.x > p2.x && second:
			p.line(p1.x, p2.y-r).arc(r, 1, p1.x-r, p2.y)
		case l.g.dir == "TB" && p1.x > p2.x:
			p.line(p2.x+r, p1.y).arc(r, 0, p2.x, p1.y+r)
		case l.g.dir == "BT" && p1.x < p2.x && second:
			p.line(p1.x, p2.y+r).arc(r, 1, p1.x+r, p2.y)
		case l.g.dir == "BT" && p1.x < p2.x:
			p.line(p2.x-r, p1.y).arc(r, 0, p2.x, p1.y-r)
		case l.g.dir == "BT" && p1.x > p2.x && second:
			p.line(p1.x, p2.y+r).arc(r, 0, p1.x-r, p2.y)
		case l.g.dir == "BT" && p1.x > p2.x:
			p.line(p2.x+r, p1.y).arc(r, 1, p2.x, p1.y-r)
		case l.vertical:
		case p1.y < p2.y && second:
			p.line(p2.x-r, p1.y).arc(r, 1, p2.x, p1.y+r)
		case p1.y < p2.y:
			p.line(p1.x, p2.y-r).arc(r, 0, p1.x+r, p2.y)
		case p1.y > p2.y && second:
			p.line(p2.x-r, p1.y).arc(r, 0, p2.x, p1.y-r)
		case p1.y > p2.y:
			p.line(p1.x, p2.y+r).arc(r, 1, p1.x+r, p2.y)
		}
	}
	p.line(p2.x, p2.y)
	l.add("links", map[string]any{"key": key, "x": 0.0, "y": 0.0, "path": p.b.String()}, p1.x, p1.y, p2.x, p2.y)
}

// circlePath returns the path of a circle of a radius around a point.
func circlePath(cx, cy, r float64) string {
	return "M" + gitNum(cx+r) + "," + gitNum(cy) + "A" + gitNum(r) + "," + gitNum(r) + " 0 1,1 " + gitNum(cx-r) + "," + gitNum(cy) +
		"A" + gitNum(r) + "," + gitNum(r) + " 0 1,1 " + gitNum(cx+r) + "," + gitNum(cy) + "Z"
}

// squarePath returns the path of a square of a half side around the
// origin.
func squarePath(h float64) string {
	n := gitNum(h)
	return "M-" + n + ",-" + n + "H" + n + "V" + n + "H-" + n + "Z"
}

// barPath returns the path of a bar of a half width from one point to
// another. Two bars from the same call turn the same way, so they fill
// where they cross.
func barPath(x0, y0, x1, y1, t float64) string {
	dx, dy := x1-x0, y1-y0
	d := math.Hypot(dx, dy)
	nx, ny := -dy/d*t, dx/d*t
	return "M" + gitNum(x0+nx) + "," + gitNum(y0+ny) + "L" + gitNum(x1+nx) + "," + gitNum(y1+ny) +
		"L" + gitNum(x1-nx) + "," + gitNum(y1-ny) + "L" + gitNum(x0-nx) + "," + gitNum(y0-ny) + "Z"
}

// commit draws a commit, its label and its tags. A commit is a dot in the
// color of its branch, with a ring of the background around it. A merge
// has a hole, a reverse commit a cross, a highlight is a square with a
// hole, and a cherry-pick has the two cherries of Mermaid.
func (l *gitLayout) commit(c *gitCommit) {
	p := l.pos[c.id]
	row := func(path string) map[string]any {
		return map[string]any{"key": c.branch, "id": c.id, "x": p.x, "y": p.y, "path": path}
	}
	shape, back := circlePath(0, 0, gitR), circlePath(0, 0, gitR+2)
	var marks []string
	switch c.symbol() {
	case gitMerge:
		marks = append(marks, circlePath(0, 0, gitR/2))
	case gitReverse:
		const a = 3.5
		marks = append(marks, barPath(-a, -a, a, a, 0.9)+barPath(-a, a, a, -a, 0.9))
	case gitHighlight:
		shape, back = squarePath(gitR), squarePath(gitR+2)
		marks = append(marks, squarePath(gitR/2-0.5))
	case gitCherryPick:
		marks = append(marks, circlePath(-2.2, 1.6, 1.9)+circlePath(2.2, 1.6, 1.9)+
			barPath(-2.2, 1, 0, -4, 0.45)+barPath(2.2, 1, 0, -4, 0.45))
	}
	e := gitR + 2
	l.add("commitBacks", row(back), p.x-e, p.y-e, p.x+e, p.y+e)
	kind := map[gitType]string{
		gitNormal: "normal", gitReverse: "reverse", gitHighlight: "highlight", gitMerge: "merge", gitCherryPick: "cherry-pick",
	}[c.symbol()]
	r := row(shape)
	r["type"] = kind
	l.rows["commits"] = append(l.rows["commits"], r)
	for _, m := range marks {
		l.rows["commitMarks"] = append(l.rows["commitMarks"], row(m))
	}
	if l.g.cfg.showCommitLabel && c.typ != gitCherryPick && (c.typ != gitMerge || c.customID) {
		l.commitLabel(c, p)
	}
	l.tags(c, p)
}

// commitLabel draws the id of a commit under it, or at its left in a
// vertical graph. A label that turns runs up to the right, at 45 degrees,
// and ends near the commit.
func (l *gitLayout) commitLabel(c *gitCommit, p gitPoint) {
	w, h := math.Ceil(l.m.Width(gitLabelFont, c.id)), lh(gitLabelFont)
	var x, y, angle float64
	align, baseline := "center", "top"
	switch {
	case l.vertical && l.g.cfg.rotate:
		x, y, align, baseline, angle = p.x-gitR-3, p.y+gitR+3, "right", "middle", -45
	case l.vertical:
		x, y, align, baseline = p.x-gitR-6, p.y, "right", "middle"
	case l.g.cfg.rotate:
		x, y, align, baseline, angle = p.x+2, p.y+gitR+5, "right", "middle", -45
	default:
		x, y = p.x, p.y+gitR+5
	}
	// The box of the text, turned about its anchor.
	x0 := map[string]float64{"left": 0, "center": -w / 2, "right": -w}[align]
	y0 := map[string]float64{"top": 0, "middle": -h / 2}[baseline]
	sin, cos := math.Sincos(angle * math.Pi / 180)
	for _, q := range [][2]float64{{x0, y0}, {x0 + w, y0}, {x0, y0 + h}, {x0 + w, y0 + h}} {
		l.grow(x+q[0]*cos-q[1]*sin, y+q[0]*sin+q[1]*cos)
	}
	l.rows["commitLabels"] = append(l.rows["commitLabels"], map[string]any{
		"key": c.branch, "text": c.id, "x": x, "y": y, "align": align, "baseline": baseline, "angle": angle,
	})
}

// tags draws the tags of a commit: labels with a point and a hole, the
// first tag of the source the furthest from the commit. They stack above
// the commit, or at its right in a vertical graph.
func (l *gitLayout) tags(c *gitCommit, p gitPoint) {
	if len(c.tags) == 0 {
		return
	}
	tw := 0.0
	for _, t := range c.tags {
		tw = math.Max(tw, math.Ceil(l.m.Width(gitLabelFont, t)))
	}
	h := lh(gitLabelFont) + 4
	for k := range c.tags {
		t := c.tags[len(c.tags)-1-k]
		var x0, cy float64
		if l.vertical {
			x0, cy = p.x+gitR+12, p.y+float64(k)*(h+4)
		} else {
			x0, cy = p.x-tw/2-4, p.y-gitR-14-float64(k)*(h+4)
		}
		x1, top, bottom := x0+tw+8, cy-h/2, cy+h/2
		tip := x0 - 7
		path := "M" + gitNum(tip) + "," + gitNum(cy) + "L" + gitNum(x0) + "," + gitNum(top) + "H" + gitNum(x1) + "V" + gitNum(bottom) + "H" + gitNum(x0) + "Z"
		l.add("tags", map[string]any{"key": c.branch, "x": 0.0, "y": 0.0, "path": path}, tip, top, x1, bottom)
		l.rows["tagHoles"] = append(l.rows["tagHoles"], map[string]any{"x": 0.0, "y": 0.0, "path": circlePath(x0-1.5, cy, 1.4)})
		l.rows["tagLabels"] = append(l.rows["tagLabels"], map[string]any{"key": c.branch, "x": x0 + 4 + tw/2, "y": cy, "text": t})
	}
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31).
// Each branch takes the next color of the scheme. The rows move so that
// the graph starts at 0.
func (l *gitLayout) spec() map[string]any {
	if math.IsInf(l.minX, 1) {
		l.minX, l.minY, l.maxX, l.maxY = 0, 0, 0, 0
	}
	dx, dy := -l.minX, -l.minY
	var data []any
	for _, set := range slices.Sorted(maps.Keys(l.rows)) {
		rows := l.rows[set]
		for _, r := range rows {
			m, _ := r.(map[string]any)
			for k, d := range map[string]float64{"x": dx, "x2": dx, "y": dy, "y2": dy} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round((v+d)*100) / 100
				}
			}
		}
		data = append(data, map[string]any{"name": set, "values": rows})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	color := map[string]any{"scale": "color", "field": "key"}
	var marks []any
	mark := func(name, typ, set string, style []any, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": name, "from": map[string]any{"data": set},
			"style": style, "encode": map[string]any{"update": enc},
		})
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = field("x"), field("y"), field("path")
		return enc
	}
	style := func(s ...string) []any {
		out := make([]any, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	}
	mark("lanes", "rule", "lanes", style(graph.StyleEdge), map[string]any{
		"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2"),
		"stroke": color, "strokeOpacity": value(0.45), "strokeWidth": value(1.25), "strokeDash": value([]any{3, 4}),
	})
	mark("links", "path", "links", style(graph.StyleEdge), path(map[string]any{
		"stroke": color, "strokeWidth": value(2.5),
	}))
	mark("commitBacks", "path", "commitBacks", style(graph.StyleEdgeLabelBox), path(map[string]any{"fillOpacity": value(1), "strokeOpacity": value(0)}))
	mark("commits", "path", "commits", style(graph.StyleNode), path(map[string]any{
		"fill": color, "stroke": color, "fillOpacity": value(1),
	}))
	mark("commitMarks", "path", "commitMarks", style(graph.StyleEdgeLabelBox), path(map[string]any{"fillOpacity": value(1), "strokeOpacity": value(0)}))
	mark("commitLabels", "text", "commitLabels", style(graph.StyleEdgeLabel, "zuzu.halo"), map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "align": field("align"), "baseline": field("baseline"),
		"angle": field("angle"), "fontSize": value(gitLabelFont.Size),
	})
	mark("tags", "path", "tags", style(graph.StyleCluster), path(map[string]any{"fillOpacity": value(1)}))
	mark("tagHoles", "path", "tagHoles", style(graph.StyleArrow), path(map[string]any{}))
	mark("tagLabels", "text", "tagLabels", style(graph.StyleNodeLabel), map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "align": value("center"), "baseline": value("middle"),
		"fontSize": value(gitLabelFont.Size),
	})
	mark("branchBoxes", "rect", "branchBoxes", style(graph.StyleNode), map[string]any{
		"x": field("x"), "x2": field("x2"), "y": field("y"), "y2": field("y2"),
		"fill": color, "stroke": color, "cornerRadius": value(4),
	})
	mark("branchLabels", "text", "branchLabels", style(graph.StyleNodeLabel), map[string]any{
		"x": field("x"), "y": field("y"), "text": field("lines"), "align": value("center"), "baseline": value("middle"),
		"fontSize": value(gitBranchFont.Size), "fontWeight": value(gitBranchFont.Weight), "lineHeight": value(lh(gitBranchFont)),
	})
	var domain []any
	for _, b := range l.branches {
		domain = append(domain, b.name)
	}
	pad := l.g.cfg.padding
	return map[string]any{
		"width":    math.Ceil(l.maxX - l.minX),
		"height":   math.Ceil(l.maxY - l.minY),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": domain, "range": "category"}},
		"marks":    marks,
	}
}
