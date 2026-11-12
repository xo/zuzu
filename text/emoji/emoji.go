// Package emoji bundles Noto Color Emoji, version 2.057, under the SIL Open
// Font License 1.1, so that zuzu draws an emoji in color (D32). The file
// LICENSE.txt holds the license, and README.md names the source.
//
// The font is the CBDT build, NotoColorEmoji.ttf, of 10.7 MB. Only a program
// that imports this package holds it. Give it to a render with
// zuzu.WithEmoji:
//
//	img, report, err := c.Image(ctx, zuzu.WithEmoji(emoji.Font()))
package emoji

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/text"
)

// Family is the family of the font.
const Family = text.EmojiFamily

//go:embed NotoColorEmoji.ttf
var data []byte

// font makes the fallback once, so that every render shares its parsed
// font.
var font = sync.OnceValue(func() *text.Fallback {
	return text.NewFallback(Family, data)
})

// Font returns the font, as a fallback for the characters that IBM Plex
// Sans does not have.
func Font() *text.Fallback {
	return font()
}
