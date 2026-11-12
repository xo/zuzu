package mermaid

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// pkField is a field of a packet: its first and its last bit, and its
// label.
type pkField struct {
	start, end int
	label      string
}

// pkConfig is the config of a packet, with the defaults of Mermaid 12.3.
type pkConfig struct {
	rowHeight, bitWidth float64
	bitsPerRow          int
	showBits            bool
	descending          bool
	paddingX, paddingY  float64
	// theme holds the theme variables of a packet that the source gives,
	// such as blockFillColor, by their names.
	theme map[string]any
}

// The theme variables of a packet that zuzu reads, in
// themeVariables.packet: the colors as texts, and the sizes as numbers.
var (
	pkThemeColors = []string{"startByteColor", "endByteColor", "labelColor", "titleColor", "blockStrokeColor", "blockFillColor"}
	pkThemeSizes  = []string{"byteFontSize", "labelFontSize", "titleFontSize", "blockStrokeWidth"}
)

// The statements of a packet, as the grammar of Mermaid 12.3 reads them.
var (
	// pkBlockRE matches a field: a bit, a range of bits or a number of
	// bits after +, a colon and a string in double or single quotes.
	pkBlockRE = regexp.MustCompile(`^(?:(\d+)(?:[\t ]*-[\t ]*(\d+))?|\+[\t ]*(\d+))[\t ]*:[\t ]*("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')$`)
	// pkTitleRE matches a title: the word title, alone or with a text
	// after a space.
	pkTitleRE = regexp.MustCompile(`^title(?:[\t ](.*))?$`)
	// pkIntRE matches a number as Mermaid reads it, with no 0 before it.
	pkIntRE = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)
)

// pkMaxRows is the number of rows at which Mermaid stops adding the rows
// of a packet.
const pkMaxRows = 10000

// packetChart compiles a packet: its fields as boxes on rows of bits, each
// with its label, and the numbers of its first and its last bit over it,
// into a Vega spec (D31). The compiler lays the rows out in Go, as Mermaid
// 12.3 does.
func packetChart(c *chart) (map[string]any, bool, error) {
	cfg, err := packetSettings(c)
	if err != nil {
		return nil, false, err
	}
	title, fields, err := packetParse(c)
	if err != nil {
		return nil, false, err
	}
	if title == "" {
		title = c.d.Title
	}
	rows := packetRows(fields, cfg.bitsPerRow)
	if len(rows) == 0 {
		return emptyChart(title, ""), false, nil
	}
	spec := packetSpec(rows, cfg)
	if title != "" {
		t := map[string]any{"text": title, "anchor": "start"}
		if v, ok := cfg.theme["titleColor"]; ok {
			t["color"] = v
		}
		if v, ok := cfg.theme["titleFontSize"]; ok {
			t["fontSize"] = v
		}
		spec["title"] = t
	}
	return spec, false, nil
}

// packetSettings reads the section packet of the config, and the theme
// variables of a packet (D32). A value that is not a positive number, or
// not true or false, is an issue, and the setting keeps its default. A
// color that is not a text, a size that is not a number, and each other
// theme variable are issues.
func packetSettings(c *chart) (pkConfig, error) {
	cfg := pkConfig{rowHeight: 32, bitWidth: 32, bitsPerRow: 32, showBits: true, paddingX: 5, paddingY: 5, theme: map[string]any{}}
	read := []string{"packet.rowHeight", "packet.bitWidth", "packet.bitsPerRow", "packet.showBits",
		"packet.bitOrder", "packet.paddingX", "packet.paddingY"}
	tv, _ := c.d.Config["themeVariables"].(map[string]any)
	theme, _ := tv["packet"].(map[string]any)
	for _, k := range pkThemeColors {
		if v, ok := theme[k].(string); ok && v != "" {
			cfg.theme[k] = v
			read = append(read, "themeVariables.packet."+k)
		}
	}
	for _, k := range pkThemeSizes {
		if n, ok := configNumber(theme, k); ok && n > 0 {
			cfg.theme[k] = n
			read = append(read, "themeVariables.packet."+k)
		}
	}
	if err := c.unreadSettings(read...); err != nil {
		return cfg, err
	}
	sec, _ := c.d.Config["packet"].(map[string]any)
	for _, k := range []string{"rowHeight", "bitWidth", "bitsPerRow", "paddingX", "paddingY"} {
		v, ok := sec[k]
		if !ok {
			continue
		}
		n, isNum := configNumber(sec, k)
		switch {
		case !isNum || n < 0 || n == 0 && k != "paddingX" && k != "paddingY",
			k == "bitsPerRow" && n != math.Trunc(n):
			if err := c.add("the setting packet.%s %v, which is not a positive number", k, v); err != nil {
				return cfg, err
			}
			continue
		}
		switch k {
		case "rowHeight":
			cfg.rowHeight = n
		case "bitWidth":
			cfg.bitWidth = n
		case "bitsPerRow":
			cfg.bitsPerRow = int(n)
		case "paddingX":
			cfg.paddingX = n
		case "paddingY":
			cfg.paddingY = n
		}
	}
	if v, ok := sec["showBits"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			if err := c.add("the setting packet.showBits %v, which is not true or false", v); err != nil {
				return cfg, err
			}
		} else {
			cfg.showBits = b
		}
	}
	if v, ok := sec["bitOrder"]; ok {
		switch v {
		case "ascending":
		case "descending":
			cfg.descending = true
		default:
			if err := c.add("the setting packet.bitOrder %v, which is not ascending or descending", v); err != nil {
				return cfg, err
			}
		}
	}
	// Mermaid leaves 10 more pixels over each row for the numbers of the
	// bits.
	if cfg.showBits {
		cfg.paddingY += 10
	}
	return cfg, nil
}

