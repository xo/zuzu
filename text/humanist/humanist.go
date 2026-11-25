// Package humanist bundles Inter, version 4.001, a humanist sans serif, under
// the SIL Open Font License 1.1 (D35 part 11, D58 part 1). The file
// LICENSE.txt holds the license, and README.md names the source.
//
// The font is one collection in WOFF2, of 433 KB, with six faces: the
// weights 400, 500 and 600, each upright and italic, as the bundled IBM
// Plex Sans has (D34). Only a program that imports this package holds it.
// A caller gives it to zuzu with zuzu.WithFont, and the chart measures and
// draws its text in it, with IBM Plex Sans for the characters that it does
// not have:
//
//	font := zuzu.WithFont(humanist.Font())
//	c, report, err := zuzu.Parse(src, font)
//	img, report, err := c.Image(ctx, font)
package humanist

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/text"
)

// Family is the family of the font.
const Family = "Inter"

//go:embed Inter.woff2
var data []byte

// font makes the font once, so that every render shares its decoded and
// parsed font.
var font = sync.OnceValue(func() *text.Fallback {
	return text.NewFallback(Family, data)
})

// Font returns Inter. It decodes the collection from WOFF2 on its
// first use, and Data of the font gives the TrueType collection that a
// renderer of okage loads (D34).
func Font() *text.Fallback {
	return font()
}
