// Package k8s bundles the icons of Kubernetes, from the pack of Iconify of
// 2026-10-20, under Apache-2.0, as a pack of icons for the diagrams of zuzu
// (D31). It leaves out the logo of Kubernetes, which is a trademark. The
// file LICENSE holds the license of the pack, and k8s.json holds the pack in
// the JSON format of Iconify.
package k8s

import (
	_ "embed"
	"sync"

	"github.com/xo/zuzu/icon"
)

//go:embed k8s.json
var data []byte

// pack reads the pack once.
var pack = sync.OnceValue(func() *icon.Pack {
	p, err := icon.Read(data)
	if err != nil {
		panic("the pack k8s does not read: " + err.Error())
	}
	return p
})

// Pack returns the pack. Give it to a render with zuzu.WithIcons, under the
// name that a diagram uses for it.
func Pack() *icon.Pack {
	return pack()
}
