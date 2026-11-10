package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// The types of a field.
const (
	quantitative = "quantitative"
	nominal      = "nominal"
	ordinal      = "ordinal"
	temporal     = "temporal"
)

// channelDef is the definition of a channel after the compiler has read
// it: the field that it reads, the field that the marks draw, and how.
type channelDef struct {
	channel string
	def     *FieldDef
	// field is the field of the data, and typ its type.
	field string
	typ   string
	// aggregate is the op of an aggregate, such as sum or count.
	aggregate string
	// argField is the field whose least or greatest value an argmin or an
	// argmax finds. The channel reads its field of that row.
	argField string
	// bin holds the parameters of a bin, and binned says that the data is
	// already binned.
	bin    map[string]any
	binned bool
	// units are the units of time of a time unit, and utc says that they
	// are in UTC.
	units    []string
	unitName string
	// unitStep is the number of units of each interval, when it is not 1.
	unitStep float64
	// nested is the path of a field inside an object, which the data
	// flattens into the field.
	nested []string
	// bracket says that the field ends with a bracket, as a['b'] does,
	// after which a suffix is a part of its own.
	bracket bool
	utc     bool
	// binnedUnit says that the data holds the start of each unit already.
	binnedUnit bool
	// out is the field that the marks draw, and outEnd the end of a bin or
	// of a time unit.
	out, outEnd string
	// label is the field of the text of each bin of a discrete channel,
	// such as "1 – 2", and sortBy the field that orders those texts.
	label, sortBy string
	// value is a constant value of the channel, and hasValue says that it
	// has one.
	value    any
	hasValue bool
	// datum is a constant of the data, which goes through the scale.
	datum    any
	hasDatum bool
	// cond is the condition of the channel, if it has one.
	cond []condition
}

// condition is one condition of a channel: a test and the value or the
// field it gives.
type condition struct {
	test     string
	value    any
	hasValue bool
	field    string
}

// isAggregate reports whether the channel aggregates its field.
func (c *channelDef) isAggregate() bool {
	return c.aggregate != ""
}

// isField reports whether the channel reads a field of the data.
func (c *channelDef) isField() bool {
	return c.field != "" || c.aggregate == "count"
}

// continuous reports whether the channel draws on a continuous scale.
func (c *channelDef) continuous() bool {
	switch c.typ {
	case quantitative:
		return true
	case temporal:
		return true
	}
	return false
}

