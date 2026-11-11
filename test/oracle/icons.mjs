// icons.mjs draws each icon of each pack that zuzu bundles with
// @iconify/utils, the reference of the icons of D31, and writes a digest of
// the view box and the body of each pack to testdata/icons/oracle.json. The
// test test/icons reads the same packs with the package icon of zuzu and
// gives the same digests.
//
// Run it from the folder test: node oracle/icons.mjs. With --dump PACK, it
// prints the lines of one pack instead, to find the icon that differs.
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { getIconData, iconToSVG } from "@iconify/utils";

// The bundled packs, and a small pack of turns, flips and aliases that the
// bundled packs do not use.
const packs = {
  carbon: "../icons/carbon/carbon.json",
  fa: "../icons/fa/fa.json",
  k8s: "../icons/k8s/k8s.json",
  mdi: "../icons/mdi/mdi.json",
  transforms: "testdata/icons/transforms.json",
};

// lines returns, for each name of a pack in order, the name, the view box
// and the body, joined by tabs.
function lines(name) {
  const pack = JSON.parse(readFileSync(packs[name], "utf8"));
  const names = [...Object.keys(pack.icons), ...Object.keys(pack.aliases ?? {})].sort();
  return names.map((n) => {
    const data = getIconData(pack, n);
    const svg = iconToSVG(data);
    return `${n}\t${svg.attributes.viewBox}\t${svg.body}`;
  });
}

const i = process.argv.indexOf("--dump");
if (i >= 0) {
  process.stdout.write(lines(process.argv[i + 1]).join("\n") + "\n");
} else {
  const out = {};
  for (const p of Object.keys(packs)) {
    const l = lines(p);
    out[p] = { count: l.length, sha256: createHash("sha256").update(l.join("\n") + "\n").digest("hex") };
  }
  mkdirSync("testdata/icons", { recursive: true });
  writeFileSync("testdata/icons/oracle.json", JSON.stringify(out, null, 1) + "\n");
}
