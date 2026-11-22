module github.com/xo/zuzu/elk

go 1.27.3

// The engine binds to the interface of a layout engine in the package graph
// of the zuzu in this repository (D44).
require github.com/xo/zuzu v0.8.0

require (
	github.com/xo/okage v0.20.1 // indirect
	golang.org/x/image v0.48.0 // indirect
)

replace github.com/xo/zuzu => ../