// readChannel reads the definition of a channel. repeat names the field of
// a repeat, for a field {"repeat": "row"}.
func readChannel(channel string, def *FieldDef, repeat map[string]string) (*channelDef, error) {
	c := &channelDef{channel: channel, def: def}
	if !def.Field.IsZero() {
		var f any
		if err := def.Field.Decode(&f); err != nil {
			return nil, err
		}
		switch x := f.(type) {
		case string:
			c.field = x
		case map[string]any:
			if r, ok := x["repeat"].(string); ok {
				c.field = repeat[r]
			}
		}
		c.bracket = strings.HasSuffix(c.field, "]")
		if parts := dataflow.SplitField(c.field); len(parts) > 1 {
			// A field inside an object becomes a flat field whose name
			// joins the path with dots, which the compiler escapes.
			c.nested = parts
			c.field = escapeField(strings.Join(parts, "."))
		}
	}
	c.typ = def.Type
	if !def.Aggregate.IsZero() {
		var a any
		if err := def.Aggregate.Decode(&a); err != nil {
			return nil, err
		}
		switch x := a.(type) {
		case string:
			c.aggregate = x
		case map[string]any:
			for _, op := range []string{"argmin", "argmax"} {
				if v, ok := x[op]; ok {
					c.aggregate = op
					c.argField = expr.String(v)
				}
			}
		}
	}
	if !def.Bin.IsZero() {
		var b any
		if err := def.Bin.Decode(&b); err != nil {
			return nil, err
		}
		switch x := b.(type) {
		case bool:
			if x {
				c.bin = map[string]any{}
			}
		case string:
			c.binned = x == "binned"
		case map[string]any:
			if x["binned"] == true {
				c.binned = true
			} else {
				c.bin = x
			}
		}
	}
	if !def.TimeUnit.IsZero() {
		var tu any
		if err := def.TimeUnit.Decode(&tu); err != nil {
			return nil, err
		}
		name := ""
		switch x := tu.(type) {
		case string:
			name = x
		case map[string]any:
			name = expr.String(x["unit"])
		}
		if rest, ok := strings.CutPrefix(name, "binned"); ok {
			// The data holds the start of each unit already, and the end
			// is one unit later.
			name = rest
			c.binnedUnit = true
		}
		c.units, c.utc = timeUnits(name)
		c.unitName = strings.TrimPrefix(name, "utc")
		if c.utc {
			c.unitName = name
		}
		if m, ok := tu.(map[string]any); ok {
			if st, ok := m["step"].(float64); ok && st != 1 {
				// A step of more than one unit names the field after it.
				c.unitStep = st
				c.unitName += "_step_" + strconv.FormatFloat(st, 'f', -1, 64)
			}
		}
	}
	if !def.Value.IsZero() {
		c.hasValue = true
		if def.Value.Signal != "" {
			c.value = map[string]any{"signal": def.Value.Signal}
		} else if err := def.Value.Decode(&c.value); err != nil {
			return nil, err
		}
	}
	if !def.Datum.IsZero() {
		c.hasDatum = true
		if err := def.Datum.Decode(&c.datum); err != nil {
			return nil, err
		}
		if m, ok := c.datum.(map[string]any); ok {
			if r, ok := m["repeat"].(string); ok {
				// The datum of a repeat is the value of the repeat.
				c.datum = repeat[r]
			}
		}
	}
	if len(def.Condition) > 0 {
		conds, err := readConditions(def.Condition)
		if err != nil {
			return nil, err
		}
		c.cond = conds
	}
	if c.typ == "" {
		switch {
		case c.aggregate != "" && c.aggregate != "argmin" && c.aggregate != "argmax":
			c.typ = quantitative
		case c.bin != nil || c.binned:
			c.typ = quantitative
		case c.units != nil:
			c.typ = temporal
		case channel == "theta" || channel == "radius" || channel == "size" || channel == "opacity":
			c.typ = quantitative
		case scaleTypeIn(def, "time", "utc"):
			c.typ = temporal
		case scaleTypeIn(def, "linear", "log", "pow", "sqrt", "symlog", "quantize", "quantile", "threshold"):
			// A continuous scale reads numbers.
			c.typ = quantitative
		case c.hasDatum:
			switch c.datum.(type) {
			case float64:
				c.typ = quantitative
			default:
				c.typ = nominal
			}
		default:
			c.typ = nominal
		}
	}
	if c.bin != nil && c.bin["maxbins"] == nil && c.bin["step"] == nil {
		// A facet and a channel such as color or size have few bins by
		// default, as Vega-Lite documents.
		switch channel {
		case "row", "column", "facet", "size", "color", "fill", "stroke", "strokeWidth", "opacity", "fillOpacity", "strokeOpacity", "shape":
			c.bin["maxbins"] = 6.0
		case "strokeDash":
			c.bin["maxbins"] = 4.0
		}
	}
	c.out = c.outputField()
	if c.binnedUnit {
		c.out = c.field
	}
	switch {
	case c.bin != nil:
		c.outEnd = c.out + "_end"
	case c.units != nil:
		c.outEnd = c.out + "_end"
	}
	return c, nil
}

// readConditions reads the condition of a channel: one condition or a
// list. A condition with a param of a selection is not drawn, since a
// person cannot change a static chart (D7).
func readConditions(raw []byte) ([]condition, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	list, ok := v.([]any)
	if !ok {
		list = []any{v}
	}
	var out []condition
	for _, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		c := condition{}
		switch t := m["test"].(type) {
		case string:
			c.test = t
		case map[string]any:
			c.test = predicate(t)
		}
		if _, ok := m["param"]; ok {
			// An empty selection selects every item, so the condition holds.
			c.test = "true"
		}
		if v, ok := m["value"]; ok {
			c.value, c.hasValue = v, true
		}
		if f, ok := m["field"].(string); ok {
			c.field = f
		}
		out = append(out, c)
	}
	return out, nil
}

// outputField returns the name of the field that the marks draw, as
// Vega-Lite names it: sum_price for an aggregate, bin_maxbins_10_price for
// a bin, month_date for a time unit, and `__count` for a count.
func (c *channelDef) outputField() string {
	switch {
	case c.aggregate == "count":
		return "__count"
	case c.argField != "":
		return c.aggregate + "_" + c.argField + "[" + strconv.Quote(c.field) + "]"
	case c.aggregate != "":
		return c.aggregate + "_" + c.field
	case c.bin != nil:
		return binPrefix(c.bin) + "_" + c.field
	case c.units != nil:
		return c.unitName + "_" + c.field
	}
	return c.field
}

// binPrefix names a bin by its parameters, as bin_maxbins_10 for the
// default of 10 bins.
func binPrefix(b map[string]any) string {
	keys := make([]string, 0, len(b))
	for k := range b {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"_"+binValue(b[k]))
	}
	if _, ok := b["maxbins"]; !ok && b["step"] == nil && b["steps"] == nil && b["extent"] == nil {
		parts = append([]string{"maxbins_10"}, parts...)
	}
	return "bin_" + strings.Join(parts, "_")
}

