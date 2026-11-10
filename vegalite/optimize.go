package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/vega"
)

// mergeIdentical merges the datasets that derive from the same dataset
// with the same transforms into one, as vega-lite.js does. Then a dataset
// that has one dataset derived from it, and that nothing else reads, takes
// the transforms of that dataset. A dataset that goes frees its number. It
// reports whether it changed anything.
func (c *compiler) mergeIdentical(out *vega.Spec, read map[string]bool) bool {
	changed := false
	for c.mergeOnce(out, read) {
		changed = true
	}
	return changed
}

// mergeOnce makes one merge, and reports whether it made one.
func (c *compiler) mergeOnce(out *vega.Spec, read map[string]bool) bool {
	children := map[string][]int{}
	for i, d := range c.data {
		if len(d.Source.List) == 1 {
			children[d.Source.List[0]] = append(children[d.Source.List[0]], i)
		}
	}
	for _, p := range c.data {
		kids := children[p.Name]
		seen := map[string]int{}
		for _, k := range kids {
			if invalidFilter(c.data[k].Transform) && !c.shared[p.Name] {
				// Only the views that share a source share their filters
				// of invalid values.
				continue
			}
			key, err := canonical(c.data[k].Transform)
			if err != nil {
				continue
			}
			first, ok := seen[key]
			if !ok {
				seen[key] = k
				continue
			}
			c.drop(out, read, k, c.data[first].Name)
			return true
		}
	}
	for _, p := range c.data {
		if c.mergePrefix(out, read, children[p.Name]) {
			return true
		}
	}
	for pi, p := range c.data {
		kids := children[p.Name]
		if len(kids) != 1 || read[p.Name] || !p.Values.IsZero() || !c.derived(p.Name) {
			continue
		}
		k := kids[0]
		c.data[pi].Transform = append(c.data[pi].Transform, c.data[k].Transform...)
		c.drop(out, read, k, p.Name)
		return true
	}
	return false
}

// mergePrefix finds two datasets of a list that start with the same
// transform, other than a filter of invalid values, and moves that
// transform into a new dataset that both derive from. The new dataset
// takes the name of the first, which takes a new name. It reports whether
// it made a change.
func (c *compiler) mergePrefix(out *vega.Spec, read map[string]bool, kids []int) bool {
	seen := map[string]int{}
	var group []int
	for _, k := range kids {
		ts := c.data[k].Transform
		if len(ts) == 0 || invalidFilter(ts[:1]) || ts[0].Type == "lookup" || shiftedUnits(ts) {
			// vega-lite.js keeps a lookup in each view that has one.
			continue
		}
		key, err := canonical(ts[0])
		if err != nil {
			continue
		}
		first, ok := seen[key]
		if !ok {
			seen[key] = k
			continue
		}
		group = []int{first, k}
		break
	}
	if group == nil {
		return false
	}
	a := group[0]
	name := c.data[a].Name
	renamed := fmt.Sprintf("data_%d", c.nData)
	// A merged step that its parent can take in leaves the children
	// numbered last to first, and one that stays a dataset of its own,
	// first to last, as vega-lite.js numbers them.
	absorbed := false
	if src := c.data[a].Source.List; len(src) == 1 {
		for _, d := range c.data {
			if d.Name == src[0] {
				kids := 0
				for _, e := range c.data {
					if len(e.Source.List) == 1 && e.Source.List[0] == d.Name {
						kids++
					}
				}
				absorbed = kids == len(group) && d.Values.IsZero() && !read[d.Name] && c.derived(d.Name)
			}
		}
	}
	if n, ok := dataNumber(name); ok && !absorbed {
		// The first takes the next number, and the datasets after it
		// move up one, as vega-lite.js numbers them.
		names := map[string]string{}
		for _, d := range c.data {
			if m, ok := dataNumber(d.Name); ok && m >= n {
				names[d.Name] = fmt.Sprintf("data_%d", m+1)
			}
		}
		renamed = names[name]
		c.nData++
		c.renameAll(out, read, names)
	} else {
		c.nData++
		c.renameAll(out, read, map[string]string{name: renamed})
	}
	merged := vega.Data{Name: name, Source: c.data[a].Source, Transform: c.data[a].Transform[:1:1]}
	c.data[a].Transform = c.data[a].Transform[1:]
	c.data[a].Source = vega.List(name)
	b := group[1]
	c.data[b].Transform = c.data[b].Transform[1:]
	c.data[b].Source = vega.List(name)
	c.data = slices.Insert(c.data, a, merged)
	// The datasets left with no transforms read the new dataset.
	for _, n := range []string{c.data[b+1].Name, renamed} {
		for i, d := range c.data {
			if d.Name == n && len(d.Transform) == 0 {
				c.drop(out, read, i, name)
				break
			}
		}
	}
	return true
}

