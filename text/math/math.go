// Package math gives STIX Two Math, a font of math under the SIL Open Font
// License 1.1, so that zuzu draws the math of KaTeX in a label of Mermaid
// (D36). The font comes from the package texmath/stixtwomath of okage, with
// its license in OFL.txt beside it. Only a program that imports this
// package holds the font, about 819 KiB. Give it to a parse with
// zuzu.WithMath:
//
//	c, report, err := zuzu.Parse(src, zuzu.WithMath(math.Font()))
package math

import (
	"sync"

	"github.com/xo/okage/texmath/stixtwomath"

	"github.com/xo/zuzu/text"
)

// font makes the font once, so that every chart shares its parsed face.
var font = sync.OnceValue(func() *text.MathFont {
	return text.NewMathFont(stixtwomath.Data())
})

// Font returns STIX Two Math.
func Font() *text.MathFont {
	return font()
}
