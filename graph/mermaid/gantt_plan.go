package mermaid

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// gantt is a gantt chart of Mermaid, read and planned: each task with its
// start and its end.
type gantt struct {
	c *chart
	// loc is the time zone of the plan, and today the time that fills the
	// parts of a date that the source does not give.
	loc   *time.Location
	today time.Time

	title, desc        string
	dateFormat         string
	axisFormat         string
	tickInterval       string
	weekday            string
	topAxis, inclusive bool
	excludes, includes []string
	// lists are the statements excludes and includes, for the report.
	lists []ganttListStmt
	// weekend is the first day of the weekend: Saturday, or Friday.
	weekend time.Weekday

	tasks []*ganttTask
	byID  map[string]*ganttTask
	toks  []ganttToken
}

// ganttTask is a task of a gantt, as the source gives it and as the plan
// places it.
type ganttTask struct {
	name, section, id string
	line              int
	done, active      bool
	crit, milestone   bool
	vert              bool

	// The start is the end of the task before it, the latest end of the
	// tasks of after, or a date.
	prev      *ganttTask
	after     []string
	startText string
	// The end is the earliest start of the tasks of until, a date, or a
	// duration from the start.
	until   []string
	endText string

	start, end, render time.Time
	state              int
}

// The states of a task in the plan.
const (
	taskUnplanned = iota
	taskPlanning
	taskPlanned
)

// ganttTags are the tags that a task can start with.
var ganttTags = map[string]bool{"done": true, "active": true, "crit": true, "milestone": true, "vert": true}

// ganttTickRe matches a tickInterval, such as 1day or 2week.
var ganttTickRe = regexp.MustCompile(`^([1-9][0-9]*)(millisecond|second|minute|hour|day|week|month)$`)

