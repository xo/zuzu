# D8. The scene graph is the boundary between the layout and every renderer

Status: Decided.

The runtime produces a scene graph, and every renderer reads it: the SVG
writer, the direct okage renderer and the browser. A scene graph is a tree of
groups and marks, where each mark holds items with their final positions,
sizes and colors. A renderer never reads a spec.

zuzu uses the shape of the scene graph of Vega, which vega.js writes as JSON
with `sceneToJSON`. So the marks and the data bound to each item can be
compared with vega.js (D17), while the positions, the guides and the look
belong to zuzu (D12). The scene graph has a JSON form, so a server can send
it to a browser (D19).

Gemini and Astra proposed fixing the scene graph first on 2026-11-10, and the
maintainers accepted it.
