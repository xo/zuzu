package icon_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
)

func TestPacks(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]*icon.Pack{"carbon": carbon.Pack(), "fa": fa.Pack(), "mdi": mdi.Pack(), "k8s": k8s.Pack()} {
		shapes, images := 0, 0
		for _, n := range p.Names() {
			ic, ok := p.Icon(n)
			if !ok {
				t.Errorf("%s: %s does not resolve", name, n)
				continue
			}
			vb, _ := ic.Render()
			if s, ok := ic.Shape(); ok {
				if _, _, err := s.Scale(vb, 24); err != nil {
					t.Errorf("%s: %s: %v", name, n, err)
				}
				shapes++
				continue
			}
			if _, err := ic.SVG("#123456", 24, 24); err != nil {
				t.Errorf("%s: %s: %v", name, n, err)
			}
			images++
		}
		t.Logf("%s: %d paths, %d images", name, shapes, images)
	}
}

func TestBuiltins(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"cloud", "database", "disk", "internet", "server"} {
		ic, ok := icon.Builtin(n)
		if !ok {
			t.Fatalf("no %s", n)
		}
		s, ok := ic.Shape()
		if !ok || !s.Stroke || s.StrokeWidth != 1.5 {
			t.Errorf("%s: %+v %v", n, s, ok)
		}
	}
	if s, ok := icon.Missing().Shape(); !ok || !s.Stroke {
		t.Error("the missing icon is not a shape")
	}
}

func TestClean(t *testing.T) {
	t.Parallel()
	src := `<?xml version="1.0"?><!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10" onload="x()">` +
		`<script>alert(1)</script><style>@import url(http://a/b.css); .a{fill:url(http://x/y)} .b{fill:url(#g)}</style>` +
		`<foreignObject><div>x</div></foreignObject><image xlink:href="http://a/b.png"/><use href="#p"/>` +
		`<path id="p" d="M0 0h1" fill="url('https://e/f')" onclick="y()"/></svg>`
	b, err := icon.Clean([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"script", "alert", "onload", "onclick", "foreignObject", "http://a", "https://e", "@import", "http://x"} {
		if strings.Contains(s, bad) {
			t.Errorf("%q is left in %s", bad, s)
		}
	}
	for _, good := range []string{`href="#p"`, "url(#g)", `d="M0 0h1"`} {
		if !strings.Contains(s, good) {
			t.Errorf("%q is gone from %s", good, s)
		}
	}
	ic, err := icon.FromSVG([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if ic.Width != 10 || ic.Height != 10 || strings.Contains(ic.Body, "<svg") {
		t.Errorf("FromSVG: %+v", ic)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	p, err := icon.Read([]byte(`{"prefix":"t","width":24,"height":16,"icons":{"a":{"body":"<path d=\"M0 0h1\"/>"}},
		"aliases":{"b":{"parent":"a","rotate":1},"c":{"parent":"b","hFlip":true},"d":{"parent":"a","hFlip":true,"vFlip":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, vb, body string
	}{
		{"a", "[0 0 24 16]", `<path d="M0 0h1"/>`},
		{"b", "[0 0 16 24]", `<g transform="rotate(90 8 8)"><path d="M0 0h1"/></g>`},
		{"c", "[0 0 16 24]", `<g transform="rotate(90 8 8) translate(24 0) scale(-1 1)"><path d="M0 0h1"/></g>`},
		{"d", "[0 0 24 16]", `<g transform="rotate(180 12 8)"><path d="M0 0h1"/></g>`},
	}
	for _, test := range tests {
		ic, ok := p.Icon(test.name)
		if !ok {
			t.Fatalf("no %s", test.name)
		}
		vb, body := ic.Render()
		if s := fmtVB(vb); s != test.vb || body != test.body {
			t.Errorf("%s: expected %s %s, got %s %s", test.name, test.vb, test.body, s, body)
		}
	}
}

func fmtVB(vb [4]float64) string {
	return fmt.Sprint(vb)
}