// readGantt reads the statements of a gantt.
func readGantt(c *chart, today time.Time, loc *time.Location) (*gantt, error) {
	g := &gantt{c: c, loc: loc, today: today, dateFormat: "YYYY-MM-DD", weekend: time.Saturday, byID: map[string]*ganttTask{}}
	if err := g.config(); err != nil {
		return nil, err
	}
	section := ""
	auto := 0
	var prev *ganttTask
	lines := c.d.Lines
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		c.line = l.N
		key, rest := ganttStatement(l.Text)
		var err error
		switch key {
		case "dateformat":
			g.dateFormat = rest
		case "axisformat":
			g.axisFormat = rest
		case "tickinterval":
			g.tickInterval = rest
		case "inclusiveenddates":
			g.inclusive = true
		case "topaxis":
			g.topAxis = true
		case "excludes":
			g.excludes = append(g.excludes, ganttList(rest)...)
			g.lists = append(g.lists, ganttListStmt{key: key, text: rest, line: l.N})
		case "includes":
			g.includes = append(g.includes, ganttList(rest)...)
			g.lists = append(g.lists, ganttListStmt{key: key, text: rest, line: l.N})
		case "todaymarker":
			if !strings.EqualFold(rest, "off") {
				err = c.add("todayMarker needs the time of the render, so zuzu draws no marker")
			}
		case "weekday":
			g.weekday = strings.ToLower(rest)
		case "weekend":
			switch strings.ToLower(rest) {
			case "friday":
				g.weekend = time.Friday
			case "saturday":
				g.weekend = time.Saturday
			default:
				return nil, c.errorf("the weekend starts on friday or saturday, not %q", rest)
			}
		case "title":
			g.title = rest
		case "section":
			section = rest
		case "click":
			err = c.add("zuzu does not draw a click on a task")
		case "acctitle", "accdescr":
			text, n, ok := ganttAccText(lines, i)
			if !ok {
				return nil, c.errorf("the accessible description has no end")
			}
			i = n
			g.desc = strings.TrimSpace(g.desc + " " + text)
		default:
			name, data, ok := strings.Cut(l.Text, ":")
			if !ok {
				return nil, c.errorf("%q is not a statement or a task", l.Text)
			}
			t, terr := g.task(strings.TrimSpace(name), data, section, &auto)
			if terr != nil {
				return nil, terr
			}
			t.prev = prev
			prev = t
			g.tasks = append(g.tasks, t)
			if t.id != "" {
				if _, ok := g.byID[t.id]; ok {
					err = c.add("the id %s names more than one task, and zuzu uses the first", t.id)
				} else {
					g.byID[t.id] = t
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}
	c.line = 0
	return g, nil
}

// config reads the settings of a gantt from the front matter and the
// directives.
func (g *gantt) config() error {
	if m, ok := g.c.d.Config["displayMode"].(string); ok && m != "" {
		if err := g.c.add("zuzu does not draw the display mode %s of a gantt", m); err != nil {
			return err
		}
	}
	cfg, _ := g.c.d.Config["gantt"].(map[string]any)
	for _, k := range slices.Sorted(maps.Keys(cfg)) {
		v := cfg[k]
		switch k {
		case "axisFormat":
			g.axisFormat, _ = v.(string)
		case "tickInterval":
			g.tickInterval, _ = v.(string)
		case "topAxis":
			g.topAxis, _ = v.(bool)
		case "weekday":
			s, _ := v.(string)
			g.weekday = strings.ToLower(s)
		case "displayMode":
			if s, _ := v.(string); s != "" {
				if err := g.c.add("zuzu does not draw the display mode %s of a gantt", s); err != nil {
					return err
				}
			}
		default:
			if err := g.c.add("zuzu does not read the setting gantt.%s", k); err != nil {
				return err
			}
		}
	}
	return nil
}

// ganttStatement splits a line into its keyword, in lower case, and the rest.
// The keyword is empty for a line that is a task.
func ganttStatement(s string) (string, string) {
	for _, k := range []string{"dateFormat", "axisFormat", "tickInterval", "inclusiveEndDates", "topAxis",
		"excludes", "includes", "todayMarker", "weekday", "weekend", "title", "section", "click"} {
		if len(s) < len(k) || !strings.EqualFold(s[:len(k)], k) {
			continue
		}
		rest := s[len(k):]
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
			return strings.ToLower(k), strings.TrimSpace(rest)
		}
	}
	for _, k := range []string{"accTitle", "accDescr"} {
		if len(s) < len(k) || !strings.EqualFold(s[:len(k)], k) {
			continue
		}
		rest := strings.TrimSpace(s[len(k):])
		if strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "{") {
			return strings.ToLower(k), rest
		}
	}
	return "", s
}

// ganttAccText reads the text of accTitle or accDescr at the line i: one line
// after a colon, or the lines in braces. It returns the index of the last
// line it read.
func ganttAccText(lines []Line, i int) (string, int, bool) {
	_, rest := ganttStatement(lines[i].Text)
	if t, ok := strings.CutPrefix(rest, ":"); ok {
		return strings.TrimSpace(t), i, true
	}
	rest = strings.TrimPrefix(rest, "{")
	var parts []string
	for {
		if before, _, ok := strings.Cut(rest, "}"); ok {
			parts = append(parts, before)
			return strings.Join(strings.Fields(strings.Join(parts, " ")), " "), i, true
		}
		parts = append(parts, rest)
		i++
		if i >= len(lines) {
			return "", i, false
		}
		rest = lines[i].Text
	}
}

// ganttListStmt is a statement excludes or includes: its keyword, its list
// as the source gives it, and its line.
type ganttListStmt struct {
	key, text string
	line      int
}

// ganttList splits the list of excludes or includes at commas and spaces.
func ganttList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

