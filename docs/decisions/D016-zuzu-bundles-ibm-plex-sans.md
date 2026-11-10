# D16. zuzu bundles IBM Plex Sans

Status: Decided.

The maintainers decided on 2026-11-10, answering open question 2, that zuzu
bundles IBM Plex Sans. The targets already use it (D13).

IBM Plex Sans is under the SIL Open Font License 1.1, which lets zuzu embed
and redistribute the font files with their license. The module embeds the
weights that `docs/STYLE.md` uses, 400, 500 and 600, and the license file
beside them. A caller can give other fonts, and the bundled font is the
default and the fallback.

A test that must give the same bytes on every machine uses the bundled font
and never a font of the system.

A color scheme can name a font that suits it (D15). zuzu bundles only IBM
Plex Sans, and a scheme that names another font falls back to it when the
caller has not given that font.
