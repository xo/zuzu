// Package mdi bundles Material Design Icons of Pictogrammers, from the pack
// of Iconify of 2025-01-21, under Apache-2.0, as a pack of icons for the
// diagrams of zuzu (D31). The file LICENSE holds the license of the pack,
// and mdi.json holds the pack in the JSON format of Iconify.
package mdi

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/icon"
)

//go:embed mdi.json
var data []byte

// pack reads the pack once.
var pack = sync.OnceValue(func() *icon.Pack {
	p, err := icon.Read(data)
	if err != nil {
		panic("the pack mdi does not read: " + err.Error())
	}
	return p
})

// Pack returns the pack. Give it to a render with zuzu.WithIcons, under the
// name that a diagram uses for it.
func Pack() *icon.Pack {
	return pack()
}