// task reads the data of a task: its tags first, then one to three items,
// the id, the start and the end.
func (g *gantt) task(name, data, section string, auto *int) (*ganttTask, error) {
	t := &ganttTask{name: name, section: section, line: g.c.line}
	items := strings.Split(data, ",")
	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}
	for len(items) > 0 && ganttTags[items[0]] {
		switch items[0] {
		case "done":
			t.done = true
		case "active":
			t.active = true
		case "crit":
			t.crit = true
		case "milestone":
			t.milestone = true
		case "vert":
			t.vert = true
		}
		items = items[1:]
	}
	if len(items) == 3 {
		t.id = items[0]
		items = items[1:]
	} else {
		*auto++
		t.id = fmt.Sprintf("task%d", *auto)
	}
	switch len(items) {
	case 2:
		if ids, ok := keyword(items[0], "after"); ok {
			t.after = strings.Fields(ids)
		} else {
			t.startText = items[0]
		}
	case 1:
	default:
		return nil, g.c.errorf("the task %q has %d items after its tags, and it takes one to three", name, len(items))
	}
	end := items[len(items)-1]
	if ids, ok := keyword(end, "until"); ok {
		t.until = strings.Fields(ids)
	} else {
		t.endText = end
	}
	if t.after == nil && t.startText == "" && len(g.tasks) == 0 {
		return nil, g.c.errorf("the first task %q has no start", name)
	}
	return t, nil
}

// plan places each task: its start, its end, and the end that it draws.
func (g *gantt) plan() error {
	g.toks = ganttFormat(g.dateFormat)
	if err := g.checkLists(); err != nil {
		return err
	}
	for _, t := range g.tasks {
		if err := g.place(t); err != nil {
			return err
		}
	}
	return nil
}

// place places one task, and the tasks it needs first.
func (g *gantt) place(t *ganttTask) error {
	switch t.state {
	case taskPlanned:
		return nil
	case taskPlanning:
		g.c.line = t.line
		return g.c.errorf("the task %q is in a cycle of after and until", t.name)
	}
	t.state = taskPlanning
	var err error
	if t.start, err = g.startOf(t); err != nil {
		return err
	}
	var manual bool
	if t.end, manual, err = g.endOf(t); err != nil {
		return err
	}
	t.render = t.end
	if !manual && len(g.excludes) > 0 {
		if err := g.skipExcluded(t); err != nil {
			return err
		}
	}
	t.state = taskPlanned
	return nil
}

// startOf returns the start of a task.
func (g *gantt) startOf(t *ganttTask) (time.Time, error) {
	switch {
	case t.after != nil:
		var start time.Time
		for _, id := range t.after {
			o, ok := g.byID[id]
			if !ok {
				continue
			}
			if err := g.place(o); err != nil {
				return time.Time{}, err
			}
			if start.IsZero() || o.end.After(start) {
				start = o.end
			}
		}
		if start.IsZero() {
			g.c.line = t.line
			return g.day(), g.c.add("after names no task: %s, so the task %q starts on the day of the render", strings.Join(t.after, " "), t.name)
		}
		return start, nil
	case t.startText != "":
		if s, ok := ganttParse(t.startText, g.toks, g.today, g.loc); ok {
			return s, nil
		}
		if s, ok := ganttLoose(t.startText, g.loc); ok {
			return s, nil
		}
		g.c.line = t.line
		return time.Time{}, g.c.errorf("the start %q of the task %q is not a date of the format %q", t.startText, t.name, g.dateFormat)
	}
	if err := g.place(t.prev); err != nil {
		return time.Time{}, err
	}
	return t.prev.end, nil
}

