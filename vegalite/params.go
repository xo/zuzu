package vegalite

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/xo/zuzu/vega"
)

// param is a parameter of a spec: a value or an expression that the spec
// names, or a selection.
type param struct {
	Name   string         `json:"name"`
	Value  jsontext.Value `json:"value,omitzero"`
	Expr   string         `json:"expr,omitzero"`
	Bind   jsontext.Value `json:"bind,omitzero"`
	Select jsontext.Value `json:"select,omitzero"`
}

// params adds a signal for each parameter of a spec that has a value or an
// expression. A selection, and a parameter that an input changes, are not
// part of a static chart (D7). A parameter bound to an input keeps its
// value.
func (c *compiler) params(s *Spec, path string) error {
	if len(s.Params) == 0 {
		return nil
	}
	var list []param
	if err := json.Unmarshal(s.Params, &list); err != nil {
		return err
	}
	for i, p := range list {
		at := fmt.Sprintf("%s/params/%d", path, i)
		if len(p.Select) > 0 {
			if err := c.issue(at, "the selection %s", p.Name); err != nil {
				return err
			}
			continue
		}
		if len(p.Bind) > 0 {
			if err := c.issue(at+"/bind", "the input of the parameter %s", p.Name); err != nil {
				return err
			}
		}
		if c.signalNames[p.Name] {
			continue
		}
		if c.signalNames == nil {
			c.signalNames = map[string]bool{}
		}
		c.signalNames[p.Name] = true
		sig := vega.Signal{Name: p.Name, Value: p.Value}
		if p.Expr != "" {
			sig = vega.Signal{Name: p.Name, Update: vega.Expr(p.Expr)}
		}
		c.signals = append(c.signals, sig)
	}
	return nil
}
