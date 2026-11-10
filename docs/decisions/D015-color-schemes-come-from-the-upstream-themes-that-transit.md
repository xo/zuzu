# D15. Color schemes come from the upstream themes that transit draws from

Status: Decided.

The maintainers asked on 2026-11-10 whether zuzu can build color schemes
from the same sources as the package `styles` of transit, which colors source
code, and said that they want many more schemes that a caller selects with
ease. zuzu does not copy from that package. This decision says how zuzu
builds its schemes on its own, in the same way.

1. The sources are the styles of chroma, which come from Pygments, and the
   popular editor themes that chroma does not have, as transit D98 lists
   them. Each scheme takes its colors from the upstream palette of its
   theme, at a commit that its `source` field names.
2. A theme comes in only if its license lets zuzu copy its colors under the
   terms of MIT, such as MIT, BSD, ISC or Apache 2.0. zuzu holds the license
   file of each theme that it copies from.
3. A scheme from an editor theme brings its own neutrals for each mode: the
   background, the panel, the text and the grid of that theme. A theme with a
   light and a dark variant, such as Catppuccin Latte and Mocha, fills both
   modes of one scheme. A theme with one variant fills one mode, and the
   other mode uses the neutrals of zuzu.
4. The categorical colors are the accents of the theme, ordered so that
   neighbors differ in hue, with a color for a rise and a fall. A palette
   with fewer than ten accents gets the rest mixed toward the text color.
5. A scheme can name a font that suits it. The font is not bundled (D16).
6. A tool converts the upstream files into schemes. It reads them at pinned
   commits and writes one JSON file for each scheme, which the package
   `theme` embeds. A person reviews each scheme in the gallery of the
   targets before it ships.

The targets hold two prototypes on 2026-11-10: `catppuccin`, from
`github.com/catppuccin/palette` at `07d02aa`, and `solarized`, from
`github.com/altercation/solarized` at `62f656a`, both under MIT. Their license
files are in `docs/targets/licenses/`.

The maintainers accepted this decision on 2026-11-10.
