# IBM Plex Mono

`IBMPlexMono.woff2` is IBM Plex Mono, version 2.3, the monospace font of
zuzu (D35 part 11, D58 part 1). It is under the SIL Open Font License 1.1,
which `LICENSE.txt` holds. The license reserves the font name Plex.

The file is one collection of six faces in WOFF2, in this order: Regular
(400), Medium (500), SemiBold (600), Italic (400), MediumItalic (500) and
SemiBoldItalic (600). The six faces are the static TrueType files of
<https://github.com/google/fonts>, in the folder `ofl/ibmplexmono` of the
commit `431022a4eeb0e1009ba0126f41353d8ca2375a2a`, unchanged:

| File | SHA-256 |
| --- | --- |
| `IBMPlexMono-Regular.ttf` | `6a3412f058c7d8dfd9170c41e85ade48e5156ecb89356110ca57a0a27734af46` |
| `IBMPlexMono-Medium.ttf` | `a9b4c49bb299e05b5f6c481e7fb5e78943d2793249a0c8874ab574a2d1ea6755` |
| `IBMPlexMono-SemiBold.ttf` | `d3c38e55c78f5b0f28009fddba4834ec503278936a5986032424c9bd2d23aa46` |
| `IBMPlexMono-Italic.ttf` | `3362fc791b0652193328b862c1c5f23a789bc7288b1617fa63302f88689a2a34` |
| `IBMPlexMono-MediumItalic.ttf` | `de6e0f805a15a2a1538be5d1074741298a07ffcba6fa1f4cd93983ff73155a7d` |
| `IBMPlexMono-SemiBoldItalic.ttf` | `391ef0e273eb52571eccaab6a6dbddeec48e1d282cb7bf153043512b797ed6f0` |

`TTCollection` of fontTools 4.68.0 joins the six files into one TrueType
collection, and shares the tables that are the same in each face. The
collection holds 819,596 bytes, and its SHA-256 is
`af5b87a1ce5a3ed2ff0bc29729225c308dc2f33b2cb284959e48d60abaa7c3f1`.
`woff2_compress` of the Google WOFF2 tools then compresses it, as D34 did
for IBM Plex Sans. `IBMPlexMono.woff2` holds 156,640 bytes, and its
SHA-256 is
`821b0fa5b8f6d712dd18cf2c946feae0b90b34f513607b5cb38873fef0a55136`.

`LICENSE.txt` is the file `OFL.txt` of the same folder of google/fonts.
