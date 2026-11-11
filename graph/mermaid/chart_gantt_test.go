package mermaid

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// ganttNow is the time of the clock of the oracle, test/oracle/gantt.mjs.
var ganttNow = time.Date(2024, 3, 15, 10, 20, 30, 400e6, time.UTC)

// oracleTask is a task as the oracle writes it, with times in
// milliseconds since the epoch.
type oracleTask struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Section   string   `json:"section"`
	Start     *int64   `json:"start"`
	End       *int64   `json:"end"`
	RenderEnd *int64   `json:"renderEnd"`
	Milestone bool     `json:"milestone"`
	Vert      bool     `json:"vert"`
	Tags      []string `json:"tags"`
}

// ganttOf reads a source and plans its gantt, in the loose mode.
func ganttOf(t *testing.T, src string, strict bool) (*gantt, *chart, error) {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	c := &chart{d: d, issues: &graph.Issues{Strict: strict}}
	g, err := planGantt(c, ganttNow)
	return g, c, err
}

func TestGanttOracle(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/gantt/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle map[string][]oracleTask
	if err := json.Unmarshal(b, &oracle); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("testdata/gantt/*.mmd")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(oracle) {
		t.Fatalf("%d fixtures and %d in the oracle: run test/oracle/gantt.mjs", len(files), len(oracle))
	}
	ms := func(tm time.Time) int64 { return tm.UnixMilli() }
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			exp, ok := oracle[filepath.Base(file)]
			if !ok {
				t.Fatal("not in the oracle")
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			g, c, err := ganttOf(t, string(src), false)
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if len(c.issues.Report) != 0 {
				t.Errorf("report: %v", c.issues.Report)
			}
			if len(g.tasks) != len(exp) {
				t.Fatalf("expected %d tasks, got %d", len(exp), len(g.tasks))
			}
			for i, e := range exp {
				task := g.tasks[i]
				var tags []string
				for _, k := range []struct {
					name string
					on   bool
				}{{"active", task.active}, {"done", task.done}, {"crit", task.crit}} {
					if k.on {
						tags = append(tags, k.name)
					}
				}
				if task.id != e.ID || task.name != e.Name || task.section != e.Section ||
					task.milestone != e.Milestone || task.vert != e.Vert || !slices.Equal(tags, e.Tags) {
					t.Errorf("task %d: expected %s %q in %q %v, got %s %q in %q %v",
						i, e.ID, e.Name, e.Section, e.Tags, task.id, task.name, task.section, tags)
				}
				if e.Start == nil || e.End == nil {
					t.Fatalf("task %d: the oracle has no time", i)
				}
				render := *e.End
				if e.RenderEnd != nil {
					render = *e.RenderEnd
				}
				if ms(task.start) != *e.Start || ms(task.end) != *e.End || ms(task.render) != render {
					t.Errorf("task %d %q: expected %s to %s, drawn to %s, got %s to %s, drawn to %s", i, e.Name,
						show(*e.Start), show(*e.End), show(render), show(ms(task.start)), show(ms(task.end)), show(ms(task.render)))
				}
			}
		})
	}
}

// show writes milliseconds since the epoch as a time in UTC.
func show(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000")
}

