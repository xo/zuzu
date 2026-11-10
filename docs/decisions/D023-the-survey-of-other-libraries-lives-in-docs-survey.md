# D23. The survey of other libraries lives in docs/survey

Status: Decided.

The maintainers asked on 2026-11-10 to keep the survey of chart libraries as
HTML in the documents, and to build a page that draws real data with the main
libraries side by side, for review, before anything is built.

`docs/survey/` holds both pages, the data, and the two scripts that generate
the data and the static images. `docs/survey/README.md` says how to open the
pages and how to generate the files again. The pages load each library from
a CDN at a pinned version, and zuzu does not use them.
