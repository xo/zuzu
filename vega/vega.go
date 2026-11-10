// Package vega holds the Go types of a Vega spec, which it reads from JSON
// and writes back to JSON (D6, D21). Parse checks the structure of a spec,
// and not whether zuzu supports each feature, which the root package
// checks.
package vega

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
)

// Error is an error of a spec.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of a spec. The packages vegalite and zuzu share them.
const (
	// ErrInvalidSpec is the error of a spec that does not follow the grammar.
	ErrInvalidSpec Error = "invalid spec"
	// ErrUnsupported is the error of a feature that zuzu does not support.
	// Each error of an unsupported feature below unwraps to it, in an Issue.
	ErrUnsupported Error = "unsupported feature"
	// ErrUnsupportedMark is the error of a mark type that zuzu does not draw.
	ErrUnsupportedMark Error = "unsupported mark"
	// ErrUnsupportedTransform is the error of a transform that zuzu does not
	// run.
	ErrUnsupportedTransform Error = "unsupported transform"
	// ErrUnsupportedFunction is the error of an expression function that zuzu
	// does not have.
	ErrUnsupportedFunction Error = "unsupported expression function"
	// ErrUnknownProperty is the error of a property that the grammar does not
	// have, or that zuzu does not know.
	ErrUnknownProperty Error = "unknown property"
)

// Issue is one feature of a spec that zuzu did not use. It is an error. It
// unwraps to Err, and to ErrUnsupported when Err is one of the errors of an
// unsupported feature.
type Issue struct { //nolint:errname // The API names it Issue, as an issue of a report (D24).
	// Err is the sentinel error of the issue.
	Err error
	// Path is where the feature is in the spec, such as marks[2].encode.
	Path string
	// Message says what the feature is.
	Message string
}

// Error satisfies the error interface.
func (i Issue) Error() string {
	var b strings.Builder
	if i.Path != "" {
		b.WriteString(i.Path)
		b.WriteString(": ")
	}
	if i.Message != "" {
		b.WriteString(i.Message)
		b.WriteString(": ")
	}
	b.WriteString(i.Err.Error())
	return b.String()
}

// Unwrap returns the sentinel errors of the issue.
func (i Issue) Unwrap() []error {
	errs := []error{i.Err}
	switch {
	case errors.Is(i.Err, ErrUnsupportedMark), errors.Is(i.Err, ErrUnsupportedTransform),
		errors.Is(i.Err, ErrUnsupportedFunction):
		errs = append(errs, ErrUnsupported)
	}
	return errs
}

// Report holds the issues of a spec, in the order zuzu found them (D26).
type Report []Issue

// Addf adds an issue, with a message that format and args make.
func (r *Report) Addf(err error, path, format string, args ...any) {
	*r = append(*r, Issue{Err: err, Path: path, Message: fmt.Sprintf(format, args...)})
}

// Parse reads a Vega spec. It checks that the JSON is well formed and that
// each field has a type that the grammar allows. A property that the types
// do not know goes in the Extra field of its object.
func Parse(src []byte) (*Spec, error) {
	var s Spec
	if err := json.Unmarshal(src, &s); err != nil {
		return nil, fmt.Errorf("parsing a vega spec: %w: %w", ErrInvalidSpec, err)
	}
	return &s, nil
}

// Write writes a spec as JSON. The same spec gives the same bytes each time.
func Write(s *Spec) ([]byte, error) {
	var b bytes.Buffer
	if err := json.MarshalWrite(&b, s, json.Deterministic(true)); err != nil {
		return nil, fmt.Errorf("writing a vega spec: %w", err)
	}
	return b.Bytes(), nil
}

// readValue reads the next JSON value whole.
func readValue(dec *jsontext.Decoder) (jsontext.Value, error) {
	v, err := dec.ReadValue()
	if err != nil {
		return nil, err
	}
	return v.Clone(), nil
}
