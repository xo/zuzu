package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/transit/styles"
)

// family is a scheme with the name of a family of styles, which takes the
// light mode from one style and the dark mode from another.
type family struct {
	name, light, dark string
}

// families names the light and the dark style of each family. A family
// whose name is the name of one of its styles replaces the scheme of that
// style, as "Schemes from upstream themes" in docs/STYLE.md says.
var families = []family{
	{"alabaster", "alabaster-light", "alabaster-dark"},
	{"ayu", "ayu-light", "ayu-dark"},
	{"catppuccin", "catppuccin-latte", "catppuccin-mocha"},
	{"edge", "edge-light", "edge-dark"},
	{"everforest", "everforest-light", "everforest-dark"},
	{"github", "github", "github-dark"},
	{"gruvbox", "gruvbox-light", "gruvbox"},
	{"horizon", "horizon-bright", "horizon"},
	{"iceberg", "iceberg-light", "iceberg-dark"},
	{"kanagawa", "kanagawa-lotus", "kanagawa-wave"},
	{"material", "material-lighter", "material"},
	{"melange", "melange-light", "melange-dark"},
	{"modus", "modus-operandi", "modus-vivendi"},
	{"monokai", "monokailight", "monokai"},
	{"night-owl", "light-owl", "night-owl"},
	{"nightfox", "nightfox-dayfox", "nightfox-nightfox"},
	{"noctis", "noctis-lux", "noctis"},
	{"oceanic-next", "oceanic-next-light", "oceanic-next"},
	{"papercolor", "papercolor-light", "papercolor-dark"},
	{"paraiso", "paraiso-light", "paraiso-dark"},
	{"poimandres", "poimandres-white", "poimandres"},
	{"rose-pine", "rose-pine-dawn", "rose-pine"},
	{"selenized", "selenized-light", "selenized-dark"},
	{"solarized", "solarized-light", "solarized-dark"},
	{"spacegray", "spacegray-light", "spacegray"},
	{"tokyonight", "tokyonight-day", "tokyonight-night"},
	{"tomorrow", "tomorrow", "tomorrow-night"},
	{"vscode", "vscode-light-plus", "vscode-dark-plus"},
	{"winter-is-coming", "winter-is-coming-light", "winter-is-coming-dark-blue"},
	{"xcode", "xcode", "xcode-dark"},
	{"zenbones", "zenbones-light", "zenbones-dark"},
}

// targetSchemes are the schemes that docs/targets/gen-targets.mjs draws,
// which the command writes to docs/targets/schemes.json.
var targetSchemes = []string{"catppuccin", "solarized"}

// The paths of three license files under theme/licenses: the license of
// chroma, of Pygments and of transit itself.
const (
	chromaLicense   = "chroma/COPYING"
	pygmentsLicense = "pygments/LICENSE"
	transitLicense  = "transit/LICENSE"
)

// scheme is a scheme that the command writes: a style, or a family with a
// light and a dark style. One of its modes is nil for a style.
type scheme struct {
	name     string
	family   bool
	light    *mode
	dark     *mode
	licenses []string
}

