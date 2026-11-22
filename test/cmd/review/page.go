package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pageData is what the template of the page reads.
type pageData struct {
	Generated string
	Fronts    []pageFront
}

// pageFront is the tab of a front end.
type pageFront struct {
	ID, Title, Original string

	Sources, ZuzuErrors, OriginalErrors, Reported int

	Rows []pageRow
}

// pageRow is the row of one source.
type pageRow struct {
	Name, Kind, Path, Source string
	Original, OriginalErr    string
	Light, Dark              string
	ZuzuErr, Report          []string
	// Flagged is true for a row with an error or a report line.
	Flagged bool

	// HasHandOriginal is true for a source whose original has a look drawn
	// by hand, with its SVG in HandOriginal or the reason in
	// HandOriginalErr.
	HasHandOriginal               bool
	HandOriginal, HandOriginalErr string
	// Hands are the drawings of zuzu in each look drawn by hand, and
	// HandCols the number of frames in the row drawn by hand.
	Hands    []pageHand
	HandCols int

	// Layouts are the drawings of zuzu in each layout. IsMermaid is true for
	// a diagram of Mermaid, whose original with layout: dagre is in
	// DagreOriginal or its reason in DagreOriginalErr.
	Layouts                         []pageLayout
	IsMermaid                       bool
	DagreOriginal, DagreOriginalErr string
}

// pageLayout is a drawing of zuzu in a layout.
type pageLayout struct {
	Name, Light, Dark, Position string
	Errs                        []string
}

// pageHand is a drawing of zuzu in a look drawn by hand.
type pageHand struct {
	Look, Light, Dark string
}

// writePage writes index.html.
func writePage(cfg config, srcs []*source) error {
	data := pageData{Generated: time.Now().UTC().Format("2006-01-02 15:04 UTC")}
	for _, f := range fronts {
		c := countFront(srcs, f.id)
		if c.sources == 0 {
			continue
		}
		pf := pageFront{
			ID: f.id, Title: f.title, Original: f.original,
			Sources: c.sources, ZuzuErrors: c.zuzuErrors, OriginalErrors: c.originalErrors, Reported: c.reported,
		}
		for _, s := range srcs {
			if s.front != f.id {
				continue
			}
			path, err := filepath.Rel(cfg.root, s.path)
			if err != nil || strings.HasPrefix(path, "..") {
				path = s.path
			}
			row := pageRow{
				Name: s.name, Kind: s.kind, Path: filepath.ToSlash(path), Source: sourceFile(s),
				Original: s.original, OriginalErr: s.originalErr,
				Light: s.light, Dark: s.dark,
				ZuzuErr: s.zuzuErr, Report: s.report,
				Flagged: s.originalErr != "" || len(s.zuzuErr) > 0 || len(s.report) > 0,
			}
			if len(s.hands) > 0 {
				row.HasHandOriginal = hasHandOriginal(s)
				row.HandOriginal, row.HandOriginalErr = s.handOriginal, s.handOriginalErr
				for _, h := range s.hands {
					row.Hands = append(row.Hands, pageHand{Look: string(h.look), Light: h.light, Dark: h.dark})
				}
				// The first frame holds the original drawn by hand, or says that
				// the original has none, so that each drawing of zuzu stays
				// under the column of zuzu.
				row.HandCols = len(row.Hands) + 1
			}
			for _, d := range s.layouts {
				row.Layouts = append(row.Layouts, pageLayout{Name: d.name, Light: d.light, Dark: d.dark, Position: d.position, Errs: d.errs})
			}
			row.IsMermaid = s.front == frontMermaid
			row.DagreOriginal, row.DagreOriginalErr = s.dagreOriginal, s.dagreOriginalErr
			pf.Rows = append(pf.Rows, row)
		}
		data.Fronts = append(data.Fronts, pf)
	}
	var b bytes.Buffer
	if err := page.Execute(&b, data); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.out, "index.html"), b.Bytes(), 0o644)
}

