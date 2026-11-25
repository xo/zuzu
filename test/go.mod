module github.com/xo/zuzu/test

go 1.27.3

require (
	github.com/xo/okage v0.20.1
	github.com/xo/transit/styles v0.7.8
	github.com/xo/zuzu v0.8.0
	github.com/xo/zuzu/elk v0.0.0
)

require golang.org/x/image v0.48.0 // indirect

replace github.com/xo/zuzu => ../

// The review page and the position error draw with the engine of ELK of
// the module in ../elk (D44 part 4, D45). The test module is never
// imported by a program, so no program ships the code under the EPL-2.0
// through it.
replace github.com/xo/zuzu/elk => ../elk
