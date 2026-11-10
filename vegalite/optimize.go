package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/xo/zuzu/vega"
)

// mergeIdentical merges the datasets that derive from the same dataset
// with the same transforms into one, as vega-lite.js does. Then a dataset
// that has one dataset derived from it, and that nothing else reads, takes
// the transforms of that dataset. A dataset that goes frees its number.
func (c *compiler) mergeIdentical(out *vega.Spec, read map[string]bool) {
	for {
		if !c.mergeOnce(out, read) {
			return
		}
	}
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
			if invalidFilter(c.data[k].Transform) {
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
	}
	for j := range out.Marks {
		renameMarkData(&out.Marks[j], rename)
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