// page is the template of index.html. It needs no outside script, so that
// it opens from a file as well as from a server.
var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>zuzu review</title>
<style>
:root{--bg:#f4f5f7;--fg:#1b2230;--muted:#5f6b7d;--line:#d5d9e0;--card:#fff;--err:#b42318;--warn:#8a5a00;--accent:#2f6fde}
@media (prefers-color-scheme: dark){:root{--bg:#0b0e13;--fg:#e7ebf2;--muted:#9ba5b7;--line:#2a313d;--card:#151a22;--err:#ff8a80;--warn:#f0c060;--accent:#7aa7ff}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.4 system-ui,sans-serif}
header{position:sticky;top:0;z-index:2;background:var(--bg);border-bottom:1px solid var(--line);padding:10px 16px}
h1{font-size:16px;margin:0 8px 0 0;display:inline}
.meta{color:var(--muted);font-size:12px}
.tabs{display:flex;flex-wrap:wrap;gap:6px;margin:8px 0}
.tabs button{font:inherit;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px;padding:4px 10px;cursor:pointer}
.tabs button[aria-selected=true]{border-color:var(--accent);box-shadow:inset 0 -2px 0 var(--accent)}
.tabs .n{color:var(--muted);font-size:12px}
.tools{display:flex;flex-wrap:wrap;gap:12px;align-items:center}
.tools input[type=search]{font:inherit;padding:4px 8px;border:1px solid var(--line);border-radius:6px;background:var(--card);color:var(--fg);min-width:220px}
main{padding:8px 16px 40px}
.summary{color:var(--muted);margin:8px 0 12px}
.row{background:var(--card);border:1px solid var(--line);border-radius:8px;margin:0 0 14px;padding:10px 12px}
.row h2{font-size:14px;margin:0 0 8px;display:flex;flex-wrap:wrap;gap:8px;align-items:baseline}
.row h2 .kind{font-weight:normal;font-size:12px;border:1px solid var(--line);border-radius:10px;padding:0 8px;color:var(--muted)}
.row h2 a{font-weight:normal;font-size:12px;color:var(--accent)}
.pair{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.hand{display:grid;gap:12px;margin-top:12px;padding-top:10px;border-top:1px dashed var(--line)}
.hand.cols-1{grid-template-columns:1fr}
.hand.cols-2{grid-template-columns:1fr 1fr}
.hand.cols-3{grid-template-columns:1fr 1fr 1fr}
.hand.cols-4{grid-template-columns:1fr 1fr 1fr 1fr}
.layouts{display:grid;gap:12px;margin-top:12px;padding-top:10px;border-top:1px dashed var(--line);grid-template-columns:repeat(auto-fit,minmax(240px,1fr))}
.layouts .pos{font-size:12px;color:var(--muted);margin-top:4px}
.side{min-width:0}
.label{font-size:12px;color:var(--muted);margin-bottom:4px}
.frame{background:#fff;border:1px solid var(--line);border-radius:4px;padding:8px;min-height:60px;display:flex;align-items:center;justify-content:center;overflow:hidden}
.frame img{display:block;max-width:100%;max-height:520px;height:auto;width:auto}
.zuzu .frame.dark{background:#10141b}
.z-dark{display:none}
body.dark .z-light{display:none}
body.dark .z-dark{display:flex}
.frame .none{color:#888;font-size:12px}
pre.err{color:var(--err);white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px;margin:6px 0 0;max-height:200px;overflow:auto}
ul.report{color:var(--warn);font-size:12px;margin:6px 0 0;padding-left:18px;max-height:200px;overflow:auto;overflow-wrap:anywhere}
.hidden{display:none}
@media (max-width:760px){.pair,.hand[class]{grid-template-columns:1fr}}
</style>
</head>
<body>
<header>
<h1>zuzu review</h1><span class="meta">Each source drawn by its original renderer, on the left, and by zuzu, on the right. Below a dashed line, each source drawn by hand: by mmdc with look: handDrawn for Mermaid, and by zuzu in each look drawn by hand. Below another, each diagram in each layout: the original, and the original with layout: dagre for Mermaid, beside zuzu.layered, zuzu.dagre and zuzu.elk, each with its error of the places of the nodes against the original of its layout (0 is the same). Written {{.Generated}}.</span>
<nav class="tabs" role="tablist">
{{- range .Fronts}}
<button role="tab" data-tab="{{.ID}}" aria-selected="false">{{.Title}} <span class="n">{{.Sources}}</span></button>
{{- end}}
</nav>
<div class="tools">
<input type="search" id="q" placeholder="Filter by name" aria-label="Filter by name">
<label><input type="checkbox" id="flagged"> Only sources with errors or report lines</label>
<label><input type="checkbox" id="dark"> zuzu in the dark mode</label>
<span class="meta" id="shown"></span>
</div>
</header>
<main>
{{- range .Fronts}}
<section class="front hidden" id="front-{{.ID}}" data-front="{{.ID}}">
<p class="summary">{{.Sources}} sources. {{.ZuzuErrors}} with a zuzu error, {{.OriginalErrors}} with an error of the original, {{.Reported}} with report lines. The original is drawn by {{.Original}}.</p>
{{- range .Rows}}
<article class="row" data-name="{{.Name}} {{.Kind}}" data-flagged="{{if .Flagged}}1{{end}}">
<h2><span>{{.Name}}</span>{{if .Kind}}<span class="kind">{{.Kind}}</span>{{end}}<a href="{{.Source}}" title="{{.Path}}">source</a></h2>
<div class="pair">
<div class="side original">
<div class="label">original</div>
<div class="frame">{{if .Original}}<img src="{{.Original}}" loading="lazy" alt="{{.Name}}, original">{{else}}<span class="none">no drawing</span>{{end}}</div>
{{- if .OriginalErr}}<pre class="err">{{.OriginalErr}}</pre>{{end}}
</div>
<div class="side zuzu">
<div class="label">zuzu</div>
<div class="frame z-light">{{if .Light}}<img src="{{.Light}}" loading="lazy" alt="{{.Name}}, zuzu, light">{{else}}<span class="none">no drawing</span>{{end}}</div>
<div class="frame dark z-dark">{{if .Dark}}<img src="{{.Dark}}" loading="lazy" alt="{{.Name}}, zuzu, dark">{{else}}<span class="none">no drawing</span>{{end}}</div>
{{- range .ZuzuErr}}<pre class="err">{{.}}</pre>{{end}}
{{- if .Report}}<ul class="report">{{range .Report}}<li>{{.}}</li>{{end}}</ul>{{end}}
</div>
</div>
{{- if .Hands}}
<div class="hand cols-{{.HandCols}}">
<div class="side original">
{{- if .HasHandOriginal}}
<div class="label">original, look: handDrawn</div>
<div class="frame">{{if .HandOriginal}}<img src="{{.HandOriginal}}" loading="lazy" alt="{{.Name}}, original, drawn by hand">{{else}}<span class="none">no drawing</span>{{end}}</div>
{{- if .HandOriginalErr}}<pre class="err">{{.HandOriginalErr}}</pre>{{end}}
{{- else}}
<div class="label">original, drawn by hand</div>
<div class="frame"><span class="none">the original has no look drawn by hand</span></div>
{{- end}}
</div>
{{- $name := .Name}}
{{- range .Hands}}
<div class="side zuzu">
<div class="label">zuzu, look {{.Look}}</div>
<div class="frame z-light">{{if .Light}}<img src="{{.Light}}" loading="lazy" alt="{{$name}}, zuzu, {{.Look}}, light">{{else}}<span class="none">no drawing</span>{{end}}</div>
<div class="frame dark z-dark">{{if .Dark}}<img src="{{.Dark}}" loading="lazy" alt="{{$name}}, zuzu, {{.Look}}, dark">{{else}}<span class="none">no drawing</span>{{end}}</div>
</div>
{{- end}}
</div>
{{- end}}
{{- if .Layouts}}
<div class="layouts">
<div class="side original">
<div class="label">original{{if .IsMermaid}}, default layout: ELK{{end}}</div>
<div class="frame">{{if .Original}}<img src="{{.Original}}" loading="lazy" alt="{{.Name}}, original">{{else}}<span class="none">no drawing</span>{{end}}</div>
</div>
<div class="side original">
<div class="label">original, layout: dagre</div>
<div class="frame">{{if .DagreOriginal}}<img src="{{.DagreOriginal}}" loading="lazy" alt="{{.Name}}, original, dagre">{{else if .IsMermaid}}<span class="none">no drawing</span>{{else}}<span class="none">the original has one layout</span>{{end}}</div>
{{- if .DagreOriginalErr}}<pre class="err">{{.DagreOriginalErr}}</pre>{{end}}
</div>
{{- $name := .Name}}
{{- range .Layouts}}
<div class="side zuzu">
<div class="label">zuzu, layout {{.Name}}</div>
<div class="frame z-light">{{if .Light}}<img src="{{.Light}}" loading="lazy" alt="{{$name}}, zuzu, layout {{.Name}}, light">{{else}}<span class="none">no drawing</span>{{end}}</div>
<div class="frame dark z-dark">{{if .Dark}}<img src="{{.Dark}}" loading="lazy" alt="{{$name}}, zuzu, layout {{.Name}}, dark">{{else}}<span class="none">no drawing</span>{{end}}</div>
{{- if .Position}}<div class="pos">{{.Position}}</div>{{end}}
{{- range .Errs}}<pre class="err">{{.}}</pre>{{end}}
</div>
{{- end}}
</div>
{{- end}}
</article>
{{- end}}
</section>
{{- end}}
</main>
<script>
(function () {
  var tabs = Array.prototype.slice.call(document.querySelectorAll("[data-tab]"));
  var q = document.getElementById("q");
  var flagged = document.getElementById("flagged");
  var dark = document.getElementById("dark");
  var shown = document.getElementById("shown");
  var current = "";
  function show(id) {
    current = id;
    tabs.forEach(function (b) { b.setAttribute("aria-selected", b.dataset.tab === id ? "true" : "false"); });
    document.querySelectorAll("section.front").forEach(function (s) { s.classList.toggle("hidden", s.dataset.front !== id); });
    if (location.hash !== "#" + id) history.replaceState(null, "", "#" + id);
    filter();
  }
  function filter() {
    var text = q.value.trim().toLowerCase();
    var only = flagged.checked;
    var n = 0, all = 0;
    var section = document.getElementById("front-" + current);
    if (!section) return;
    section.querySelectorAll("article.row").forEach(function (r) {
      all++;
      var ok = (!text || r.dataset.name.toLowerCase().indexOf(text) >= 0) && (!only || r.dataset.flagged === "1");
      r.classList.toggle("hidden", !ok);
      if (ok) n++;
    });
    shown.textContent = n + " of " + all + " shown";
  }
  tabs.forEach(function (b) { b.addEventListener("click", function () { show(b.dataset.tab); }); });
  q.addEventListener("input", filter);
  flagged.addEventListener("change", filter);
  dark.addEventListener("change", function () { document.body.classList.toggle("dark", dark.checked); });
  var start = location.hash.slice(1);
  show(tabs.some(function (b) { return b.dataset.tab === start; }) ? start : (tabs[0] ? tabs[0].dataset.tab : ""));
})();
</script>
</body>
</html>
`))