// packetParse reads the statements of a packet: a title, accTitle,
// accDescr and the fields, one on each line. Each field starts at the bit
// after the field before it, as Mermaid requires. The last title that is
// not empty is the title, as in Mermaid.
func packetParse(c *chart) (string, []pkField, error) {
	lines := make([]Line, 0, len(c.d.Lines)+1)
	// Statements can follow the keyword on its line, as in Mermaid.
	if h := strings.TrimSpace(c.d.Header); h != "" {
		lines = append(lines, Line{Text: h, N: c.d.N})
	}
	lines = append(lines, c.d.Lines...)
	var title string
	var fields []pkField
	last := -1
	for i := 0; i < len(lines); i++ {
		c.line = lines[i].N
		n, ok, err := c.accessibility(lines, i)
		if err != nil {
			return "", nil, err
		}
		if ok {
			i = n
			continue
		}
		s := lines[i].Text
		if m := pkTitleRE.FindStringSubmatch(s); m != nil {
			if t := strings.TrimSpace(m[1]); t != "" {
				title = t
			}
			continue
		}
		m := pkBlockRE.FindStringSubmatch(s)
		if m == nil {
			return "", nil, c.errorf("the statement %q", s)
		}
		f := pkField{start: last + 1, label: pkString(m[4])}
		switch {
		case m[3] != "":
			bits, err := pkInt(c, m[3])
			if err != nil {
				return "", nil, err
			}
			if bits == 0 {
				return "", nil, c.errorf("the field %q of no bits", s)
			}
			f.end = f.start + bits - 1
		default:
			start, err := pkInt(c, m[1])
			if err != nil {
				return "", nil, err
			}
			f.end = start
			if m[2] != "" {
				if f.end, err = pkInt(c, m[2]); err != nil {
					return "", nil, err
				}
			}
			if f.end < start {
				return "", nil, c.errorf("the field %d-%d, which ends before it starts", start, f.end)
			}
			switch {
			case start != f.start && last < 0:
				return "", nil, c.errorf("the first field %d-%d, which does not start at the bit 0", start, f.end)
			case start != f.start:
				return "", nil, c.errorf("the field %d-%d, which does not start at the bit %d after the field before it", start, f.end, f.start)
			}
		}
		last = f.end
		fields = append(fields, f)
	}
	c.line = 0
	return title, fields, nil
}