// canonical writes a value as JSON with the keys of every object sorted,
// so that two equal values give the same text.
func canonical(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return "", err
	}
	b, err = json.Marshal(x, json.Deterministic(true))
	return string(b), err
}

// invalidFilter reports whether transforms hold the filter of invalid
// values of a view, which vega-lite.js never merges.
func invalidFilter(ts []vega.Transform) bool {
	for _, t := range ts {
		if t.Type == "filter" && strings.Contains(string(t.Params["expr"]), "isValid(datum[") {
			return true
		}
	}
	return false
}

// derived reports whether a dataset is one that the compiler names, a
// source or a dataset derived from one, and not a dataset the spec names.
func (c *compiler) derived(name string) bool {
	return strings.HasPrefix(name, "source_") || strings.HasPrefix(name, "data_")
}

// drop removes a dataset, makes everything that reads it read another,
// and gives the datasets numbered after it the numbers one less.
func (c *compiler) drop(out *vega.Spec, read map[string]bool, i int, to string) {
	gone := c.data[i].Name
	c.data = append(c.data[:i], c.data[i+1:]...)
	names := map[string]string{gone: to}
	if n, ok := dataNumber(gone); ok {
		c.nData--
		for _, d := range c.data {
			if m, ok := dataNumber(d.Name); ok && m > n {
				names[d.Name] = "data_" + strconv.Itoa(m-1)
			}
		}
		if m, ok := dataNumber(to); ok && m > n {
			names[gone] = "data_" + strconv.Itoa(m-1)
		}
	}
	c.renameAll(out, read, names)
}

// reverseNames gives the datasets of a list the names of the list in the
// other order, as vega-lite.js names the children of a merged aggregate.
func (c *compiler) reverseNames(out *vega.Spec, read map[string]bool, list []int) {
	names := map[string]string{}
	for i, k := range list {
		if _, ok := dataNumber(c.data[k].Name); !ok {
			return
		}
		names[c.data[k].Name] = c.data[list[len(list)-1-i]].Name
	}
	moved := make([]vega.Data, len(list))
	for i, k := range list {
		moved[len(list)-1-i] = c.data[k]
	}
	for i, k := range list {
		c.data[k] = moved[i]
	}
	c.renameAll(out, read, names)
}

// renameAll renames datasets everywhere that reads them: the datasets, the
// marks, the domains of the scales and the set of datasets that are read.
func (c *compiler) renameAll(out *vega.Spec, read map[string]bool, names map[string]string) {
	rename := func(s string) string {
		if r, ok := names[s]; ok {
			return r
		}
		return s
	}
	for j := range c.data {
		c.data[j].Name = rename(c.data[j].Name)
		for k := range c.data[j].Source.List {
			c.data[j].Source.List[k] = rename(c.data[j].Source.List[k])
		}
		for k, t := range c.data[j].Transform {
			if t.Type != "lookup" {
				continue
			}
			var from string
			if json.Unmarshal(t.Params["from"], &from) == nil && rename(from) != from {
				c.data[j].Transform[k].Params["from"] = mustJSON(rename(from))
			}
		}
	}
	for j := range out.Marks {
		renameMarkData(&out.Marks[j], rename)
	}
	if c.proj != nil {
		for j := range c.proj.fit {
			if f := &c.proj.fit[j]; f.data != "" {
				f.data = rename(f.data)
			}
		}
	}
	for j := range out.Scales {
		renameDomain(out.Scales[j].Domain.Data, rename)
	}
	next := map[string]bool{}
	for k, v := range read {
		next[rename(k)] = v
	}
	clear(read)
	maps.Copy(read, next)
}

