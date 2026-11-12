package graph

import (
	"strings"
	"unicode"
)

// FontFace reads a weight and a style from the name of a font, such as
// Helvetica-Bold, Times-BoldItalic, "Arial Light" or Helvetica:bold, and
// returns them as Vega writes them. zuzu draws in IBM Plex Sans only, so
// the rest of the name changes nothing (D16, D32). The weight is empty
// when the name gives none, and a weight of semibold or more is
// BoldWeight, the heaviest weight that zuzu bundles. The style is italic
// for Italic and for Oblique, because the bundled font slants only as
// italic, and empty otherwise.
func FontFace(name string) (weight, style string) {
	words := fontWords(name)
	for i, w := range words {
		prev := ""
		if i > 0 {
			prev = words[i-1]
		}
		switch w {
		case "italic", "oblique", "it", "slanted":
			style = "italic"
		case "bolditalic", "boldoblique":
			weight, style = BoldWeight, "italic"
		case "thin", "hairline":
			weight = "100"
		case "extralight", "ultralight":
			weight = "200"
		case "light":
			weight = "300"
			if prev == "extra" || prev == "ultra" {
				weight = "200"
			}
		case "medium":
			weight = "500"
		case "semibold", "demibold", "demi", "bold", "extrabold", "ultrabold", "heavy", "black":
			weight = BoldWeight
		}
	}
	return weight, style
}

// fontWords splits the name of a font into words, in lower case: at a
// space, a dash, an underscore, a comma, a colon or an equals sign, and
// where a lower case letter meets an upper case one, as in BoldItalic.
func fontWords(name string) []string {
	var words []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			words = append(words, strings.ToLower(b.String()))
			b.Reset()
		}
	}
	var last rune
	for _, r := range name {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(last):
			flush()
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
		last = r
	}
	flush()
	return words
}
