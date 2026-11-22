// Package dagro lays out a directed graph in ranks, as dagre 3.1.1 does. It
// is a copy of dagro, the Go implementation of dagre by the D2 team, with
// the changes that zuzu needs (D39, D44). The folder is internal/dagre and
// the package keeps the name dagro of upstream, so that each copied file
// keeps its package clause. The engine of zuzu.dagre in the package graph
// calls it. The README.md of this folder lists the files and the changes,
// and docs/UPSTREAM.md holds the rules for a copy.
//
// This file is a file of zuzu, not of dagro.
package dagro
