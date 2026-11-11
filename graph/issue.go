package graph

import (
	"fmt"

	"github.com/xo/zuzu/vega"
)

// Issues collects the parts of a source that zuzu does not draw: an error
// in the strict mode, and a line of the report in the loose mode (D26).
type Issues struct {
	Strict bool
	Report vega.Report
	seen   map[string]bool
}

// Add records a part that zuzu does not draw, once for each message. In the
// strict mode it returns the issue as an error.
func (is *Issues) Add(path, format string, args ...any) error {
	i := vega.Issue{Err: vega.ErrUnsupported, Path: path, Message: fmt.Sprintf(format, args...)}
	if is.Strict {
		return i
	}
	if is.seen == nil {
		is.seen = map[string]bool{}
	}
	if !is.seen[i.Message] {
		is.seen[i.Message] = true
		is.Report = append(is.Report, i)
	}
	return nil
}
