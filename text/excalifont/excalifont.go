// Package excalifont bundles Excalifont, version 1.000, under the SIL Open
// Font License 1.1, for the look like Excalidraw (D40). The file
// LICENSE.txt holds the license, and README.md names the source.
//
// The font is the file Excalifont-Regular.woff2 of Excalidraw, of 52 KB,
// unchanged. It has one face, of weight 400 and upright, with the
// characters of Latin, Greek and Cyrillic. Only a program that imports
// this package holds it. A caller gives it to zuzu with zuzu.WithLookFont,
// and without it the look like Excalidraw draws its text in IBM Plex Sans
// (D40):
//
//	font := zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())
//	c, report, err := zuzu.Parse(src, zuzu.WithLook(theme.LookExcalidraw), font)
//	img, report, err := c.Image(ctx, font)
package excalifont

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/text"
)

// Family is the family of the font.
const Family = "Excalifont"

//go:embed Excalifont-Regular.woff2
var data []byte

// font makes the font once, so that every render shares its decoded and
// parsed font.
var font = sync.OnceValue(func() *text.Fallback {
	return text.NewFallback(Family, data)
})

// Font returns Excalifont. It decodes the font from WOFF2 on its first
// use, and Data of the font gives the TrueType bytes that a renderer of
// okage loads (D34).
func Font() *text.Fallback {
	return font()
}