// mergeSources merges the sources that read the same url with the same
// format, and that take no transforms, into the first of them. The
// sources after a merged one take the numbers one less.
func (c *compiler) mergeSources(out *vega.Spec, read map[string]bool) {
	for {
		merged := false
		seen := map[string]int{}
		for i, d := range c.data {
			if d.URL.IsZero() || len(d.Transform) > 0 || !strings.HasPrefix(d.Name, "source_") {
				continue
			}
			format := ""
			if d.Format != nil {
				f := *d.Format
				f.Parse = vega.Value{}
				format, _ = canonical(f)
			}
			key, _ := canonical(d.URL)
			key += format
			first, ok := seen[key]
			if !ok {
				seen[key] = i
				continue
			}
			// The parse of the merged source joins the parse of the first.
			if d.Format != nil && !d.Format.Parse.IsZero() {
				parse := map[string]any{}
				if f := c.data[first].Format; f != nil && !f.Parse.IsZero() {
					_ = f.Parse.Decode(&parse)
				}
				var more map[string]any
				_ = d.Format.Parse.Decode(&more)
				maps.Copy(parse, more)
				c.data[first].Format.Parse = vega.Lit(parse)
			}
			gone := d.Name
			n, _ := strconv.Atoi(strings.TrimPrefix(gone, "source_"))
			c.data = append(c.data[:i], c.data[i+1:]...)
			names := map[string]string{gone: c.data[first].Name}
			for _, other := range c.data {
				if m, err := strconv.Atoi(strings.TrimPrefix(other.Name, "source_")); err == nil && strings.HasPrefix(other.Name, "source_") && m > n {
					names[other.Name] = "source_" + strconv.Itoa(m-1)
				}
			}
			c.nSource--
			c.renameAll(out, read, names)
			if c.shared == nil {
				c.shared = map[string]bool{}
			}
			c.shared[c.data[first].Name] = true
			merged = true
			break
		}
		if !merged {
			return
		}
	}
}

// liftFlattens moves the formulas that flatten fields inside objects, at
// the start of the datasets that derive from a source of a url, into the
// source, once each, as vega-lite.js parses its data there.
func (c *compiler) liftFlattens(read map[string]bool) {
	if len(c.flattens) == 0 {
		return
	}
	isFlatten := func(t vega.Transform) bool {
		if t.Type != "formula" {
			return false
		}
		var e string
		_ = json.Unmarshal(t.Params["expr"], &e)
		return slices.Contains(c.flattens, e)
	}
	for pi := range c.data {
		if c.data[pi].URL.IsZero() {
			continue
		}
		have := map[string]bool{}
		for _, t := range c.data[pi].Transform {
			have[string(t.Params["expr"])] = true
		}
		for k := range c.data {
			if len(c.data[k].Source.List) != 1 || c.data[k].Source.List[0] != c.data[pi].Name {
				continue
			}
			for len(c.data[k].Transform) > 0 && isFlatten(c.data[k].Transform[0]) {
				t := c.data[k].Transform[0]
				if key := string(t.Params["expr"]); !have[key] {
					have[key] = true
					c.data[pi].Transform = append(c.data[pi].Transform, t)
				}
				c.data[k].Transform = c.data[k].Transform[1:]
			}
		}
	}
	c.removeEmpty(read)
}

