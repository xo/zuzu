# D17. Conformance with vega.js measures meaning, not look

Status: Decided.

The corpus of `vegagoja` holds 579 Vega-Lite specs, the 578 Vega specs that
vega-lite.js compiled from them, and 87 Vega specs, with the SVGs that vega.js
drew. The test module copies it. An oracle runs vega.js and vega-lite.js in
Node.js, with the current releases, and writes references for each spec.

zuzu does not reproduce the look of Vega (D12), so a comparison of SVG or of
pixels with vega.js tells nothing. The comparison measures the meaning of a
chart instead. These must match exactly, within a tolerance for floats:

1. The Vega spec that `vegalite` compiles, against the one that vega-lite.js
   compiles.
2. The rows of each dataset after the transforms: the count, the fields and
   the values. The order counts only where the spec sets it.
3. The type and the domain of each scale.
4. The number of items of each mark, and the datum bound to each item.
5. A label whose format the spec gives, such as `,.2f` or `%Y`.

These match with a tolerance, or as a relation:

1. The position of each item, as a fraction of the domain of its scale, and
   not in pixels, because the size of the plot area differs.
2. The colors of a category, as a relation: two items of the same category
   have the same color and two of different categories do not.
3. The tick values and the default formats, which are choices of design
   where the spec does not set them.

A test never compares the range of a scale, a band width or a plot area with
vega.js, because they depend on the layout. Each report gives the share of
passes in the set of specs that zuzu claims to support and the share in the
whole corpus. okage is tested apart from zuzu, so that a defect in one does
not hide a defect in the other.

The look has its own tests. The output of zuzu has golden files, and a person
reviews a change to them in a gallery, as the targets (D13) are reviewed.

Gemini, Astra and DeepSeek proposed a structural comparison before pixels on
2026-11-10, and Astra proposed the two shares. After D12, Gemini and DeepSeek
proposed the comparison of meaning, and Gemini warned about the ranges and
the bands. The maintainers accepted both. This decision merges the two.
