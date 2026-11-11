# D16. zuzu bundles IBM Plex Sans

Status: Decided. Amended by D32.

The maintainers decided on 2026-11-10, answering open question 2, that zuzu
bundles IBM Plex Sans. The targets already use it (D13).

IBM Plex Sans is under the SIL Open Font License 1.1, which lets zuzu embed
and redistribute the font files with their license. The module embeds the
weights that `docs/STYLE.md` uses, 400, 500 and 600, each upright and
italic, and the license file beside them. A caller can give other fonts,
and the bundled font is the default and the fallback.

The six files are the TTF files of the release `@ibm/plex-sans@1.1.0` of
`github.com/IBM/plex`, version 3.005 of the font, from the folder
`fonts/complete/ttf` of `ibm-plex-sans.zip`. Each file holds about 200 KB,
and the six hold 1.2 MB. The italic faces joined on 2026-11-11, so that the
direct renderer draws a text in the italic style with a true italic face.
They come from the same release as the upright faces, under the same
license.

A test that must give the same bytes on every machine uses the bundled font
and never a font of the system.

A color scheme can name a font that suits it (D15). zuzu bundles only IBM
Plex Sans, and a scheme that names another font falls back to it when the
caller has not given that font.
