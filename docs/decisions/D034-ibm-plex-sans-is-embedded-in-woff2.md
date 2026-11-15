# D34. IBM Plex Sans is embedded in WOFF2

Status: Decided. Amends D16.

On 2026-11-15 the maintainers chose to embed IBM Plex Sans in WOFF2,
because okage now decodes WOFF2 (okage D45). WOFF2 is a font format for the
web. It holds the same tables as a TrueType file, compressed with brotli.

## The decision

1. The package `text` embeds the six faces of IBM Plex Sans as one
   collection in WOFF2, `fonts/IBMPlexSans.woff2`, not as six `.ttf`
   files. A collection is one font file that holds several faces. This
   amends D16.
2. `text.Fonts` gives the collection as a TrueType collection,
   `fonts/IBMPlexSans.ttc`, beside `LICENSE.txt`. The package decodes it
   with `svg.ConvertFontData` of okage on the first call, and keeps the
   result. A renderer of okage loads the six faces from it, and go-text
   parses it with `ParseTTC`. The package reads the weight and the style of
   each face from the face.
3. The package `text/emoji` keeps Noto Color Emoji as `NotoColorEmoji.ttf`
   (D32).
4. `text.NewFallback` accepts a font in TrueType, OpenType, WOFF, WOFF2 or
   EOT. It decodes a font in a web format on its first use, and
   `Fallback.Data` gives the TrueType or OpenType bytes. It uses TrueType
   and OpenType bytes as they are, with no copy.
5. The collection comes from the six `.ttf` files of D16, in this order:
   Regular, Medium, SemiBold, Italic, MediumItalic and SemiBoldItalic.
   `TTCollection` of fontTools joins them, and shares the tables that are
   the same in each face. `woff2_compress` of the Google WOFF2 tools then
   compresses the collection. The fonts, their version and their license
   do not change.

## The sizes

| Font | TrueType | WOFF2 |
| --- | --- | --- |
| IBM Plex Sans, six faces, as six files | 1,235,804 bytes | 405,684 bytes |
| IBM Plex Sans, six faces, as one collection | 1,202,116 bytes | 277,124 bytes |
| Noto Color Emoji | 10,730,124 bytes | 9,559,412 bytes |

Brotli compresses the six faces of the collection as one stream, so the
parts that the faces share compress once. On an AMD Ryzen 9 9950X, the
decode of the collection takes 8 ms and allocates 13 MB. The decoded
collection, 1.2 MB, stays in memory after the first use. The decode of
Noto Color Emoji takes 81 ms and allocates 40 MB.

## Why

A program that imports zuzu holds IBM Plex Sans. One collection in WOFF2
makes that part of the program 958 KB smaller, and the decode on the first
use is short. The raster tests give the same pixels with the decoded
collection as with the six TrueType files, and the widths of
`text.Estimate` do not change.

Noto Color Emoji stays in TrueType. Its emoji are PNG images, which brotli
does not compress much, so WOFF2 saves only 11 percent of the file. The
decode costs 81 ms, and the decoded 10.7 MB stays on the heap, where the
TrueType file stays in the program and costs no heap.
