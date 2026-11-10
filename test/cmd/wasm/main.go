// Command wasm imports the packages of zuzu and draws one small scene, so
// that CI can build them with TinyGo for WebAssembly (D29). TinyGo builds a
// program and not a library, so this program exists to be built and run.
package main

import (
	"fmt"
	"os"

	_ "github.com/xo/zuzu"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/theme"
)

func main() {
	th := theme.New(theme.Dark, theme.Zuzu)
	s := &scene.Scene{
		Width:      100,
		Height:     60,
		Background: scene.Paint{Color: th.Background},
		Root: scene.Mark{Type: scene.Rect, Items: []scene.Item{
			{X: 10, Y: 10, Width: 80, Height: 40, Fill: scene.Paint{Color: th.Category[0]}, CornerRadius: 4},
		}},
	}
	if err := svg.Write(os.Stdout, s); err != nil {
		fmt.Fprintln(os.Stderr, "wasm:", err)
		os.Exit(1)
	}
}
