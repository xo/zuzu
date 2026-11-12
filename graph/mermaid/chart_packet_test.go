package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
)

// pkWant is what a fixture of a packet draws. fields holds each field as
// its first bit, its last bit and its label, in the order of the source,
// with a field that goes on in the next row split in two. The other fields
// are the bits of a row, the width of a bit, whether the numbers of the
// bits draw, whether the bits run from right to left, and the title.
type pkWant struct {
	fields     []string
	bitsPerRow int
	bitWidth   float64
	showBits   bool
	descending bool
	title      string
}

func TestPacketFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]pkWant{
		"01-tcp.mmd": {
			fields: []string{
				"0-15 Source Port", "16-31 Destination Port", "32-63 Sequence Number", "64-95 Acknowledgment Number",
				"96-99 Data Offset", "100-105 Reserved", "106-106 URG", "107-107 ACK", "108-108 PSH", "109-109 RST",
				"110-110 SYN", "111-111 FIN", "112-127 Window", "128-143 Checksum", "144-159 Urgent Pointer",
				"160-191 (Options and Padding)", "192-223 Data (variable length)", "224-255 Data (variable length)",
			},
			bitsPerRow: 32, bitWidth: 32, showBits: true, title: "TCP Packet",
		},
		"02-relative.mmd": {
			fields:     []string{"0-15 Source Port", "16-31 Destination Port", "32-47 Length", "48-63 Checksum", "64-95 Data (variable length)"},
			bitsPerRow: 32, bitWidth: 32, showBits: true, title: "UDP Packet",
		},
		"03-descending.mmd": {
			fields:     []string{"0-7 DATA", "8-11 TYPE", "12-12 EN", "13-15 RESERVED"},
			bitsPerRow: 16, bitWidth: 32, showBits: true, descending: true, title: "Status Register",
		},
		"04-no-bits.mmd": {
			fields:     []string{"0-3 Head", "4-7 Body", "8-15 Body", "16-21 Body", "22-23 Tail"},
			bitsPerRow: 8, bitWidth: 24,
		},
		"05-grammar.mmd": {
			fields:     []string{"0-3 Version", "4-7 IHL", "8-13 DSCP", `14-15 a "quoted" field`},
			bitsPerRow: 32, bitWidth: 32, showBits: true,
		},
	}
	files := fixtures(t, "packet")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			want, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			items, report, err := drawDiagram(t, string(src), true)
			if err != nil || len(report) != 0 {
				t.Fatalf("the report %v and the error %v", report, err)
			}
			checkPacket(t, items, want)
			d, err := mermaid.Read(string(src))
			if err != nil {
				t.Fatal(err)
			}
			b, _, _, err := mermaid.Chart(d, true)
			if err != nil {
				t.Fatal(err)
			}
			var spec struct {
				Title struct {
					Text string `json:"text"`
				} `json:"title"`
			}
			if err := json.Unmarshal(b, &spec); err != nil {
				t.Fatal(err)
			}
			if spec.Title.Text != want.title {
				t.Errorf("expected the title %q, got %q", want.title, spec.Title.Text)
			}
		})
	}
}

// checkPacket checks the fields of a packet that drew: their bits, their
// labels, their places in the rows, and the numbers of their bits.
func checkPacket(t *testing.T, items []drawnItem, want pkWant) {
	t.Helper()
	fields := drawnOf(items, "fields")
	labels := drawnOf(items, "labels")
	if len(fields) == 0 {
		t.Fatal("no field")
	}
	// col returns the column of the first bit of a field from the left.
	col := func(f drawnItem) float64 {
		start, end := int(num(f.datum["start"])), int(num(f.datum["end"]))
		c := float64(start % want.bitsPerRow)
		if want.descending {
			c = float64(want.bitsPerRow) - c - float64(end-start+1)
		}
		return c
	}
	left := fields[0].item.X - col(fields[0])*want.bitWidth
	var got []string
	for _, f := range fields {
		start, end := int(num(f.datum["start"])), int(num(f.datum["end"]))
		got = append(got, fmt.Sprintf("%d-%d %v", start, end, f.datum["label"]))
		// A field is as wide as its bits, and in the row and the columns
		// of its bits.
		b := boxOf(f)
		bits := float64(end - start + 1)
		if x := left + col(f)*want.bitWidth; b.x0 != x {
			t.Errorf("the field %d-%d starts at %v, not at %v", start, end, b.x0, x)
		}
		if w := bits*want.bitWidth - 5; b.x1-b.x0 != w {
			t.Errorf("the field %d-%d is %v wide, not %v", start, end, b.x1-b.x0, w)
		}
		row := start / want.bitsPerRow
		if end/want.bitsPerRow != row {
			t.Errorf("the field %d-%d spans two rows", start, end)
		}
	}
	if !slices.Equal(got, want.fields) {
		t.Errorf("expected the fields %q, got %q", want.fields, got)
	}
	if len(labels) != len(fields) {
		t.Errorf("expected a label for each field, got %d", len(labels))
	}
	for i, l := range labels {
		if i < len(fields) && l.text != fields[i].datum["label"] {
			t.Errorf("expected the label %q, got %q", fields[i].datum["label"], l.text)
		}
	}
	nBits := 0
	if want.showBits {
		for _, f := range fields {
			nBits += 2
			if f.datum["start"] == f.datum["end"] {
				nBits--
			}
		}
	}
	if n := len(drawnOf(items, "startBits")) + len(drawnOf(items, "endBits")); n != nBits {
		t.Errorf("expected %d numbers of bits, got %d", nBits, n)
	}
}

func TestPacketErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"a gap between two fields":   "packet\n  0-10: \"a\"\n  12-15: \"b\"\n",
		"a field that ends before":   "packet\n  0-10: \"a\"\n  10-5: \"b\"\n",
		"a field of no bits":         "packet\n  +0: \"a\"\n",
		"a first field after bit 0":  "packet\n  1-5: \"a\"\n",
		"two fields on a line":       "packet\n  0-10: \"a\" 11: \"b\"\n",
		"a label with no quotes":     "packet\n  0-10: a\n",
		"a number with a 0 before":   "packet\n  00-3: \"a\"\n",
		"a title in capitals":        "packet\n  0-3: \"a\"\n  TITLE x\n",
		"a number with a fraction":   "packet\n  0-1.5: \"a\"\n",
		"an accDescr with no end":    "packet\n  accDescr {\n  0-3: \"a\"\n",
		"a field with no colon":      "packet\n  0-3 \"a\"\n",
		"a string with no end quote": "packet\n  0-3: \"a\n",
	}
	for name, src := range tests {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, _, _, err := mermaid.Chart(d, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%s: expected an error of the syntax, got %v", name, err)
		}
	}
}

func TestPacketReads(t *testing.T) {
	t.Parallel()
	// Each source draws with no issue, as it does in Mermaid.
	tests := map[string]string{
		"no field":                 "packet\n",
		"a color of the theme":     "---\nconfig:\n  themeVariables:\n    packet:\n      startByteColor: red\n---\npacket\n  0-3: \"a\"\n",
		"an empty label":           "packet\n  0-3: \"\"\n",
		"a title of no text":       "packet\n  title\n  0-3: \"a\"\n",
		"a field after the word":   "packet 0-3: \"a\"\n",
		"spaces around the dash":   "packet\n  0 - 3 : \"a\"\n",
		"a field over three rows":  "packet\n  0-70: \"long\"\n  +40: \"rel\"\n",
		"the settings of a packet": "---\nconfig:\n  packet:\n    rowHeight: 20\n    bitWidth: 20\n    bitsPerRow: 16\n    paddingX: 2\n    paddingY: 2\n    showBits: false\n    bitOrder: ascending\n    useMaxWidth: false\n---\npacket\n  0-3: \"a\"\n",
	}
	for name, src := range tests {
		report, err := chartOf(src, true)
		if err != nil || len(report) != 0 {
			t.Errorf("%s: the report %v and the error %v", name, report, err)
		}
	}
}

func TestPacketSettings(t *testing.T) {
	t.Parallel()
	const body = "packet\n  0-3: \"a\"\n"
	tests := map[string]string{
		"bits per row of no number":    "---\nconfig:\n  packet:\n    bitsPerRow: many\n---\n" + body,
		"bits per row with a fraction": "---\nconfig:\n  packet:\n    bitsPerRow: 7.5\n---\n" + body,
		"a row of no height":           "---\nconfig:\n  packet:\n    rowHeight: 0\n---\n" + body,
		"showBits of no boolean":       "---\nconfig:\n  packet:\n    showBits: 1\n---\n" + body,
		"another order of bits":        "---\nconfig:\n  packet:\n    bitOrder: random\n---\n" + body,
		"another theme variable":       "---\nconfig:\n  themeVariables:\n    primaryColor: red\n---\n" + body,
		"a size that is not a number":  "---\nconfig:\n  themeVariables:\n    packet:\n      labelFontSize: big\n---\n" + body,
		"another setting":              "---\nconfig:\n  packet:\n    padding: 3\n---\n" + body,
	}
	for name, src := range tests {
		if _, err := chartOf(src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%s: expected an issue, got %v", name, err)
		}
		report, err := chartOf(src, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%s: expected one line of the report, got %v and %v", name, report, err)
		}
	}
}

func TestPacketTheme(t *testing.T) {
	t.Parallel()
	// The theme variables of a packet set the colors and the sizes of its
	// parts, as in Mermaid (D32).
	src := "---\nconfig:\n  themeVariables:\n    packet:\n      startByteColor: red\n      endByteColor: blue\n" +
		"      labelColor: green\n      labelFontSize: 14px\n      byteFontSize: 8\n      blockFillColor: '#ffeecc'\n" +
		"      blockStrokeColor: purple\n      blockStrokeWidth: 2\n      titleColor: red\n      titleFontSize: 20px\n---\n" +
		"packet\n  title Theme\n  0-15: \"a\"\n"
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	field := drawnOf(items, "fields")[0].item
	label := drawnOf(items, "labels")[0].item
	start, end := drawnOf(items, "startBits")[0].item, drawnOf(items, "endBits")[0].item
	for _, tt := range []struct{ name, got, want string }{
		{"the fill of a field", field.Fill.Color, "#ffeecc"},
		{"the stroke of a field", field.Stroke.Color, "purple"},
		{"the color of a label", label.Fill.Color, "green"},
		{"the color of a first bit", start.Fill.Color, "red"},
		{"the color of a last bit", end.Fill.Color, "blue"},
	} {
		if tt.got != tt.want {
			t.Errorf("%s: expected %s, got %s", tt.name, tt.want, tt.got)
		}
	}
	if label.FontSize != 14 || start.FontSize != 8 || field.StrokeWidth == nil || *field.StrokeWidth != 2 {
		t.Errorf("expected the sizes 14, 8 and 2, got %v, %v and %v", label.FontSize, start.FontSize, field.StrokeWidth)
	}
}
