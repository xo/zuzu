# D61. A tooltip shows its value as vega.js shows it

Status: Decided.

On 2026-11-28 the maintainers chose how zuzu writes the text of a tooltip
of Vega. vega.js shows a tooltip with its default handler, without a
plugin. zuzu writes the same text, which the SVG writer writes as a
`<title>` (D32 part 10). The look of zuzu stays its own (D12), so a field
that the config of vega.js sets shows the value of the theme of zuzu.

1. A tooltip whose value is the item itself, such as
   `"tooltip": {"signal": "item"}`, shows the fields of the item, as
   vega.js shows them after the run. zuzu writes the text once the item is
   whole, after its encodings and the transforms of its mark. It writes
   one line `key: value` for each field, in this order:
   1. `datum: {…}`, `exit: false`, `mark: {…}` and `bounds: {…}`. The
      bounds show as `{…}`, so zuzu shows them although an expression
      cannot read them (D59 part 1).
   2. For a group item, `items` with one `{…}` for each axis, legend,
      title and mark in the group, such as `items: [{…}, {…}]`.
   3. The defaults of the config, which the theme of zuzu and the config
      of the spec give, sorted by key. A property of the look whose name
      starts with `zuzu.` does not show.
   4. The properties that the encodings set, in the order of the spec,
      `enter` before `update`. A key keeps the place where it was first
      set. A value of undefined adds no key, because vega.js sets a
      property only when its value changes. The width or the x that x2,
      xc, y2 and yc give comes after the set that gives it, x before y.
   5. For a group item that holds at least one mark, `context: {…}`.
2. A tooltip whose value is another part of the scene, such as
   `item.mark`, or the item of a guide or of the root group, goes in the
   report. vega.js sets the fields of those parts in its own way.
3. The default fields of the config differ from those of vega.js, as the
   look does. No report line says so, because D12 covers the look. For
   example, the theme gives an arc `cornerRadius` and `padAngle`, which
   vega.js does not give it.
4. An object shows its keys in the order of the object, as `Object.keys`
   of JavaScript gives them. An index of an array, such as `2`, comes
   first, in the order of its number. zuzu knows the order of an object
   literal of an expression, in the whole expression or in a branch of a
   condition, and of an object of JSON in the spec. A map of Go keeps no
   order, so zuzu sorts the keys of any other object, such as a row of a
   dataset.
5. In an array, null and undefined show as nothing, as
   `Array.prototype.join` writes them, so `[1, null]` shows as `[1, ]`.
6. A date shows as `Date.prototype.toString()` of JavaScript in the time
   zone of the call (D27), such as
   `Wed Jan 01 2020 00:00:00 GMT-0500 (Eastern Standard Time)`. The
   function `toString` of an expression writes a date in the same way.
   The name of the time zone comes from `expr/zonenames.json`, which
   `test/oracle/zonenames.mjs` writes from Node.js for each zone of Go.
   JavaScript names a zone by its name of today. So a date at an offset
   that the zone does not have in 2024 takes the name of the standard or
   the daylight time of the zone. A zone that Node.js does not know, such
   as `time.Local` or a fixed zone, takes the name `GMT+hh:mm`.
