#!/usr/bin/env bash
# diagrams.sh draws the references of the fixtures of DOT, and of the
# flowcharts, the state diagrams and the class diagrams of Mermaid (D31).
# It needs dot of Graphviz 16, and mmdc of mermaid-cli that bundles
# Mermaid 12.3.0. Run it from the folder test.
# If mmdc cannot find its Chrome, give it a config of puppeteer in the
# variable PUPPETEER_CONFIG, as {"executablePath": "..."}.
set -euo pipefail
dir=testdata/diagrams
for f in "$dir"/dot/*.dot; do
	dot -Tjson0 "$f" >"${f%.dot}.json"
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
args=(-q)
if [ -n "${PUPPETEER_CONFIG:-}" ]; then
	args+=(-p "$PUPPETEER_CONFIG")
fi
for f in "$dir"/mermaid/*.mmd; do
	base=$(basename "${f%.mmd}")
	mmdc "${args[@]}" -i "$f" -o "$tmp/$base.svg"
	go run ./cmd/mmdcref "$tmp/$base.svg" >"${f%.mmd}.json"
done