// liftBins moves the extents and the bins that start the datasets derived
// from a dataset into that dataset, when two or more of them start so, as
// vega-lite.js does: those of the last dataset first.
func (c *compiler) liftBins(out *vega.Spec, read map[string]bool) {
	children := map[string][]int{}
	for i, d := range c.data {
		if len(d.Source.List) == 1 {
			children[d.Source.List[0]] = append(children[d.Source.List[0]], i)
		}
	}
	startsWithBin := func(ts []vega.Transform) bool {
		return len(ts) >= 2 && ts[0].Type == "extent" && ts[1].Type == "bin"
	}
	for pi, p := range c.data {
		var kids []int
		for _, k := range children[p.Name] {
			if startsWithBin(c.data[k].Transform) {
				kids = append(kids, k)
			}
		}
		if len(kids) < 2 || len(kids) != len(children[p.Name]) {
			continue
		}
		order := append([]int{kids[len(kids)-1]}, kids[:len(kids)-1]...)
		for _, k := range order {
			for startsWithBin(c.data[k].Transform) {
				c.data[pi].Transform = append(c.data[pi].Transform, c.data[k].Transform[:2]...)
				c.data[k].Transform = c.data[k].Transform[2:]
			}
		}
		// The datasets take their numbers in the same order.
		var numbers []int
		for _, k := range kids {
			if n, ok := dataNumber(c.data[k].Name); ok {
				numbers = append(numbers, n)
			}
		}
		if len(numbers) == len(kids) {
			slices.Sort(numbers)
			names := map[string]string{}
			for i, k := range order {
				names[c.data[k].Name] = "data_" + strconv.Itoa(numbers[i])
			}
			c.renameAll(out, read, names)
		}
	}
	c.removeEmpty(read)
}

// release frees the number that a layer over a dataset took when the
// datasets of its layers merged into one: the datasets numbered after it
// take the numbers one less.
func (c *compiler) release(out *vega.Spec, read map[string]bool) {
	for _, r := range slices.Backward(c.reserved) {
		kids := 0
		for _, d := range c.data {
			if len(d.Source.List) == 1 && d.Source.List[0] == r.data {
				kids++
			}
		}
		if kids >= 2 {
			continue
		}
		names := map[string]string{}
		for _, d := range c.data {
			if m, ok := dataNumber(d.Name); ok && m > r.number {
				names[d.Name] = "data_" + strconv.Itoa(m-1)
			}
		}
		c.nData--
		c.renameAll(out, read, names)
	}
	c.reserved = nil
}

// dataNumber returns the number of a dataset named data_N.
func dataNumber(name string) (int, bool) {
	s, ok := strings.CutPrefix(name, "data_")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// renameMarkData renames the datasets that a mark and its marks read.
func renameMarkData(m *vega.Mark, rename func(string) string) {
	if m.From != nil {
		m.From.Data = rename(m.From.Data)
		if m.From.Facet != nil {
			m.From.Facet.Data = rename(m.From.Facet.Data)
		}
	}
	if len(m.Data) > 0 {
		// A group that defines a dataset hides the dataset of the same
		// name outside it.
		local := map[string]bool{}
		if m.From != nil && m.From.Facet != nil {
			local[m.From.Facet.Name] = true
		}
		for _, d := range m.Data {
			local[d.Name] = true
		}
		outer := rename
		rename = func(n string) string {
			if local[n] {
				return n
			}
			return outer(n)
		}
	}
	for i := range m.Scales {
		renameDomain(m.Scales[i].Domain.Data, rename)
	}
	for i := range m.Marks {
		renameMarkData(&m.Marks[i], rename)
	}
}

// renameDomain renames the datasets of the domain of a scale.
func renameDomain(d *vega.DataRef, rename func(string) string) {
	if d == nil {
		return
	}
	d.Data = rename(d.Data)
	for i, f := range d.Fields {
		if m, ok := f.(map[string]any); ok {
			m["data"] = rename(fmt.Sprint(m["data"]))
			d.Fields[i] = m
		}
	}
}

// shiftedUnits reports whether a list of transforms starts with the end of
// a time unit that the shift of a bar follows. vega-lite.js makes the two
// in one step, which does not merge with the end of a time unit alone.
func shiftedUnits(ts []vega.Transform) bool {
	if len(ts) < 2 || ts[1].Type != "formula" {
		return false
	}
	var as string
	return json.Unmarshal(ts[1].Params["as"], &as) == nil && strings.HasSuffix(as, "_offsetted_rect_start")
}
