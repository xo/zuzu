package dataflow

import (
	"bytes"
	"encoding/csv"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/geo"
	"github.com/xo/zuzu/vega"
)

// Format says how to read data, with its signals resolved.
type Format struct {
	// Type is json, csv, tsv or dsv. The default is json.
	Type string
	// Delimiter is the delimiter of dsv.
	Delimiter string
	// Header names the columns of delimited text that has no header line.
	Header []string
	// Property is the field of JSON that holds the rows.
	Property string
	// Parse gives the type of each field: boolean, integer, number, date,
	// string, or date:'spec' and utc:'spec' with a format spec of
	// d3-time-format.
	Parse map[string]string
	// Auto infers the type of each field.
	Auto bool
	// Feature names the object of a TopoJSON topology whose geometries
	// are the rows, and Mesh names the object whose shared borders are
	// the row.
	Feature, Mesh string
}

// Read reads rows from the text of data in a format. A value that is not
// an object becomes the row {"data": value}, as Vega reads it.
func Read(src []byte, f Format, loc *time.Location) ([]Row, error) {
	var rows []Row
	switch strings.ToLower(f.Type) {
	case "", "json":
		var v any
		if err := json.Unmarshal(src, &v); err != nil {
			return nil, fmt.Errorf("reading json: %w: %w", ErrFormat, err)
		}
		if f.Property != "" {
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("reading the property %q of json that is not an object: %w", f.Property, ErrFormat)
			}
			v = FieldAccessor(f.Property).Get(m)
		}
		rows = Ingest(v)
	case "topojson":
		var v any
		if err := json.Unmarshal(src, &v); err != nil {
			return nil, fmt.Errorf("reading topojson: %w: %w", ErrFormat, err)
		}
		r, err := ReadTopoJSON(v, f)
		if err != nil {
			return nil, err
		}
		rows = r
	case "csv":
		r, err := readDelimited(src, ',', f.Header)
		if err != nil {
			return nil, err
		}
		rows = r
	case "tsv":
		r, err := readDelimited(src, '\t', f.Header)
		if err != nil {
			return nil, err
		}
		rows = r
	case "dsv":
		if len([]rune(f.Delimiter)) != 1 {
			return nil, fmt.Errorf("reading dsv with the delimiter %q: %w", f.Delimiter, ErrFormat)
		}
		r, err := readDelimited(src, []rune(f.Delimiter)[0], f.Header)
		if err != nil {
			return nil, err
		}
		rows = r
	default:
		return nil, fmt.Errorf("reading the format %q: %w", f.Type, ErrFormat)
	}
	if err := ParseTypes(rows, f, loc); err != nil {
		return nil, err
	}
	return rows, nil
}

// Ingest turns values that JSON gave into rows. An array gives one row
// for each element, and any other value gives one row.
func Ingest(v any) []Row {
	list, ok := v.([]any)
	if !ok {
		if v == nil || expr.IsUndefined(v) {
			return nil
		}
		list = []any{v}
	}
	rows := make([]Row, 0, len(list))
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			rows = append(rows, m)
			continue
		}
		rows = append(rows, Row{"data": e})
	}
	return rows
}

