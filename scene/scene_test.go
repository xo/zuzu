package scene

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeReadsAVegaSceneGraph reads a scene graph that vega.js wrote with
// sceneToJSON, and checks the fields that zuzu uses.
func TestDecodeReadsAVegaSceneGraph(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("testdata", "vega-scene.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if s.Root.Type != Group || s.Root.Role != "frame" || len(s.Root.Items) != 1 {
		t.Fatalf("root: got %+v", s.Root)
	}
	marks := s.Root.Items[0].Items
	if len(marks) != 2 || marks[0].Type != Rect || marks[1].Type != Text {
		t.Fatalf("marks: got %d marks", len(marks))
	}
	rect := marks[0].Items[1]
	if rect.X != 50 || rect.Width != 5 || rect.Fill.Color != "red" || rect.CornerRadius != 2 {
		t.Errorf("rect: got %+v", rect)
	}
	text := marks[1].Items[0]
	if len(text.Text) != 2 || text.Text[1] != "b" || text.FontWeight != "600" {
		t.Errorf("text: got %+v", text)
	}
	if _, ok := marks[0].Extra["interactive"]; !ok {
		t.Errorf("the property interactive is not in Extra")
	}
}

// TestEncodeWritesBackWhatDecodeRead checks that a scene written and read
// again gives the same bytes, and that the same scene gives the same bytes
// twice.
func TestEncodeWritesBackWhatDecodeRead(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("testdata", "vega-scene.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := Encode(&first, s); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := Encode(&second, again); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("a scene written twice differs:\n%s\n%s", first.String(), second.String())
	}
}

// TestPaintReadsEachForm reads a color, a gradient and null.
func TestPaintReadsEachForm(t *testing.T) {
	t.Parallel()
	in := `{"width":1,"height":1,"root":{"marktype":"rect","items":[` +
		`{"fill":"#fff"},{"fill":{"gradient":"linear","x2":1,"stops":[{"offset":0,"color":"red"},{"offset":1,"color":"blue"}]}},{"fill":null}]}}`
	s, err := Decode(bytes.NewReader([]byte(in)))
	if err != nil {
		t.Fatal(err)
	}
	items := s.Root.Items
	if items[0].Fill.Color != "#fff" || items[1].Fill.Gradient == nil || len(items[1].Fill.Gradient.Stops) != 2 || !items[2].Fill.IsZero() {
		t.Errorf("got %+v", items)
	}
}
