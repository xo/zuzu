package mermaid

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/zuzu/text"
)

// textRun is a part of a line of a label in one font: plain, bold or
// italic. x is its offset from the start of its line, and w its width.
// space says that a space comes before it in its line.
type textRun struct {
	text         string
	bold, italic bool
	space        bool
	x, w         float64
}

// textLine is a line of a label: its runs and its width.
type textLine struct {
	runs []textRun
	w    float64
}

// String returns the text of a line, with a space between its words.
func (l textLine) String() string {
	var b strings.Builder
	for _, r := range l.runs {
		if r.space {
			b.WriteString(" ")
		}
		b.WriteString(r.text)
	}
	return b.String()
}

// boldWeight is the weight of a bold run. The bundled IBM Plex Sans has no
// weight above 600 (D16).
const boldWeight = 600

// lineBreak matches the tag br of a label.
var lineBreak = regexp.MustCompile(`(?i)<br\s*/?>`)

// labelText reads the text of a label of a chart: br as a new line, and
// the entities, such as `#quot;` and `#35;`. It keeps the marks of markdown.
func labelText(s string) string {
	s = lineBreak.ReplaceAllString(s, "\n")
	s = entities.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		switch name {
		case "quot":
			return `"`
		case "amp":
			return "&"
		case "lt":
			return "<"
		case "gt":
			return ">"
		}
		if n, err := strconv.ParseInt(name, 10, 32); err == nil && n >= 0 {
			return string(rune(n))
		}
		return m
	})
	return strings.TrimSpace(s)
}

// markdownRuns splits a label at the marks of markdown: two stars or two
// underscores around bold text, and one star or one underscore around
// italic text. A mark with no mark to close it is text, and so is an
// underscore inside a word. A new line is a run of its own, "\n". Text
// between two backticks is code, which Mermaid draws as it is written, with
// its backticks and its marks.
func markdownRuns(s string) []textRun {
	var out []textRun
	bold, italic := false, false
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, textRun{text: b.String(), bold: bold, italic: italic})
			b.Reset()
		}
	}
	word := func(i int) bool {
		if i < 0 || i >= len(s) {
			return false
		}
		c := s[i]
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\n':
			flush()
			out = append(out, textRun{text: "\n"})
			i++
		case s[i] == '`' && strings.Contains(s[i+1:], "`"):
			j := i + 1 + strings.Index(s[i+1:], "`")
			for _, c := range s[i : j+1] {
				if c == '\n' {
					flush()
					out = append(out, textRun{text: "\n"})
					continue
				}
				b.WriteRune(c)
			}
			i = j + 1
		case strings.HasPrefix(s[i:], "**") || strings.HasPrefix(s[i:], "__"):
			m := s[i : i+2]
			if bold || strings.Contains(s[i+2:], m) {
				flush()
				bold = !bold
			} else {
				b.WriteString(m)
			}
			i += 2
		case s[i] == '*' || s[i] == '_':
			inside := s[i] == '_' && word(i-1) && word(i+1)
			if !inside && (italic || strings.Contains(s[i+1:], s[i:i+1])) {
				flush()
				italic = !italic
			} else {
				b.WriteByte(s[i])
			}
			i++
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	flush()
	return out
}

// textFont returns the font of a run of a label of a size and a weight.
func textFont(size float64, weight int, r textRun) text.Font {
	if r.bold {
		weight = max(weight, boldWeight)
	}
	return text.Font{Family: text.Family, Size: size, Weight: weight, Italic: r.italic}
}

// wrapRuns lays the runs of a label out in lines no wider than limit, at
// the spaces between words, or with no limit for 0. A word wider than the
// limit has a line of its own. Each run of a line has its offset and its
// width, measured with the bundled font, as the render measures text.
func wrapRuns(runs []textRun, size float64, weight int, limit float64, m text.Measurer) []textLine {
	// piece is a part of a word in one font, and whether a space comes
	// before it.
	type piece struct {
		run   textRun
		space bool
	}
	var words [][]piece
	var hard []bool
	space, br := false, false
	for _, r := range runs {
		if r.text == "\n" {
			br, space = true, false
			continue
		}
		parts := strings.Split(r.text, " ")
		for i, p := range parts {
			if i > 0 {
				space = true
			}
			if p == "" {
				continue
			}
			pc := textRun{text: p, bold: r.bold, italic: r.italic}
			pc.w = m.Width(textFont(size, weight, pc), p)
			if space || br || len(words) == 0 {
				words = append(words, []piece{{pc, space}})
				hard = append(hard, br)
			} else {
				words[len(words)-1] = append(words[len(words)-1], piece{pc, false})
			}
			space, br = false, false
		}
	}
	spaceW := m.Width(text.Font{Family: text.Family, Size: size, Weight: weight}, " ")
	var lines []textLine
	var cur textLine
	push := func() {
		lines = append(lines, cur)
		cur = textLine{}
	}
	for i, w := range words {
		ww := 0.0
		for _, p := range w {
			ww += p.run.w
		}
		if len(cur.runs) > 0 && (hard[i] || limit > 0 && cur.w+spaceW+ww > limit) {
			push()
		}
		x := cur.w
		if len(cur.runs) > 0 {
			x += spaceW
		}
		for j, p := range w {
			r := p.run
			r.x = x
			x += r.w
			if n := len(cur.runs); n > 0 && cur.runs[n-1].bold == r.bold && cur.runs[n-1].italic == r.italic {
				last := &cur.runs[n-1]
				if j == 0 {
					last.text += " "
				}
				last.text += r.text
				last.w = x - last.x
				continue
			}
			r.space = j == 0 && len(cur.runs) > 0
			cur.runs = append(cur.runs, r)
		}
		cur.w = x
	}
	if len(cur.runs) > 0 || len(lines) == 0 {
		push()
	}
	return lines
}

// breakRuns breaks each word of runs that is wider than limit into parts no
// wider than limit, with a new line between them, as Mermaid breaks a word
// of a label that it draws in SVG. A part holds one character at least.
func breakRuns(runs []textRun, size float64, weight int, limit float64, m text.Measurer) []textRun {
	var out []textRun
	for _, r := range runs {
		if r.text == "\n" {
			out = append(out, r)
			continue
		}
		font := textFont(size, weight, r)
		words := strings.Split(r.text, " ")
		for i, w := range words {
			if m.Width(font, w) <= limit {
				continue
			}
			var parts []string
			start := 0
			for j, c := range w {
				if j > start && m.Width(font, w[start:j+utf8.RuneLen(c)]) > limit {
					parts = append(parts, w[start:j])
					start = j
				}
			}
			words[i] = strings.Join(append(parts, w[start:]), "\n")
		}
		for i, l := range strings.Split(strings.Join(words, " "), "\n") {
			if i > 0 {
				out = append(out, textRun{text: "\n"})
			}
			out = append(out, textRun{text: l, bold: r.bold, italic: r.italic})
		}
	}
	return out
}

// plainLines lays a label of plain text out in lines no wider than limit,
// and returns the lines and the width of the widest.
func plainLines(s string, size float64, weight int, limit float64, m text.Measurer) ([]string, float64) {
	var runs []textRun
	for i, l := range strings.Split(s, "\n") {
		if i > 0 {
			runs = append(runs, textRun{text: "\n"})
		}
		runs = append(runs, textRun{text: l})
	}
	var out []string
	w := 0.0
	for _, l := range wrapRuns(runs, size, weight, limit, m) {
		out = append(out, l.String())
		w = max(w, l.w)
	}
	return out, w
}
