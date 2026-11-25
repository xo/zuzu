package layout

import (
	"math"
	"testing"

	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
)

// TestRuleEndsKeepAnEndAtZero checks that a rule that the layout builds
// with an end at 0 puts that end first, so that shape.RuleEnd, which reads
// an x2 or a y2 of 0 as an end that the item does not set, keeps its
// length.
func TestRuleEndsKeepAnEndAtZero(t *testing.T) {
	t.Parallel()
	var it scene.Item
	ruleEnds(&it, 40, 10, 0, 10)
	if x2, y2 := shape.RuleEnd(&it); math.Abs(x2-it.X) != 40 || y2 != 10 {
		t.Errorf("the rule runs from %v, %v to %v, %v, want a length of 40", it.X, it.Y, x2, y2)
	}
}