func TestGanttChart(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/gantt/16-tags.mmd")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	b, lite, report, err := Chart(d, true)
	if err != nil {
		t.Fatalf("chart: %v", err)
	}
	if !lite || len(report) != 0 {
		t.Errorf("expected Vega-Lite and no report, got %v and %v", lite, report)
	}
	s := string(b)
	for _, want := range []string{`"text":"Tags in sections"`, `"2024-02-01T00:00:00.000"`, `"shape":"diamond"`, `"stroke":"` + ganttCrit + `"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the spec has no %s", want)
		}
	}
}

func TestGanttCycle(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"gantt\ndateFormat YYYY-MM-DD\nx : x1, after y1, 1d\ny : y1, after x1, 2d",
		"gantt\ndateFormat YYYY-MM-DD\nx : x1, 2024-01-01, until y1\ny : y1, after x1, 2d",
		"gantt\ndateFormat YYYY-MM-DD\nx : x1, after x1, 1d",
	} {
		_, _, err := ganttOf(t, src, false)
		if !errors.Is(err, ErrSyntax) || !strings.Contains(err.Error(), "cycle") {
			t.Errorf("%q: expected an error of a cycle, got %v", src, err)
		}
	}
}

func TestGanttErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, exp string
	}{
		{"gantt\na : 2024-01-01", "has no start"},
		{"gantt\na : not a date, 1d", "is not a date"},
		{"gantt\nweekend sunday\na : 2024-01-01, 1d", "the weekend starts"},
		{"gantt\njust words", "is not a statement"},
		{"gantt\na : a, 2024-01-01, 1d, 2d", "takes one to three"},
		{"gantt\ndateFormat YYYY-MM-DD\naccDescr {\nnever ends", "has no end"},
	}
	for _, test := range tests {
		_, _, err := ganttOf(t, test.src, false)
		if !errors.Is(err, ErrSyntax) || !strings.Contains(err.Error(), test.exp) {
			t.Errorf("%q: expected an error with %q, got %v", test.src, test.exp, err)
		}
	}
}

func TestGanttIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, exp string
	}{
		{"gantt\ndateFormat YYYY-MM-DD\na : a1, 2024-01-01, 1d\nclick a1 href \"https://example.com\"", "click"},
		{"gantt\ndateFormat YYYY-MM-DD\na : 2024-01-01, 3dX", `not a date of the format "YYYY-MM-DD" or a duration`},
		{"gantt\ndateFormat YYYY-MM-DD\na : after zz, 1d", "after names no task"},
		{"gantt\ndateFormat YYYY-MM-DD\na : 2024-01-01, until zz", "until names no task"},
		{"gantt\ntodayMarker stroke-width:5px\na : 2024-01-01, 1d", "todayMarker"},
		{"gantt\nexcludes weekdays\na : 2024-01-01, 1d", "excludes \"weekdays\""},
		{"gantt\nincludes monday\na : 2024-01-01, 1d", "includes \"monday\""},
		{"---\ndisplayMode: compact\n---\ngantt\na : 2024-01-01, 1d", "display mode compact"},
		{"---\nconfig:\n  gantt:\n    barHeight: 30\n---\ngantt\na : 2024-01-01, 1d", "gantt.barHeight"},
		{"gantt\na : a1, 2024-01-01, 1d\nb : a1, 2024-01-01, 1d", "more than one task"},
	}
	for _, test := range tests {
		g, c, err := ganttOf(t, test.src, false)
		if err != nil {
			t.Fatalf("%q: %v", test.src, err)
		}
		if !hasIssue(c.issues.Report, test.exp) {
			t.Errorf("%q: expected an issue with %q, got %v", test.src, test.exp, c.issues.Report)
		}
		if _, err := g.spec(); err != nil {
			t.Errorf("%q: spec: %v", test.src, err)
		}
		if _, _, err := ganttOf(t, test.src, true); err == nil {
			t.Errorf("%q: expected an error in the strict mode", test.src)
		}
	}
	// A tickInterval that does not read is an issue of the spec.
	g, c, err := ganttOf(t, "gantt\ntickInterval 1decade\na : 2024-01-01, 1d", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.spec(); err != nil || !hasIssue(c.issues.Report, "tickInterval") {
		t.Errorf("expected an issue of tickInterval, got %v and %v", err, c.issues.Report)
	}
}

// TestGanttColonFormat follows Mermaid 12.3.0 and mmdc: the value of
// dateFormat is all the text after the keyword, so the colon is part of the
// format. A start date reads as the Date of JavaScript reads it, and an end
// date does not read, so its task has no length.
func TestGanttColonFormat(t *testing.T) {
	t.Parallel()
	src := "gantt\n dateFormat                :YYYY-MM-DD\n" +
		" excludes                  :excludes the named days\n" +
		" Completed task            :done,    des1, 2014-01-06,2014-01-08\n" +
		" Active task               :active,  des2, 2014-01-09, 3d\n" +
		" Describe                  :a1, after des1, 3d"
	g, c, err := ganttOf(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.dateFormat != ":YYYY-MM-DD" {
		t.Errorf("the format %q", g.dateFormat)
	}
	day := func(tm time.Time) string { return tm.UTC().Format(time.DateOnly) }
	exp := []string{"2014-01-06 2014-01-06", "2014-01-09 2014-01-12", "2014-01-06 2014-01-09"}
	for i, task := range g.tasks {
		if got := day(task.start) + " " + day(task.end); got != exp[i] {
			t.Errorf("task %q: expected %s, got %s", task.name, exp[i], got)
		}
	}
	if len(c.issues.Report) != 2 {
		t.Fatalf("expected 2 issues, got %v", c.issues.Report)
	}
	for i, want := range []string{
		`line 3: excludes ":excludes the named days" holds no dates, days of the week or weekends`,
		`line 4: the end "2014-01-08" of the task "Completed task" is not a date of the format ":YYYY-MM-DD"`,
	} {
		if got := c.issues.Report[i].Error(); !strings.Contains(got, want) {
			t.Errorf("issue %d: expected %q, got %q", i, want, got)
		}
	}
}

func TestGanttListIssues(t *testing.T) {
	t.Parallel()
	src := "gantt\ndateFormat YYYY-MM-DD\nexcludes weekends, sunday, 2024-01-03, nope, never\n" +
		"excludes saturday\nincludes 2024-01-06 friday\na : 2024-01-01, 1d"
	_, c, err := ganttOf(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`line 3: the items "nope", "never" of excludes are not dates, days of the week or weekends`,
		`line 5: the items "friday" of includes are not dates`,
	}
	if len(c.issues.Report) != 2 {
		t.Fatalf("expected 2 issues, got %v", c.issues.Report)
	}
	for i, w := range want {
		if got := c.issues.Report[i].Error(); !strings.Contains(got, w) {
			t.Errorf("issue %d: expected %q, got %q", i, w, got)
		}
	}
}

// hasIssue reports whether a report holds an issue with a text.
func hasIssue(r vega.Report, s string) bool {
	for _, i := range r {
		if strings.Contains(i.Message, s) {
			return true
		}
	}
	return false
}

func TestGanttStatements(t *testing.T) {
	t.Parallel()
	src := "gantt\nTITLE  Big plan\nDATEFORMAT YYYY\naxisFormat %Y\ntopAxis\n" +
		"accTitle: The plan\naccDescr {\n  It has\n  two lines\n}\nSECTION One\na : 2020, 1y"
	g, c, err := ganttOf(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.title != "Big plan" || g.dateFormat != "YYYY" || g.axisFormat != "%Y" || !g.topAxis ||
		g.desc != "The plan It has two lines" || len(c.issues.Report) != 0 {
		t.Errorf("got %+v and %v", g, c.issues.Report)
	}
	if s := g.tasks[0].start.Format(time.DateOnly); s != "2020-01-01" || g.tasks[0].section != "One" {
		t.Errorf("got %s in %q", s, g.tasks[0].section)
	}
}

func TestGanttParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		layout, in, exp string
	}{
		{"YYYY-MM-DD", "2024-01-05", "2024-01-05T00:00:00.000"},
		{"YYYY-MM-DD", "2024-1-5", ""},
		{"YYYY-MM-DD", "2024-02-30", ""},
		{"D", "3", "2024-03-03T00:00:00.000"},
		{"D", "03", ""},
		{"HH:mm", "17:49", "2024-03-15T17:49:00.000"},
		{"HH:mm", "25:00", ""},
		{"YYYY", "2020", "2020-01-01T00:00:00.000"},
		{"MM", "05", "2024-05-01T00:00:00.000"},
		{"YY-MM-DD", "68-01-05", "2068-01-05T00:00:00.000"},
		{"YY-MM-DD", "70-01-05", "1970-01-05T00:00:00.000"},
		{"h:mm a", "2:30 pm", "2024-03-15T14:30:00.000"},
		{"h:mm a", "2:30 PM", ""},
		{"hh A", "12 AM", "2024-03-15T00:00:00.000"},
		{"Do MMMM YYYY", "22nd March 2024", "2024-03-22T00:00:00.000"},
		{"Do MMMM YYYY", "22th March 2024", ""},
		{"[on] YYYY", "on 2024", "2024-01-01T00:00:00.000"},
		{"YYYY-MM-DDTHH:mm:ssZ", "2024-01-05T10:00:00+05:00", "2024-01-05T05:00:00.000"},
		{"YYYY-MM-DD HH:mm ZZ", "2024-01-05 10:00 -0130", "2024-01-05T11:30:00.000"},
		{"x", "1704412800000", "2024-01-05T00:00:00.000"},
		{"X", "71", "1970-01-01T00:01:11.000"},
		{"YYYY-MM-DD HH:mm:ss.SSS", "2024-01-05 10:00:00.123", "2024-01-05T10:00:00.123"},
		{"Q YYYY", "3 2024", "2024-07-01T00:00:00.000"},
		{"dddd YYYY-MM-DD", "Friday 2024-01-05", "2024-01-05T00:00:00.000"},
		{"dddd YYYY-MM-DD", "Monday 2024-01-05", ""},
	}
	for _, test := range tests {
		tm, ok := ganttParse(test.in, ganttFormat(test.layout), ganttNow, time.UTC)
		got := ""
		if ok {
			got = tm.Format(ganttLayout)
		}
		if got != test.exp {
			t.Errorf("%s %q: expected %q, got %q", test.layout, test.in, test.exp, got)
		}
	}
}
