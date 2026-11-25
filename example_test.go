package zuzu_test

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"log"
	"strings"
	"testing/fstest"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/text/excalifont"
	"github.com/xo/zuzu/theme"
)

// The examples in this file hold the code of README.md.
// TestTheReadmeCodeIsAnExample checks that each block of Go code in README.md
// is code of this file, and TestTheReadmeImagesAreDrawn draws the images of
// README.md from the sources here.

func ExampleParse() {
	src := []byte(`{
  "$schema": "https://vega.github.io/schema/vega-lite/v6.json",
  "data": {"values": [{"a": "x", "b": 1}, {"a": "y", "b": 2}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "a", "type": "nominal"},
    "y": {"field": "b", "type": "quantitative"}
  }
}`)
	chart, report, err := zuzu.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(chart.Spec().Marks) > 0, len(report))
	// Output: true 0
}

func ExampleChart_Image() {
	ctx := context.Background()
	src := []byte(`{
  "$schema": "https://vega.github.io/schema/vega-lite/v6.json",
  "width": 320,
  "height": 180,
  "data": {"values": [
    {"fruit": "Apples", "sold": 28},
    {"fruit": "Bananas", "sold": 55},
    {"fruit": "Cherries", "sold": 43},
    {"fruit": "Dates", "sold": 91},
    {"fruit": "Figs", "sold": 81}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "fruit", "type": "nominal", "title": null, "axis": {"labelAngle": 0}},
    "y": {"field": "sold", "type": "quantitative"},
    "color": {"field": "fruit", "legend": null}
  }
}`)
	chart, _, err := zuzu.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, issue := range report {
		log.Println(issue)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy(), len(report))
	// Output: 358 231 0
}

func ExampleChart_SVG() {
	ctx := context.Background()
	chart, _, err := zuzu.Parse([]byte("flowchart LR\n  A --> B"))
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	report, err := chart.SVG(ctx, &buf)
	if err != nil {
		log.Fatal(err)
	}
	head, _, _ := strings.Cut(buf.String(), " font-family")
	fmt.Println(head)
	fmt.Println(len(report))
	// Output:
	// <svg xmlns="http://www.w3.org/2000/svg" width="277" height="47" viewBox="0 0 277 47"
	// 0
}

func ExampleParse_mermaid() {
	ctx := context.Background()
	src := []byte(`flowchart TD
  A[Spec] --> B{Vega-Lite?}
  B -- yes --> C[Compile]
  B -- no --> D[Run]
  C --> D
  D --> E[(Scene)]`)
	chart, _, err := zuzu.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy(), len(report))
	// Output: 174 446 0
}

func ExampleParse_dot() {
	ctx := context.Background()
	src := []byte(`digraph {
  rankdir=LR
  parse -> compile -> run -> scene
  scene -> svg
  scene -> image
}`)
	chart, _, err := zuzu.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy(), len(report))
	// Output: 465 119 0
}

func ExampleWithMode() {
	ctx := context.Background()
	chart, _, err := zuzu.Parse([]byte("flowchart LR\n  A --> B"))
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx, zuzu.WithMode(theme.Dark))
	if err != nil {
		log.Fatal(err)
	}
	bg := img.RGBAAt(0, 0)
	fmt.Printf("#%02x%02x%02x %d\n", bg.R, bg.G, bg.B, len(report))
	// Output: #10141b 0
}

func ExampleWithScheme() {
	ctx := context.Background()
	chart, _, err := zuzu.Parse([]byte("flowchart LR\n  A --> B"))
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx,
		zuzu.WithMode(theme.Dark),
		zuzu.WithScheme(theme.Ember),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy(), len(report))
	// Output: 277 47 0
}

func ExampleWithLook() {
	ctx := context.Background()
	src := []byte(`flowchart TD
  A[Spec] --> B{Vega-Lite?}
  B -- yes --> C[Compile]
  B -- no --> D[Run]
  C --> D
  D --> E[(Scene)]`)
	look := zuzu.WithLook(theme.LookExcalidraw)
	font := zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())
	chart, _, err := zuzu.Parse(src, look, font)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	report, err := chart.SVG(ctx, &buf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(strings.Contains(buf.String(), excalifont.Family), len(report))
	// Output: true 0
}

func ExampleWithLoader() {
	ctx := context.Background()
	src := []byte(`{
  "$schema": "https://vega.github.io/schema/vega-lite/v6.json",
  "width": 320,
  "height": 180,
  "data": {"url": "temps.csv"},
  "mark": {"type": "line", "point": true},
  "encoding": {
    "x": {"field": "month", "type": "ordinal", "sort": null, "title": null},
    "y": {"field": "temp", "type": "quantitative", "title": "°C"},
    "color": {"field": "city", "title": null}
  }
}`)
	// Any fs.FS works, such as os.DirFS("data").
	fsys := fstest.MapFS{"temps.csv": {Data: []byte(`month,city,temp
Jan,Oslo,-4
Feb,Oslo,-4
Mar,Oslo,0
Apr,Oslo,5
May,Oslo,11
Jun,Oslo,15
Jan,Rome,8
Feb,Rome,9
Mar,Rome,11
Apr,Rome,14
May,Rome,18
Jun,Rome,22
`)}}
	chart, _, err := zuzu.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	img, report, err := chart.Image(ctx, zuzu.WithLoader(zuzu.FSLoader(fsys)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy(), len(report))
	// Output: 361 241 0
}
