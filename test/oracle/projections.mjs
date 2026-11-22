// projections.mjs registers with vega.js the projections of
// vega-projection-extended that zuzu draws (D52), from d3-geo-projection,
// which vega.js already depends on. vega-projection-extended registers the
// same functions under the same names. Each oracle imports this module once,
// before it parses a spec.
import * as vega from "vega";
import * as d3gp from "d3-geo-projection";

// extended maps the name of a projection in a spec to its function in
// d3-geo-projection.
const extended = {
  airy: d3gp.geoAiry,
  armadillo: d3gp.geoArmadillo,
  baker: d3gp.geoBaker,
  berghaus: d3gp.geoBerghaus,
  bottomley: d3gp.geoBottomley,
  collignon: d3gp.geoCollignon,
  eckert1: d3gp.geoEckert1,
  guyou: d3gp.geoGuyou,
  hammer: d3gp.geoHammer,
  littrow: d3gp.geoLittrow,
  wagner6: d3gp.geoWagner6,
  wiechel: d3gp.geoWiechel,
  winkel3: d3gp.geoWinkel3,
  interruptedSinusoidal: d3gp.geoInterruptedSinusoidal,
  interruptedMollweide: d3gp.geoInterruptedMollweide,
  interruptedMollweideHemispheres: d3gp.geoInterruptedMollweideHemispheres,
  polyhedralButterfly: d3gp.geoPolyhedralButterfly,
  peirceQuincuncial: d3gp.geoPeirceQuincuncial,
};

for (const [name, fn] of Object.entries(extended)) {
  vega.projection(name, fn);
}
