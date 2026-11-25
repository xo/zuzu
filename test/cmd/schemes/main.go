// Command schemes converts the styles of the package styles of transit into
// the color schemes of zuzu (D58).
//
// It reads the styles of the version of github.com/xo/transit/styles that
// the go.mod of the test module requires, and writes these files under the
// root module:
//
//	theme/schemes.go          the schemes as Go values
//	theme/licenses/           the license file of each source that a scheme uses
//	docs/SCHEMES.md           the table of the schemes
//	docs/targets/schemes.json the schemes that the targets draw
//
// "Schemes from upstream themes" in docs/STYLE.md holds the rules. The
// command prints the styles that it leaves out, and why.
//
// Run it from the test module:
//
//	go run ./cmd/schemes
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// modulePath is the module of the styles of transit.
const modulePath = "github.com/xo/transit/styles"

func main() {
	root := flag.String("root", "..", "the folder of the root module of zuzu")
	flag.Parse()
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	out, err := run(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemes:", err)
		os.Exit(1)
	}
	fmt.Printf("%d schemes, %d of them families\n", out.schemes, out.families)
	for _, l := range out.left {
		fmt.Printf("left out %s: %s\n", l.name, l.why)
	}
}

// run writes the files under root, and returns what it wrote.
func run(ctx context.Context, root string) (output, error) {
	dir, version, err := module(ctx)
	if err != nil {
		return output{}, err
	}
	out, err := generate(dir, version)
	if err != nil {
		return output{}, err
	}
	if err := os.RemoveAll(filepath.Join(root, "theme", "licenses")); err != nil {
		return output{}, err
	}
	for _, name := range slices.Sorted(maps.Keys(out.files)) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return output{}, err
		}
		if err := os.WriteFile(path, out.files[name], 0o644); err != nil {
			return output{}, err
		}
	}
	return out, nil
}

// module returns the folder and the version of the module of the styles,
// as the go command of the test module resolves it.
func module(ctx context.Context) (string, string, error) {
	b, err := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}\t{{.Version}}", modulePath).Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", "", fmt.Errorf("listing %s: %w: %s", modulePath, err, exit.Stderr)
		}
		return "", "", fmt.Errorf("listing %s: %w", modulePath, err)
	}
	dir, version, ok := strings.Cut(strings.TrimSpace(string(b)), "\t")
	if !ok || dir == "" {
		return "", "", fmt.Errorf("listing %s: the go command printed %q", modulePath, b)
	}
	return dir, version, nil
}

// licenseFiles returns the license files of the module of the styles, by
// their path under theme/licenses: each file in licenses/, and the license
// of transit itself as transit/LICENSE.
func licenseFiles(dir string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	fsys := os.DirFS(dir)
	err := fs.WalkDir(fsys, "licenses", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		rel, _ := strings.CutPrefix(name, "licenses/")
		out[rel] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	b, err := fs.ReadFile(fsys, "LICENSE")
	if err != nil {
		return nil, err
	}
	out[transitLicense] = b
	return out, nil
}