// endOf returns the end of a task, and whether the source gave it as a
// date.
func (g *gantt) endOf(t *ganttTask) (time.Time, bool, error) {
	if t.until != nil {
		var end time.Time
		for _, id := range t.until {
			o, ok := g.byID[id]
			if !ok {
				continue
			}
			if err := g.place(o); err != nil {
				return time.Time{}, false, err
			}
			if end.IsZero() || o.start.Before(end) {
				end = o.start
			}
		}
		if end.IsZero() {
			g.c.line = t.line
			return g.day(), false, g.c.add("until names no task: %s, so the task %q ends on the day of the render", strings.Join(t.until, " "), t.name)
		}
		return end, false, nil
	}
	if e, ok := ganttParse(t.endText, g.toks, g.today, g.loc); ok {
		if g.inclusive {
			e = e.AddDate(0, 0, 1)
		}
		return e, true, nil
	}
	if d, ok := ganttParseDuration(t.endText); ok {
		return d.add(t.start), false, nil
	}
	g.c.line = t.line
	return t.start, false, g.c.add("the end %q of the task %q is not a date of the format %q or a duration, so the task has no length", t.endText, t.name, g.dateFormat)
}

// day returns the start of the day of today.
func (g *gantt) day() time.Time {
	y, m, d := g.today.In(g.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, g.loc)
}

// ganttMaxSteps is the most days that the walk over the excluded days takes for
// one task.
const ganttMaxSteps = 200000

// skipExcluded moves the end of a task later by one day for each excluded
// day in it. It walks from the start one day at a time, up to the end. Each
// step that lands on an excluded day moves the end one day later. The task
// draws up to the end, less the excluded days just before the last step.
func (g *gantt) skipExcluded(t *ganttTask) error {
	d := t.start
	run, before := 0, 0
	stepped := false
	for range ganttMaxSteps {
		next := d.AddDate(0, 0, 1)
		if next.After(t.end) {
			if stepped {
				t.render = t.end.AddDate(0, 0, -before)
			}
			return nil
		}
		d, stepped = next, true
		if g.excluded(d) {
			t.end = t.end.AddDate(0, 0, 1)
			run++
			continue
		}
		before, run = run, 0
	}
	g.c.line = t.line
	return g.c.errorf("the excludes leave too few days for the task %q", t.name)
}

// excluded reports whether the day of a time is excluded: a day of the
// weekend, a named day of the week, or a date. A date in includes is never
// excluded.
func (g *gantt) excluded(t time.Time) bool {
	iso := t.Format("2006-01-02")
	local := ganttWrite(t, g.toks, 0)
	for _, s := range g.includes {
		if s == iso || s == local {
			return false
		}
	}
	wd := t.Weekday()
	for _, s := range g.excludes {
		switch {
		case s == iso || s == local:
			return true
		case strings.EqualFold(s, "weekends"):
			if wd == g.weekend || wd == (g.weekend+1)%7 {
				return true
			}
		case strings.EqualFold(s, ganttDays[wd]):
			return true
		}
	}
	return false
}

// checkLists reports the items of each statement excludes and includes that
// zuzu does not read, in one issue for the statement: an item that is not a
// date of the format, a day of the week or weekends. Mermaid ignores such an
// item.
func (g *gantt) checkLists() error {
	read := func(s string, days bool) bool {
		if days && strings.EqualFold(s, "weekends") {
			return true
		}
		for _, n := range ganttDays {
			if days && strings.EqualFold(s, n) {
				return true
			}
		}
		if _, ok := ganttParse(s, g.toks, g.today, g.loc); ok {
			return true
		}
		_, ok := ganttParse(s, ganttFormat("YYYY-MM-DD"), g.today, g.loc)
		return ok
	}
	defer func() { g.c.line = 0 }()
	for _, st := range g.lists {
		days := st.key == "excludes"
		items := ganttList(st.text)
		var unread []string
		for _, s := range items {
			if !read(s, days) {
				unread = append(unread, strconv.Quote(s))
			}
		}
		if len(unread) == 0 {
			continue
		}
		g.c.line = st.line
		what, verb := "dates", "include"
		if days {
			what, verb = "dates, days of the week or weekends", "exclude"
		}
		var err error
		if len(unread) == len(items) {
			err = g.c.add("%s %q holds no %s, so it does not %s a day", st.key, st.text, what, verb)
		} else {
			err = g.c.add("the items %s of %s are not %s, so they do not %s a day", strings.Join(unread, ", "), st.key, what, verb)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
