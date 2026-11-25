# Inter

`Inter.woff2` is Inter, version 4.001, the humanist sans of zuzu (D35 part
11, D58 part 1). It is under the SIL Open Font License 1.1, which
`LICENSE.txt` holds. The license names no Reserved Font Name.

The file is one collection of six faces in WOFF2, in this order: Regular
(400), Medium (500), SemiBold (600), Italic (400), MediumItalic (500) and
SemiBoldItalic (600). The faces come from the two variable fonts of
<https://github.com/google/fonts>, in the folder `ofl/inter` of the commit
`0b58fb370093f9a9f4ff785d94405710b79de67c`:

| File | SHA-256 |
| --- | --- |
| `Inter[opsz,wght].ttf` | `29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031` |
| `Inter-Italic[opsz,wght].ttf` | `acd98e64795781b2058f07b18475e0ecee2a0fe2b42a49e2f9e37d0d6bf66ce6` |

`varLib.instancer` of fontTools 4.68.0 cuts each face from its variable
font at the optical size 14 and the weight of the face, with
`updateFontNames`. A variable font holds a range of weights in one file. A
static face holds one, and okage and go-text read the static faces.
`TTCollection` of fontTools then joins the six faces into one TrueType
collection, and shares the tables that are the same in each face. The
collection holds 1,853,600 bytes, and its SHA-256 is
`1d815fb10b00bcd2eafc1a69e2925f0c40ca66c97746382a3ba2c2a241ace55e`.
`woff2_compress` of the Google WOFF2 tools then compresses it, as D34 did
for IBM Plex Sans. `Inter.woff2` holds 432,696 bytes, and its SHA-256 is
`a2c7007404cc21e582b69c2313421b235522f60eeb6ec5ebfa66cb081516f74b`.

`LICENSE.txt` is the file `OFL.txt` of the same folder of google/fonts.
