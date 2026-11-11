// Package fa bundles the solid icons of Font Awesome Free, version 7.3.1,
// under CC BY 4.0, as a pack of icons for the diagrams of zuzu (D31). It
// holds no regular icon and no brand. The file LICENSE holds the license of
// the pack, and fa.json holds the pack in the JSON format of Iconify.
//
// The flowcharts of Mermaid name these icons as fa:name and fas:name, so
// give the pack under both names. CC BY 4.0 asks the one who shares an image
// of an icon for the credit. Each icon of the pack carries Credit, and the
// SVG writer names it in the metadata of a chart that draws one.
package fa

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/icon"
)

// Credit is the attribution that CC BY 4.0 asks for.
const Credit = "Icons: Font Awesome Free 7.3.1 by Fonticons, Inc., https://fontawesome.com, under CC BY 4.0, https://creativecommons.org/licenses/by/4.0/" //nolint:gosec // A credit, not a secret.

//go:embed fa.json
var data []byte

// pack reads the pack once.
var pack = sync.OnceValue(func() *icon.Pack {
	p, err := icon.Read(data)
	if err != nil {
		panic("the pack fa does not read: " + err.Error())
	}
	p.Credit = Credit
	return p
})

// Pack returns the pack. Give it to a render with zuzu.WithIcons, under the
// name that a diagram uses for it.
func Pack() *icon.Pack {
	return pack()
}