// pkInt reads a number of bits, as Mermaid reads it: a 0 cannot come
// before another digit.
func pkInt(c *chart, s string) (int, error) {
	if !pkIntRE.MatchString(s) {
		return 0, c.errorf("the number %s", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, c.errorf("the number %s", s)
	}
	return n, nil
}

// pkString reads a string in quotes. A backslash takes the character after
// it, and \n, \t and the other escapes of JavaScript read as their
// characters, as in Mermaid.
func pkString(s string) string {
	s = s[1 : len(s)-1]
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// packetRows splits the fields into rows of bitsPerRow bits. A field that
// does not fit in the rest of its row goes on in the next row, as in
// Mermaid, with its label in each row.
func packetRows(fields []pkField, bitsPerRow int) [][]pkField {
	var rows [][]pkField
	var row []pkField
	for _, f := range fields {
		for len(rows) < pkMaxRows {
			end := (len(rows)+1)*bitsPerRow - 1
			if f.end <= end {
				row = append(row, f)
				if f.end == end {
					rows, row = append(rows, row), nil
				}
				break
			}
			row = append(row, pkField{start: f.start, end: end, label: f.label})
			rows, row = append(rows, row), nil
			f.start = end + 1
		}
	}
	if len(row) > 0 && len(rows) < pkMaxRows {
		rows = append(rows, row)
	}
	return rows
}

// The fonts of a packet: a label, and a number of a bit, which Mermaid
// draws at 12 and 10 pixels.
var (
	pkFont    = text.Font{Family: text.Family, Size: 11, Weight: 400}
	pkBitFont = text.Font{Family: text.Family, Size: 9, Weight: 400}
)

// packetSpec writes the rows of a packet as a Vega spec, with the sizes of
// Mermaid: each row rowHeight high under a space of paddingY, each bit
// bitWidth wide, and a space of paddingX after each field. The number of
// the first bit of a field is over its left end, and the number of its
// last bit over its right end. A field of one bit has one number, in its
// middle. With bitOrder descending, the bits of a row run from right to
// left.
func packetSpec(rows [][]pkField, cfg pkConfig) map[string]any {
	var blocks, starts, ends []any
	for r, row := range rows {
		y := float64(r)*(cfg.rowHeight+cfg.paddingY) + cfg.paddingY
		for _, f := range row {
			n := f.end - f.start + 1
			col := f.start % cfg.bitsPerRow
			if cfg.descending {
				col = cfg.bitsPerRow - col - n
			}
			x := float64(col)*cfg.bitWidth + 1
			w := float64(n)*cfg.bitWidth - cfg.paddingX
			blocks = append(blocks, map[string]any{
				"start": f.start, "end": f.end, "label": f.label,
				"x": x, "y": y, "x2": x + w, "y2": y + cfg.rowHeight,
			})
			if !cfg.showBits {
				continue
			}
			first, last := f.start, f.end
			if cfg.descending {
				first, last = last, first
			}
			if n == 1 {
				starts = append(starts, map[string]any{"bit": first, "x": x + w/2, "y": y - 2, "align": "center"})
				continue
			}
			starts = append(starts, map[string]any{"bit": first, "x": x, "y": y - 2, "align": "left"})
			ends = append(ends, map[string]any{"bit": last, "x": x + w, "y": y - 2, "align": "right"})
		}
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	// themed sets a property of an encoding from a theme variable that the
	// source gives.
	themed := func(enc map[string]any, prop, name string) {
		if v, ok := cfg.theme[name]; ok {
			enc[prop] = value(v)
		}
	}
	block := map[string]any{"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2"), "cornerRadius": value(3)}
	if _, ok := cfg.theme["blockFillColor"]; ok {
		// Mermaid fills a field with the color, opaque.
		block["fillOpacity"] = value(1)
	}
	themed(block, "fill", "blockFillColor")
	themed(block, "stroke", "blockStrokeColor")
	themed(block, "strokeWidth", "blockStrokeWidth")
	label := map[string]any{
		"x": map[string]any{"signal": "(datum.x + datum.x2) / 2"}, "y": map[string]any{"signal": "(datum.y + datum.y2) / 2"},
		"text": field("label"), "align": value("center"), "baseline": value("middle"), "fontSize": value(pkFont.Size),
	}
	themed(label, "fill", "labelColor")
	themed(label, "fontSize", "labelFontSize")
	data := []any{map[string]any{"name": "fields", "values": blocks}}
	marks := []any{
		map[string]any{"type": "rect", "name": "fields", "from": map[string]any{"data": "fields"}, "style": []any{graph.StyleNode},
			"encode": map[string]any{"update": block}},
		map[string]any{"type": "text", "name": "labels", "from": map[string]any{"data": "fields"}, "style": []any{graph.StyleNodeLabel},
			"encode": map[string]any{"update": label}},
	}
	for _, set := range []struct {
		name, color string
		rows        []any
	}{{"startBits", "startByteColor", starts}, {"endBits", "endByteColor", ends}} {
		if len(set.rows) == 0 {
			continue
		}
		enc := map[string]any{
			"x": field("x"), "y": field("y"), "text": field("bit"), "align": field("align"),
			"baseline": value("alphabetic"), "fontSize": value(pkBitFont.Size),
		}
		themed(enc, "fill", set.color)
		themed(enc, "fontSize", "byteFontSize")
		data = append(data, map[string]any{"name": set.name, "values": set.rows})
		marks = append(marks, map[string]any{"type": "text", "name": set.name, "from": map[string]any{"data": set.name},
			"style": []any{graph.StyleEdgeLabel}, "encode": map[string]any{"update": enc}})
	}
	return map[string]any{
		"width":    cfg.bitWidth*float64(cfg.bitsPerRow) + 2,
		"height":   float64(len(rows))*(cfg.rowHeight+cfg.paddingY) + cfg.paddingY,
		"autosize": "pad",
		"padding":  8,
		"data":     data,
		"marks":    marks,
	}
}
