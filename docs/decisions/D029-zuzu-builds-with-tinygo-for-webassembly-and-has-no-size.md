# D29. zuzu builds with TinyGo for WebAssembly, and has no size target

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu targets TinyGo for a build
to WebAssembly where one is needed, as okage does (okage D25). zuzu then
keeps to what TinyGo supports:

1. No cgo, which D5 already holds.
2. `encoding/json/v2` and `encoding/json/jsontext` of the standard library,
   which TinyGo supports, and no decoder of its own. A field with more than
   one form implements `UnmarshalJSONFrom`, and no registry of types exists
   (D24).
3. No large package for one small use, such as `golang.org/x/text` for a
   locale (D27).

A build with the Go toolchain stays the main build. The maintainers also
decided that zuzu has no target for the size of a binary.
