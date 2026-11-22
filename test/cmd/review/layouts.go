package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/elk"
	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/test/diagrams"
)

// measureLayouts gives each layout of a diagram of Mermaid its error of the
// places of the nodes against the original of its layout (D40 part 11,
// D44 part 4): zuzu.dagre against mmdc with layout: dagre, and zuzu.layered
// and zuzu.elk against the default original of mmdc, which is ELK. It reads
// the places of an original with cmd/mmdcref, which it builds once. A
// diagram of DOT has no reference of its places, so its layouts show no
// error.
func measureLayouts(ctx context.Context, cfg config, srcs []*source) {
	tool := filepath.Join(cfg.out, "mmdcref")
	//nolint:gosec // the command is fixed, and the output is a file in the output folder of the flag -out.
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", tool, "./cmd/mmdcref").CombinedOutput(); err != nil {
		msg := fmt.Sprintf("building cmd/mmdcref: %v: %s", err, strings.TrimSpace(string(out)))
		fmt.Fprintln(os.Stderr, "review:", msg)
		return
	}
	defer func() { _ = os.Remove(tool) }()
	for _, s := range srcs {
		if s.front != frontMermaid || len(s.layouts) == 0 {
			continue
		}
		src, err := os.ReadFile(s.path)
		if err != nil {
			continue
		}
		refs := map[string]diagrams.Places{}
		for name, svg := range map[string]string{graph.EngineELK: s.original, graph.EngineDagre: s.dagreOriginal} {
			if svg == "" {
				continue
			}
			if p, err := readOriginalPlaces(ctx, cfg, tool, svg); err == nil {
				refs[name] = p
			}
		}
		for i := range s.layouts {
			d := &s.layouts[i]
			against, what := graph.EngineELK, "the original"
			if d.name == graph.EngineDagre {
				against, what = graph.EngineDagre, "the original with layout: dagre"
			}
			want, ok := refs[against]
			if !ok {
				continue
			}
			got, _, err := diagrams.ZuzuPlaces(ctx, src, zuzu.WithLayoutEngine(elk.Layout()), zuzu.WithLayout(d.name))
			if err != nil {
				continue
			}
			if e, n, ok := diagrams.PositionError(got, want); ok {
				d.position = fmt.Sprintf("error %.3f over %d nodes, against %s", e, n, what)
			}
		}
	}
}

// readOriginalPlaces reads the places of the nodes of an original that mmdc
// drew, with the built cmd/mmdcref.
func readOriginalPlaces(ctx context.Context, cfg config, tool, svg string) (diagrams.Places, error) {
	out, err := exec.CommandContext(ctx, tool, filepath.Join(cfg.out, svg)).Output() //nolint:gosec // the tool is built from this repository, and the file is an original that this command drew.
	if err != nil {
		return nil, err
	}
	ref := filepath.Join(cfg.out, strings.TrimSuffix(svg, ".svg")+".ref.json")
	if err := os.WriteFile(ref, out, 0o644); err != nil {
		return nil, err
	}
	return diagrams.ReadPlaces(ref)
}