// readDelimited reads delimited text with a header line, or with the
// names of header. A row with fewer fields gets empty strings.
func readDelimited(src []byte, comma rune, header []string) ([]Row, error) {
	r := csv.NewReader(bytes.NewReader(src))
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = false
	cols := header
	var rows []Row
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading delimited text: %w: %w", ErrFormat, err)
		}
		if cols == nil {
			cols = rec
			continue
		}
		row := make(Row, len(cols))
		for i, c := range cols {
			if i < len(rec) {
				row[c] = rec[i]
			} else {
				row[c] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ParseTypes converts the fields of rows to the types that a format gives.
// It changes the rows.
func ParseTypes(rows []Row, f Format, loc *time.Location) error {
	if len(rows) == 0 {
		return nil
	}
	types := f.Parse
	if f.Auto {
		types = inferTypes(rows)
	}
	for field, typ := range types {
		parse, err := parser(typ, loc)
		if err != nil {
			return fmt.Errorf("parsing the field %q: %w", field, err)
		}
		if typ == "unknown" {
			continue
		}
		for _, r := range rows {
			r[field] = parse(r[field])
		}
	}
	return nil
}

// parser returns the function that converts a value to a type.
func parser(typ string, loc *time.Location) (func(any) any, error) {
	switch typ {
	case "boolean":
		return toBoolean, nil
	case "integer", "number":
		return toNumber, nil
	case "date":
		return func(v any) any { return toDate(v, loc) }, nil
	case "string":
		return toString, nil
	case "unknown", "":
		return func(v any) any { return v }, nil
	}
	kind, pattern, ok := strings.Cut(typ, ":")
	if ok && (kind == "date" || kind == "utc") {
		if len(pattern) >= 2 && (pattern[0] == '\'' || pattern[0] == '"') && pattern[len(pattern)-1] == pattern[0] {
			pattern = pattern[1 : len(pattern)-1]
		}
		zone := loc
		if kind == "utc" {
			zone = time.UTC
		}
		parse := format.Parse(pattern, format.EnUS, zone)
		return func(v any) any {
			if v == nil {
				return nil
			}
			t, ok := parse(expr.String(v))
			if !ok {
				return nil
			}
			return t
		}, nil
	}
	return nil, fmt.Errorf("the type %q: %w", typ, ErrFormat)
}

// empty reports whether a value is null, undefined or the empty string.
func empty(v any) bool {
	return v == nil || expr.IsUndefined(v) || v == ""
}

// toBoolean converts a value to a boolean: the strings false and 0 are
// false, and null and the empty string are null.
func toBoolean(v any) any {
	switch {
	case empty(v):
		return nil
	case v == "false" || v == "0":
		return false
	}
	return expr.Truthy(v)
}

// toNumber converts a value to a number. Null and the empty string are
// null.
func toNumber(v any) any {
	if empty(v) {
		return nil
	}
	return expr.Number(v)
}

// toString converts a value to a string. Null and the empty string are
// null.
func toString(v any) any {
	if empty(v) {
		return nil
	}
	return expr.String(v)
}

// toDate converts a value to milliseconds since 1970, as Date.parse reads
// a string. A number and a date stay as they are.
func toDate(v any, loc *time.Location) any {
	switch x := v.(type) {
	case float64, time.Time:
		return x
	case string:
		if x == "" {
			return nil
		}
		t, ok := format.ParseDate(x, loc)
		if !ok {
			return math.NaN()
		}
		return float64(t.UnixMilli())
	}
	if empty(v) {
		return nil
	}
	return math.NaN()
}

// typeTests are the types that inference tries, in order.
var typeTests = []struct {
	name string
	test func(any) bool
}{
	{"boolean", func(v any) bool { return v == "true" || v == "false" || v == true || v == false }},
	{"integer", func(v any) bool {
		f := expr.Number(v)
		_, d := v.(time.Time)
		return !d && !math.IsNaN(f) && f == math.Trunc(f) && !math.IsInf(f, 0)
	}},
	{"number", func(v any) bool { _, d := v.(time.Time); return !d && !math.IsNaN(expr.Number(v)) }},
	{"date", func(v any) bool { _, ok := format.ParseDate(expr.String(v), time.UTC); return ok }},
}

// inferTypes infers the type of each field of the first row from the
// values of every row: the first type of typeTests that every valid value
// passes, or string.
func inferTypes(rows []Row) map[string]string {
	types := map[string]string{}
	for field := range rows[0] {
		pass := make([]bool, len(typeTests))
		for i := range pass {
			pass[i] = true
		}
		for _, r := range rows {
			v := r[field]
			if !valid(v) {
				continue
			}
			for i, t := range typeTests {
				if pass[i] && !t.test(v) {
					pass[i] = false
				}
			}
		}
		types[field] = "string"
		for i, ok := range pass {
			if ok {
				types[field] = typeTests[i].name
				break
			}
		}
	}
	return types
}

// ReadTopoJSON reads the rows of a TopoJSON topology: a row for each
// geometry of the object that Feature names. The mesh of an object, its
// borders joined into lines, is not supported.
func ReadTopoJSON(v any, f Format) ([]Row, error) {
	switch {
	case f.Feature != "":
		fs, err := geo.Feature(v, f.Feature)
		if err != nil {
			return nil, fmt.Errorf("reading topojson: %w: %w", ErrFormat, err)
		}
		return Ingest(fs), nil
	case f.Mesh != "":
		return nil, vega.Issue{Err: vega.ErrUnsupported, Message: "the mesh of topojson"}
	}
	return nil, fmt.Errorf("reading topojson with no feature or mesh: %w", ErrFormat)
}
