package svg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
)

// TestMathPathNamesItsTeX checks that a path of math writes its outline,
// and its TeX as its label, so that a viewer needs no font of math and a
// reader of the screen reads the formula (D36).
func TestMathPathNamesItsTeX(t *testing.T) {
	t.Parallel()
	root := scene.Mark{Type: scene.Group, Items: []scene.Item{{Items: []scene.Mark{
		{Type: scene.Path, Role: scene.RoleMath, Items: []scene.Item{{
			X: 10, Y: 20, Path: "M0,0L4,0L4,-4Z", Fill: scene.Paint{Color: "#1b2230"}, Text: scene.Lines{`x^2 < y`},
		}}},
		{Type: scene.Path, Items: []scene.Item{{Path: "M0,0L1,1", Text: scene.Lines{"not math"}}}},
	}}}}
	var b bytes.Buffer
	if err := Write(&b, &scene.Scene{Width: 50, Height: 50, Root: root}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `d="M0,0L4,0L4,-4Z" role="img" aria-label="x^2 &lt; y"`) {
		t.Errorf("no path of math with its label in\n%s", out)
	}
	if strings.Contains(out, "not math") {
		t.Errorf("a path that is not math writes its text in\n%s", out)
	}
}
