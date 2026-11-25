# Nunito

`Nunito.woff2` is Nunito, version 3.602, the rounded sans of zuzu (D35
part 11, D58 part 1). It is under the SIL Open Font License 1.1, which
`LICENSE.txt` holds. The license names no Reserved Font Name.

The file is one collection of six faces in WOFF2, in this order: Regular
(400), Medium (500), SemiBold (600), Italic (400), MediumItalic (500) and
SemiBoldItalic (600). The faces come from the two variable fonts of
<https://github.com/google/fonts>, in the folder `ofl/nunito` of the
commit `8b0a1d0f5983c89bc2b93f1b5fb55f9e252744b5`:

| File | SHA-256 |
| --- | --- |
| `Nunito[wght].ttf` | `bb55a5ca5c2042335b3991af27c4d0705d0ef41cac6164ac737fd8f2a1e85207` |
| `Nunito-Italic[wght].ttf` | `b520cc871868b0acfca1beda875df7f4a44ebce914f8a89f83977fc9c09529c8` |

`varLib.instancer` of fontTools 4.68.0 cuts each face from its variable
font at the weight of the face, with `updateFontNames`. A variable font
holds a range of weights in one file. A static face holds one, and okage
and go-text read the static faces. `TTCollection` of fontTools then joins
the six faces into one TrueType collection, and shares the tables that
are the same in each face. The collection holds 730,820 bytes, and its
SHA-256 is
`14f92563e9b662467f56dabc5a82e1d488d32b6d6770aa53eb06fbc875d5dc9b`.
`woff2_compress` of the Google WOFF2 tools then compresses it, as D34 did
for IBM Plex Sans. `Nunito.woff2` holds 175,980 bytes, and its SHA-256 is
`d1cc4dd3f5ee0c71347eb0f88cb0f01c2cd2d7582565f3045ae6b39808cb56ab`.

`LICENSE.txt` is the file `OFL.txt` of the same folder of google/fonts.
