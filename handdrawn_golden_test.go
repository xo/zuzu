//go:build amd64 && !amd64.v3

package zuzu

// The golden files of the look like Excalidraw hold the output of amd64
// below the level v3, so this test runs only there. The copy of rough.js
// calls Sin and Cos of the package math of Go, which give other bits on
// another machine, so the bytes differ there. zuzu promises the same bytes
// on one architecture, not across machines (D41).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
)

// TestLookGolden draws each source of testdata/look in the look like
// Excalidraw, with Excalifont, in the light and the dark mode, and compares
// the SVG with its golden file, such as testdata/look/bar-light.svg. The
// SVG embeds Excalifont (D43), and the golden file holds the SHA-256 of the
// font in place of its bytes, so that each file stays small.
func TestLookGolden(t *testing.T) {
	t.Parallel()
	for _, name := range lookCases {
		c := lookChart(t, name)
		base := name[:strings.IndexByte(name, '.')]
		for _, mode := range []theme.Mode{theme.Light, theme.Dark} {
			got := fontDigests(t, lookSVG(t, c, mode))
			golden := "testdata/look/" + base + "-" + mode.String() + ".svg"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("the SVG differs from %s: run go test -run TestLookGolden -update and look at it", golden)
			}
		}
	}
}

// fontData matches the bytes of a font that the SVG embeds in a data URL.
var fontData = regexp.MustCompile(`base64,([A-Za-z0-9+/=]+)\) format`)

// fontDigests puts the SHA-256 of each font that an SVG embeds in place of
// its bytes.
func fontDigests(t *testing.T, svg []byte) []byte {
	t.Helper()
	return fontData.ReplaceAllFunc(svg, func(m []byte) []byte {
		data := fontData.FindSubmatch(m)[1]
		b, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		return []byte("sha256," + hex.EncodeToString(sum[:]) + ") format")
	})
}