// modes returns the modes that the scheme has, light first.
func (s scheme) modes() []*mode {
	var out []*mode
	for _, m := range []*mode{s.light, s.dark} {
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

// leftOut is a style that is not a scheme, and why.
type leftOut struct {
	name, why string
}

// output is what generate makes: the files by their path under the root
// module, the counts of the schemes and the families, and the styles left
// out.
type output struct {
	files    map[string][]byte
	schemes  int
	families int
	left     []leftOut
}

// generate reads the styles of the module in dir and returns the files
// that the command writes.
func generate(dir, version string) (output, error) {
	licenses, err := licenseFiles(dir)
	if err != nil {
		return output{}, err
	}
	out := output{files: make(map[string][]byte)}
	modes := make(map[string]mode)
	var all []scheme
	for _, name := range styles.Names() {
		s, _ := styles.Get(name)
		m, ok := modeOf(s)
		if !ok {
			out.left = append(out.left, leftOut{name, fmt.Sprintf("it keeps %d colors, fewer than %d", len(m.colors), minColors)})
			continue
		}
		modes[name] = m
		sc := scheme{name: name}
		if m.dark {
			sc.dark = &m
		} else {
			sc.light = &m
		}
		all = append(all, sc)
	}
	for _, f := range families {
		light, okLight := modes[f.light]
		dark, okDark := modes[f.dark]
		switch {
		case !okLight || !okDark:
			out.left = append(out.left, leftOut{f.name, "the family needs both `" + f.light + "` and `" + f.dark + "`"})
			continue
		case light.dark || !dark.dark:
			return output{}, fmt.Errorf("the family %s: %s is not light, or %s is not dark", f.name, f.light, f.dark)
		}
		all = slices.DeleteFunc(all, func(s scheme) bool { return s.name == f.name })
		all = append(all, scheme{name: f.name, family: true, light: &light, dark: &dark})
		out.families++
	}
	slices.SortFunc(all, func(a, b scheme) int { return strings.Compare(a.name, b.name) })
	used := make(map[string]bool)
	for i := range all {
		for _, m := range all[i].modes() {
			files, err := licensesOf(*m, licenses)
			if err != nil {
				return output{}, err
			}
			for _, f := range files {
				if !slices.Contains(all[i].licenses, f) {
					all[i].licenses = append(all[i].licenses, f)
				}
				used[f] = true
			}
		}
	}
	for name := range used {
		out.files["theme/licenses/"+name] = licenses[name]
	}
	out.schemes = len(all)
	if out.files["theme/schemes.go"], err = goFile(all, version); err != nil {
		return output{}, err
	}
	out.files["docs/SCHEMES.md"] = markdown(all, out.left, version)
	if out.files["docs/targets/schemes.json"], err = targetsJSON(all, version); err != nil {
		return output{}, err
	}
	return out, nil
}

// licensesOf returns the license files of the source of a style, by their
// path under theme/licenses. A style of chroma takes the license of
// chroma, and of Pygments when chroma took it from Pygments. A style of
// transit takes the license of transit. A style from a repository takes
// the file that names the repository, or else the file whose name is the
// longest part of the name of the style before a dash.
func licensesOf(m mode, files map[string][]byte) ([]string, error) {
	switch {
	case strings.HasPrefix(m.source, "chroma "):
		out := []string{chromaLicense}
		if strings.Contains(m.source, "Pygments") {
			out = append(out, pygmentsLicense)
		}
		return out, nil
	case strings.HasPrefix(m.source, "transit,"):
		return []string{transitLicense}, nil
	case strings.HasPrefix(m.source, "github.com/"):
		repo, _, _ := strings.Cut(m.source, " ")
		for name, b := range files {
			if !strings.Contains(name, "/") && bytes.Contains(b, []byte("The license of "+repo+":")) {
				return []string{name}, nil
			}
		}
		for name := m.style; ; {
			if _, ok := files[name+".txt"]; ok {
				return []string{name + ".txt"}, nil
			}
			i := strings.LastIndexByte(name, '-')
			if i < 0 {
				break
			}
			name = name[:i]
		}
	}
	return nil, fmt.Errorf("the style %s: no license file for the source %q", m.style, m.source)
}

// goFile writes theme/schemes.go.
func goFile(all []scheme, version string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by test/cmd/schemes. DO NOT EDIT.\n\npackage theme\n\n")
	b.WriteString("// upstream holds the schemes from the styles of transit, sorted by their\n")
	fmt.Fprintf(&b, "// names (D58). They come from %s %s.\n", modulePath, version)
	b.WriteString("// docs/SCHEMES.md lists them.\n")
	b.WriteString("var upstream = [...]Scheme{\n")
	quote := func(list []string) string {
		q := make([]string, len(list))
		for i, s := range list {
			q[i] = strconv.Quote(s)
		}
		return "[]string{" + strings.Join(q, ", ") + "}"
	}
	for _, s := range all {
		b.WriteString("{\n")
		for _, m := range s.modes() {
			fmt.Fprintf(&b, "// %s: `%s`\n", m.style, m.source)
		}
		fmt.Fprintf(&b, "name: %q,\n", s.name)
		var own [2]string
		if m := s.light; m != nil {
			fmt.Fprintf(&b, "light: %s,\n", quote(m.colors))
			own[0] = neutrals(m)
		}
		if m := s.dark; m != nil {
			fmt.Fprintf(&b, "dark: %s,\n", quote(m.colors))
			own[1] = neutrals(m)
		}
		first := s.modes()[0]
		fmt.Fprintf(&b, "up: %d,\ndown: %d,\n", first.up, first.down)
		if s.family && (s.dark.up != first.up || s.dark.down != first.down) {
			fmt.Fprintf(&b, "darkUp: %d,\ndarkDown: %d,\n", s.dark.up, s.dark.down)
		}
		switch {
		case own[0] != "" && own[1] != "":
			fmt.Fprintf(&b, "own: [2]Neutrals{%s, %s},\n", own[0], own[1])
		case own[0] != "":
			fmt.Fprintf(&b, "own: [2]Neutrals{%s},\n", own[0])
		default:
			fmt.Fprintf(&b, "own: [2]Neutrals{1: %s},\n", own[1])
		}
		fmt.Fprintf(&b, "licenses: %s,\n", quote(s.licenses))
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("formatting theme/schemes.go: %w", err)
	}
	return src, nil
}

// neutrals returns the Go value of the neutrals of a mode: the background,
// and the text when the style gives it.
func neutrals(m *mode) string {
	if !m.ownText {
		return fmt.Sprintf("{Background: %q}", m.bg)
	}
	return fmt.Sprintf("{Background: %q, Text: %q}", m.bg, m.text)
}

// markdown writes docs/SCHEMES.md.
func markdown(all []scheme, left []leftOut, version string) []byte {
	var b strings.Builder
	b.WriteString("# Schemes\n\n")
	fmt.Fprintf(&b, "This document lists the color schemes that come from the styles of\n`%s` %s (D58). The command `test/cmd/schemes` of the test\n", modulePath, version)
	b.WriteString("module writes it, with `theme/schemes.go`. Do not edit it. Run the command\n")
	// The link is relative to docs/, where the document lives.
	fmt.Fprintf(&b, "again. \"Schemes from upstream themes\" in [%[1]s](%[1]s) holds the\nrules.\n\n", "STYLE.md")
	fmt.Fprintf(&b, "There are %d schemes. %d of them are families, which take the light\n", len(all), countFamilies(all))
	b.WriteString("mode from one style and the dark mode from another. A scheme of one style\n")
	b.WriteString("has one mode. In the other mode, it takes the neutrals of zuzu, and its\n")
	b.WriteString("colors lift for the dark mode or darken for the light mode. Each neutral\n")
	b.WriteString("other than `bg` and `text` mixes `text` into `bg`. `theme/licenses/` holds\n")
	b.WriteString("the license of each source.\n\n")
	b.WriteString("| Scheme | Style | Mode | `bg` | `text` | Colors, from 1 | Rise, fall | Source |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, s := range all {
		for _, m := range s.modes() {
			name := "light"
			if m.dark {
				name = "dark"
			}
			colors := make([]string, len(m.colors))
			for i, c := range m.colors {
				colors[i] = "`" + c + "`"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | `%s` | `%s` | %s | %d, %d | `%s` |\n",
				s.name, m.style, name, m.bg, m.text, strings.Join(colors, " "), m.up+1, m.down+1, m.source)
		}
	}
	b.WriteString("\n## Styles left out\n\n")
	fmt.Fprintf(&b, "A style with fewer than %d colors after the rules of `docs/STYLE.md` is\nnot a scheme, and neither is a family with such a style:\n\n", minColors)
	b.WriteString("| Style | Why |\n| --- | --- |\n")
	for _, l := range left {
		fmt.Fprintf(&b, "| `%s` | %s |\n", l.name, l.why)
	}
	return []byte(b.String())
}

// countFamilies returns the count of the families among the schemes.
func countFamilies(all []scheme) int {
	var n int
	for _, s := range all {
		if s.family {
			n++
		}
	}
	return n
}

// targetMode is a mode of a scheme in docs/targets/schemes.json.
type targetMode struct {
	Style  string   `json:"style"`
	Source string   `json:"source"`
	Bg     string   `json:"bg"`
	Text   string   `json:"text"`
	Cat    []string `json:"cat"`
	Up     int      `json:"up"`
	Down   int      `json:"down"`
}

// targetsJSON writes docs/targets/schemes.json: the schemes that the
// targets draw, each with its light and its dark mode.
func targetsJSON(all []scheme, version string) ([]byte, error) {
	out := map[string]any{"version": modulePath + " " + version}
	for _, name := range targetSchemes {
		i := slices.IndexFunc(all, func(s scheme) bool { return s.name == name })
		if i < 0 || !all[i].family {
			return nil, fmt.Errorf("the targets draw %s, which is not a family", name)
		}
		modes := make(map[string]targetMode)
		for key, m := range map[string]*mode{"light": all[i].light, "dark": all[i].dark} {
			modes[key] = targetMode{m.style, m.source, m.bg, m.text, m.colors, m.up, m.down}
		}
		out[name] = modes
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
