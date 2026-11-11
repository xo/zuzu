// Package carbon bundles the icons of Carbon, the design system of IBM,
// version 11.92.0, under Apache-2.0, as a pack of icons for the diagrams of
// zuzu (D31). The icons suit IBM Plex, the font of zuzu. The file LICENSE
// holds the license of the pack, and carbon.json holds the pack in the JSON
// format of Iconify.
package carbon

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/icon"
)

//go:embed carbon.json
var data []byte

// pack reads the pack once.
var pack = sync.OnceValue(func() *icon.Pack {
	p, err := icon.Read(data)
	if err != nil {
		panic("the pack carbon does not read: " + err.Error())
	}
	return p
})

// Pack returns the pack. Give it to a render with zuzu.WithIcons, under the
// name that a diagram uses for it.
func Pack() *icon.Pack {
	return pack()
}
