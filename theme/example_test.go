package theme_test

import (
	"fmt"

	"github.com/xo/zuzu/theme"
)

func ExampleNew() {
	t := theme.New(theme.Dark, theme.Ember)
	fmt.Println(t.Mode(), t.Background, t.Category[0])
	// Output: dark #10141b #f05b21
}

func ExampleSchemes() {
	fmt.Println(theme.Schemes()[:4])
	// Output: [zuzu pastel harbor ember]
}

func ExampleLookup() {
	s, ok := theme.Lookup("harbor")
	fmt.Println(s.Name(), ok)
	_, ok = theme.Lookup("nothing")
	fmt.Println(ok)
	// Output:
	// harbor true
	// false
}
