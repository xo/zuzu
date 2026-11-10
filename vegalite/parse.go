package vegalite

import (
	"bytes"
	"encoding/json/v2"
	"fmt"

	"github.com/xo/zuzu/vega"
)

// Parse reads a Vega-Lite spec. It checks that the JSON is well formed and
// that each field has a type that the grammar allows.
func Parse(src []byte) (*Spec, error) {
	var s Spec
	if err := json.Unmarshal(src, &s); err != nil {
		return nil, fmt.Errorf("parsing a vega-lite spec: %w: %w", vega.ErrInvalidSpec, err)
	}
	return &s, nil
}

// Write writes a spec as JSON. The same spec gives the same bytes each
// time.
func Write(s *Spec) ([]byte, error) {
	var b bytes.Buffer
	if err := json.MarshalWrite(&b, s, json.Deterministic(true)); err != nil {
		return nil, fmt.Errorf("writing a vega-lite spec: %w", err)
	}
	return b.Bytes(), nil
}
