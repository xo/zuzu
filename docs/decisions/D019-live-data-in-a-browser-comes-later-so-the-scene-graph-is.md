# D19. Live data in a browser comes later, so the scene graph is JSON

Status: Decided.

The maintainers asked on 2026-11-10 that zuzu later send live data to a web
browser, where an HTML and JavaScript component updates the chart. So a
canvas renderer must be possible.

The scene graph has a JSON form from phase 1 (D8), so that a server can send
it. The browser component is phase 8. Its JavaScript is an asset that the Go
module embeds and the browser runs, so it does not conflict with D5. Which
side runs the spec is open question 4 in `docs/PLAN.md`.
