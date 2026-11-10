# D14. A theme is a mode and a color scheme, as one Vega config

Status: Decided.

The maintainers asked on 2026-11-10 for a simple way to apply styles, with a
light and a dark theme from the start, and for color schemes that swap.

1. A theme has two parts that a caller sets apart. The mode, light or dark,
   sets the neutral colors: the background, the text, the grid and the axes.
   The color scheme sets the colors of the data: ten categorical colors, a
   sequential ramp, a diverging ramp, and a color for a rise and for a fall.
2. Any scheme works in either mode. A scheme can give its own colors for the
   dark mode. Without them, zuzu lifts each light color for the dark mode, and
   builds the ramps from the background of the mode and the first color of
   the scheme, so that they read in both modes. A scheme from an upstream
   theme can also bring its own neutrals for each mode (D15).
3. Both parts become one Vega `config` object, because the config of Vega
   already holds every default that a theme sets: the background, the fonts,
   the axes, the legends, the title, the defaults of each kind of mark, and
   the color ranges. The order of precedence is the mode and the scheme, then
   the `config` of the spec, then the encoding of a mark. A theme of
   vega-themes is a JSON config, so zuzu reads it as it is.
4. The default look is the look of zuzu, not of Vega (D12). A rule of the
   look that Vega has no config for, such as the round corners of a bar, is a
   default of the zuzu renderer that a theme can turn off.

The theme types and the two modes are part of phase 1. The maintainers
accepted the four first schemes, `zuzu`, `pastel`, `harbor` and `ember`, on
2026-11-10, and asked for many more. D15 says where the others come from.
`docs/STYLE.md` holds the tokens of the modes and the schemes.

This decision merges three decisions of the same day: a light and a dark
theme from the start, a theme is a Vega config, which Gemini proposed, and a
color scheme swaps apart from the mode.
