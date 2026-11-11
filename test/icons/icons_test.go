// Package icons checks the packs of icons that zuzu bundles against
// @iconify/utils (D31): for each icon, the package icon gives the same view
// box and the same body. oracle/icons.mjs writes the digests that it
// reads.
package icons

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
)

func TestOracle(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../testdata/icons/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]struct {
		Count  int    `json:"count"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	tb, err := os.ReadFile("../testdata/icons/transforms.json")
	if err != nil {
		t.Fatal(err)
	}
	transforms, err := icon.Read(tb)
	if err != nil {
		t.Fatal(err)
	}
	packs := map[string]*icon.Pack{"carbon": carbon.Pack(), "fa": fa.Pack(), "k8s": k8s.Pack(), "mdi": mdi.Pack(), "transforms": transforms}
	if len(want) != len(packs) {
		t.Fatalf("expected %d packs, the oracle has %d", len(packs), len(want))
	}
	for name, p := range packs {
		var lines []string
		for _, n := range p.Names() {
			ic, ok := p.Icon(n)
			if !ok {
				t.Errorf("%s: %s does not resolve", name, n)
				continue
			}
			vb, body := ic.Render()
			box := make([]string, 4)
			for i, v := range vb {
				box[i] = strconv.FormatFloat(v, 'f', -1, 64)
			}
			lines = append(lines, n+"\t"+strings.Join(box, " ")+"\t"+body)
		}
		sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
		w := want[name]
		if len(lines) != w.Count || hex.EncodeToString(sum[:]) != w.SHA256 {
			t.Errorf("%s: %d icons, and the oracle has %d, or a body differs: compare node oracle/icons.mjs --dump %s", name, len(lines), w.Count, name)
		}
	}
}
