package main

import (
	"encoding/base64"
	"testing"
)

// TestFlatten checks that flatten adds the translate of a nested root to
// the places inside it, and to those of a nested root inside it, and
// leaves the places inside a node and outside the root as they are.
func TestFlatten(t *testing.T) {
	t.Parallel()
	points := base64.StdEncoding.EncodeToString([]byte(`[{"x":1,"y":2}]`))
	src := `<g class="root"><g class="cluster" id="my-svg-a"><rect x="8" y="8" width="10" height="10"/></g>` +
		`<g class="root" transform="translate(100, 50)">` +
		`<g class="cluster" id="my-svg-b"><rect x="8" y="8" width="10" height="10"/></g>` +
		`<path data-id="L_x_y_0" data-points="` + points + `"/>` +
		`<g class="node" id="my-svg-flowchart-x-1" transform="translate(20, 30)"><rect x="-5" y="-5" width="10" height="10"/></g>` +
		`<g class="root" transform="translate(1, 2)"><g class="node" id="my-svg-flowchart-y-2" transform="translate(3, 4)"></g></g>` +
		`</g>` +
		`<g class="node" id="my-svg-flowchart-z-3" transform="translate(5, 6)"></g></g>`
	moved := base64.StdEncoding.EncodeToString([]byte(`[{"x":101,"y":52}]`))
	want := `<g class="root"><g class="cluster" id="my-svg-a"><rect x="8" y="8" width="10" height="10"/></g>` +
		`<g class="root">` +
		`<g class="cluster" id="my-svg-b"><rect x="108" y="58" width="10" height="10"/></g>` +
		`<path data-id="L_x_y_0" data-points="` + moved + `"/>` +
		`<g class="node" id="my-svg-flowchart-x-1" transform="translate(120, 80)"><rect x="-5" y="-5" width="10" height="10"/></g>` +
		`<g class="root"><g class="node" id="my-svg-flowchart-y-2" transform="translate(104, 56)"></g></g>` +
		`</g>` +
		`<g class="node" id="my-svg-flowchart-z-3" transform="translate(5, 6)"></g></g>`
	if got := flatten(src); got != want {
		t.Errorf("expected\n%s\ngot\n%s", want, got)
	}
}