// binValue writes a parameter of a bin in its name.
func binValue(v any) string {
	switch x := v.(type) {
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = binValue(e)
		}
		return strings.Join(parts, "_")
	case map[string]any:
		if s, ok := x["param"].(string); ok {
			return s
		}
	}
	return expr.String(v)
}

// unitOrder is the order of the units of time in a name such as
// yearmonthdate.
var unitOrder = []string{"year", "quarter", "month", "week", "day", "dayofyear", "date", "hours", "minutes", "seconds", "milliseconds"}

// timeUnits splits the name of a time unit, such as yearmonth or
// utcmonthdate, into its units, and reports whether it is in UTC.
func timeUnits(name string) ([]string, bool) {
	utc := strings.HasPrefix(name, "utc")
	rest := strings.TrimPrefix(name, "utc")
	var units []string
	for rest != "" {
		found := false
		// The longest name first, so that dayofyear wins over day.
		names := slices.Clone(unitOrder)
		sort.SliceStable(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
		for _, u := range names {
			if strings.HasPrefix(rest, u) {
				units = append(units, u)
				rest = rest[len(u):]
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	slices.SortStableFunc(units, func(a, b string) int {
		return slices.Index(unitOrder, a) - slices.Index(unitOrder, b)
	})
	return units, utc
}

// varName turns a name into a name that a signal can have.
var varRE = regexp.MustCompile(`\W`)

// varName returns a name with every character that a name of a signal
// cannot hold as an underscore.
func varName(s string) string {
	return varRE.ReplaceAllString(s, "_")
}

// fieldRef writes a field as an expression reads it, datum["name"].
// A name with a path, such as a.b or a["b"], reads each step.
func fieldRef(name string) string {
	var b strings.Builder
	b.WriteString("datum")
	for _, part := range dataflow.SplitField(name) {
		q, _ := json.Marshal(part)
		b.WriteString("[" + string(q) + "]")
	}
	return b.String()
}

// literal returns the plain name of a field that the compiler names with
// escapes, such as "a.b" for "a\\.b".
func literal(name string) string {
	if !strings.Contains(name, "\\") {
		return name
	}
	return strings.Join(dataflow.SplitField(name), ".")
}

// escapeField escapes the dots and the brackets of a plain name, so that
// Vega reads it as one field.
func escapeField(name string) string {
	r := strings.NewReplacer("\\", "\\\\", ".", "\\.", "[", "\\[", "]", "\\]")
	return r.Replace(name)
}

// flattenExpr writes the expression that reads a field inside an object,
// with a test of each object on the way.
func flattenExpr(path []string) string {
	var parts []string
	var ref strings.Builder
	ref.WriteString("datum")
	for _, p := range path {
		q, _ := json.Marshal(p)
		ref.WriteString("[" + string(q) + "]")
		parts = append(parts, ref.String())
	}
	return strings.Join(parts, " && ")
}

// predicate writes a predicate of a filter, such as {"field": "x",
// "equal": 3}, as an expression.
func predicate(p map[string]any) string {
	if v, ok := p["and"].([]any); ok {
		return joinPredicates(v, " && ")
	}
	if v, ok := p["or"].([]any); ok {
		return joinPredicates(v, " || ")
	}
	if v, ok := p["not"]; ok {
		return "!(" + predicateOf(v) + ")"
	}
	field, _ := p["field"].(string)
	f := fieldRef(field)
	var unit string
	utc := false
	if tu, ok := p["timeUnit"]; ok {
		name := expr.String(tu)
		if m, ok := tu.(map[string]any); ok {
			name = expr.String(m["unit"])
		}
		var units []string
		if rest, ok := strings.CutPrefix(name, "binned"); ok {
			// The field holds the start of each unit already.
			units, utc = timeUnits(rest)
			f = "time(" + f + ")"
		} else {
			units, utc = timeUnits(name)
			f = timeFloorExpr(units, utc, f)
		}
		if len(units) == 1 {
			unit = units[0]
		}
	}
	lit := func(v any) string {
		if m, ok := v.(map[string]any); ok {
			if _, ok := m["year"]; ok {
				return dateTimeExpr(m)
			}
		}
		if unit != "" && v != nil {
			// A value of a single time unit is a date in that unit.
			if _, isList := v.([]any); !isList {
				return dateTimeExpr(map[string]any{unit: v, "utc": utc})
			}
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
	switch {
	case p["equal"] != nil:
		return f + "===" + lit(p["equal"])
	case p["lt"] != nil:
		return f + "<" + lit(p["lt"])
	case p["lte"] != nil:
		return f + "<=" + lit(p["lte"])
	case p["gt"] != nil:
		return f + ">" + lit(p["gt"])
	case p["gte"] != nil:
		return f + ">=" + lit(p["gte"])
	case p["range"] != nil:
		r, _ := p["range"].([]any)
		var parts []string
		if len(r) > 0 && r[0] != nil {
			parts = append(parts, lit(r[0])+" <= "+f)
		}
		if len(r) > 1 && r[1] != nil {
			parts = append(parts, f+" <= "+lit(r[1]))
		}
		return strings.Join(parts, " && ")
	case p["oneOf"] != nil:
		return "indexof(" + lit(p["oneOf"]) + ", " + f + ") !== -1"
	case p["in"] != nil:
		return "indexof(" + lit(p["in"]) + ", " + f + ") !== -1"
	case p["valid"] != nil:
		if p["valid"] == true {
			return f + "!==null && !isNaN(" + f + ")"
		}
		return f + "===null || isNaN(" + f + ")"
	}
	return "true"
}

// predicateOf writes a predicate that is an expression or an object.
func predicateOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if p, ok := x["param"]; ok {
			_ = p
			return "true"
		}
		return predicate(x)
	}
	return "true"
}

// joinPredicates joins predicates with an operator.
func joinPredicates(list []any, op string) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = "(" + predicateOf(v) + ")"
	}
	return strings.Join(parts, op)
}

// timeFloorExpr writes the start of the time unit of a field.
func timeFloorExpr(units []string, utc bool, f string) string {
	fn := "datetime"
	pre := ""
	if utc {
		fn, pre = "utc", "utc"
	}
	has := func(u string) bool { return slices.Contains(units, u) }
	part := func(u, getter, def string) string {
		if has(u) {
			return pre + getter + "(" + f + ")"
		}
		return def
	}
	date := part("date", "date", "1")
	if has("day") {
		// A day of the week falls in the first week of 2012, which starts
		// on a Sunday.
		date = pre + "day(" + f + ")+1"
	}
	args := []string{
		part("year", "year", "2012"),
		part("month", "month", "0"),
		date,
		part("hours", "hours", "0"),
		part("minutes", "minutes", "0"),
		part("seconds", "seconds", "0"),
		part("milliseconds", "milliseconds", "0"),
	}
	return "time(" + fn + "(" + strings.Join(args, ", ") + "))"
}

// dateTimeExpr writes a date time object of Vega-Lite, such as {"year":
// 2000, "month": 1}, as an expression.
func dateTimeExpr(m map[string]any) string {
	fn := "datetime"
	if m["utc"] == true {
		fn = "utc"
	}
	month := "0"
	if v, ok := m["month"]; ok {
		if s, ok := v.(string); ok {
			month = strconv.Itoa(monthIndex(s))
		} else {
			month = fmt.Sprintf("%v-1", expr.String(v))
		}
	}
	get := func(k, def string) string {
		if v, ok := m[k]; ok {
			return expr.String(v)
		}
		return def
	}
	if d, ok := m["day"]; ok {
		// A day of the week is a date in the first week of 2012.
		idx := 0
		switch x := d.(type) {
		case float64:
			idx = int(x)
		case string:
			idx = dayIndex(x)
		}
		m = maps.Clone(m)
		m["date"] = idx + 1
	}
	return "time(" + fn + "(" + strings.Join([]string{get("year", "2012"), month, get("date", "1"), get("hours", "0"), get("minutes", "0"), get("seconds", "0"), get("milliseconds", "0")}, ", ") + "))"
}

// dayIndex returns the index from 0, Sunday, of a day by its name.
func dayIndex(s string) int {
	names := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	s = strings.ToLower(s)
	for i, n := range names {
		if strings.HasPrefix(s, n) {
			return i
		}
	}
	return 0
}

// monthIndex returns the index from 0 of a month by its name.
func monthIndex(s string) int {
	names := []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	s = strings.ToLower(s)
	for i, n := range names {
		if strings.HasPrefix(s, n) {
			return i
		}
	}
	return 0
}

// lit returns a Value that holds a literal.
func lit(v any) vega.Value {
	return vega.Lit(v)
}

// scaleTypeIn reports whether the scale of a definition has one of some
// types.
func scaleTypeIn(def *FieldDef, types ...string) bool {
	if def == nil || def.Scale.IsZero() {
		return false
	}
	m, _ := valueOf(def.Scale).(map[string]any)
	t, _ := m["type"].(string)
	return t != "" && slices.Contains(types, t)
}

// stackName returns the name of the start or the end of the stack of a
// channel, such as sum_x_start. After a field that ends with a bracket,
// the suffix is a part of the path of its own, as in vega-lite.js.
func (c *channelDef) stackName(suffix string) string {
	if c.bracket && c.nested != nil {
		return c.out + "\\._" + suffix
	}
	return c.out + "_" + suffix
}
