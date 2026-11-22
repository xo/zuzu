package shape

import (
	"math"
	"testing"

	"github.com/xo/zuzu/scene"
)

// TestTextOffsetsTurnWithTheAngle checks that the shift of the baseline and
// the dx and the dy of a text move it in its own frame, after the turn of
// its angle, as in Vega. A label turned to 270 with the baseline middle
// moves to the left of its point, so that it centers on it, and not down.
func TestTextOffsetsTurnWithTheAngle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		it   scene.Item
		x, y float64
	}{
		{"flat", scene.Item{X: 10, Y: 20, FontSize: 10, Baseline: "middle", Dx: 2, Dy: 1}, 12, 24},
		{"turned", scene.Item{X: 10, Y: 20, FontSize: 10, Baseline: "middle", Angle: 270}, 13, 20},
		{"turned with dx", scene.Item{X: 10, Y: 20, FontSize: 10, Angle: 90, Dx: 5}, 10, 25},
	}
	for _, tt := range tests {
		tt.it.Text = []string{"C"}
		got := TextOf(&tt.it, nil)
		if math.Abs(got.X-tt.x) > 1e-9 || math.Abs(got.Y-tt.y) > 1e-9 {
			t.Errorf("%s: the text is at %v,%v, want %v,%v", tt.name, got.X, got.Y, tt.x, tt.y)
		}
	}
}
