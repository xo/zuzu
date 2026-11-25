package geo

import (
	"fmt"
	"math"
	"testing"
)

// The tests in this file compare three details of the projections of D52
// with d3-geo-projection 4.0.0 and d3-geo 3.1.2, through geoPath: the
// meridian at ±180° of the interrupted and polyhedral projections, the
// latitudes of the lobes of an interrupted projection, and the outline of
// armadillo and berghaus with a clip angle. The numbers come from node
// with the packages of test/oracle, rounded to 15 digits.

// detail is the area and the bounds of a path in d3.
type detail struct {
	area   float64
	bounds [2][2]float64
}

// checkDetail compares the area and the bounds of the path of an object
// with those of d3, within tol pixels and within rel of the area.
func checkDetail(t *testing.T, name string, p *Projection, o any, want detail, tol, rel float64) {
	t.Helper()
	area, b := Area(p, o), Bounds(p, o)
	if math.Abs(area-want.area) > tol+rel*want.area {
		t.Errorf("%s: area %v, want %v", name, area, want.area)
	}
	for i := range 2 {
		for j := range 2 {
			if math.Abs(b[i][j]-want.bounds[i][j]) > tol {
				t.Errorf("%s: bounds %v, want %v", name, b, want.bounds)
				return
			}
		}
	}
}

// TestLobeMeridians draws a line and a polygon on the meridians at -180°
// and at 180°, with no rotation, and a line on the meridian that the
// default rotation turns to -180°. As in d3, a line on -180° draws along
// the west edge of the first lobe or face, and a line on 180° along the
// east edge of the last. zuzu moves a point on the meridian by 2e-6
// radians into its lobe. That moves it by less than 1e-3 pixels, and
// changes the area of a polygon by less than 2e-5 of it.
func TestLobeMeridians(t *testing.T) {
	t.Parallel()
	line := func(lon float64) any {
		return map[string]any{"type": "LineString", "coordinates": []any{
			[]any{lon, -60.0}, []any{lon, 0.0}, []any{lon, 60.0},
		}}
	}
	box := func(lon0, lon1 float64) any {
		return map[string]any{"type": "Polygon", "coordinates": []any{[]any{
			[]any{lon0, -30.0}, []any{lon0, 30.0}, []any{lon1, 30.0}, []any{lon1, -30.0}, []any{lon0, -30.0},
		}}}
	}
	for _, c := range []struct {
		typ string
		// rotate is the default rotation of the type, in degrees.
		rotate float64
		// want is for the line on -180°, the line on 180°, the polygon
		// on -180°, the polygon on 180° and the line that the default
		// rotation turns to -180°.
		want [5]detail
	}{
		{"interruptedSinusoidal", -20, [5]detail{
			{0, [2][2]float64{{0.498713282589904, 90.1662377608633}, {93.7350745887529, 409.833762239137}}},
			{0, [2][2]float64{{866.264925411247, 90.1662377608633}, {959.50128671741, 409.833762239137}}},
			{4060.0971818011, [2][2]float64{{0.498713282589904, 170.083118880432}, {48.5513374133816, 329.916881119568}}},
			{4060.09718180112, [2][2]float64{{911.448662586618, 170.083118880432}, {959.50128671741, 329.916881119568}}},
			{0, [2][2]float64{{0.498713282589904, 90.1662377608633}, {93.7350745887529, 409.833762239137}}},
		}},
		{"interruptedMollweide", 0, [5]detail{
			{0, [2][2]float64{{0.499577968903111, 67.2177745037401}, {75.7017730019268, 432.78222549626}}},
			{0, [2][2]float64{{818.496306344178, 67.2177745037401}, {959.500422031097, 432.78222549626}}},
			{5032.54440035276, [2][2]float64{{0.499577968903111, 153.147447151909}, {43.0313420429125, 346.852552848091}}},
			{5004.14867677169, [2][2]float64{{901.075806218139, 153.147447151909}, {959.500422031097, 346.852552848091}}},
			{0, [2][2]float64{{0.499577968903111, 67.2177745037401}, {75.7017730019268, 432.78222549626}}},
		}},
		{"interruptedMollweideHemispheres", 20, [5]detail{
			{0, [2][2]float64{{0.499577968903111, 67.2177745037401}, {85.1020473810547, 432.78222549626}}},
			{0, [2][2]float64{{874.897952618945, 67.2177745037401}, {959.500422031097, 432.78222549626}}},
			{5011.30732873722, [2][2]float64{{0.499577968903111, 153.147447151909}, {45.3017494341908, 346.852552848091}}},
			{5011.30732873717, [2][2]float64{{914.698250565809, 153.147447151909}, {959.500422031097, 346.852552848091}}},
			{0, [2][2]float64{{0.499577968903111, 67.2177745037401}, {85.1020473810547, 432.78222549626}}},
		}},
		{"polyhedralButterfly", 0, [5]detail{
			{0, [2][2]float64{{126.941475779562, 0.499873779591141}, {400.911629111842, 79.5882446677489}}},
			{0, [2][2]float64{{559.088370888158, 0.499873779591212}, {833.058524220438, 79.5882446677489}}},
			{4661.84904302257, [2][2]float64{{184.838181557544, 0.499873779591141}, {343.01492333386, 66.9529863290586}}},
			{4661.84904302258, [2][2]float64{{616.98507666614, 0.499873779591212}, {775.161818442456, 66.9529863290587}}},
			{0, [2][2]float64{{126.941475779562, 0.499873779591141}, {400.911629111842, 79.5882446677489}}},
		}},
	} {
		p := mustNew(t, c.typ)
		if err := p.Set("rotate", []any{0.0, 0.0}); err != nil {
			t.Fatal(err)
		}
		const tol, rel = 1e-3, 2e-5
		checkDetail(t, c.typ+" line -180", p, line(-180), c.want[0], tol, rel)
		checkDetail(t, c.typ+" line 180", p, line(180), c.want[1], tol, rel)
		checkDetail(t, c.typ+" polygon -180", p, box(-180, -170), c.want[2], tol, rel)
		checkDetail(t, c.typ+" polygon 180", p, box(170, 180), c.want[3], tol, rel)
		checkDetail(t, c.typ+" default rotation", mustNew(t, c.typ), line(-180-c.rotate), c.want[4], tol, rel)
	}
}

// TestLobeLatitudes sets lobes whose points have latitudes other than 0
// and ±90, and compares the points and the outline of the sphere with d3,
// with no rotation and with a rotation of [20, -30, 10]. A point nearer to
// the equator than the west point of its lobe moves by the x of the
// central meridian at its own latitude. The outline of each lobe runs from
// the latitude of its west point to that of its central point, and back
// to that of its east point. Where a lobe joins its neighbors below the
// latitude of its west point, the edge of the map is a curve, which d3
// resamples from the one edge of its outline there, and zuzu from a point
// every 2° along the edge of the clip. So the test draws the sphere with a
// precision of 0.01 pixels, where the two agree to 0.01 pixels.
func TestLobeLatitudes(t *testing.T) {
	t.Parallel()
	lobe := func(points ...float64) []any {
		out := make([]any, 0, 3)
		for i := 0; i < len(points); i += 2 {
			out = append(out, []any{points[i], points[i+1]})
		}
		return out
	}
	lobes := map[string]any{
		"interruptedSinusoidal": []any{
			[]any{lobe(-180, 15, -100, 80, -40, 10), lobe(-40, 10, 30, 90, 180, 15)},
			[]any{lobe(-180, -20, -150, -90, -90, -5), lobe(-90, -5, 0, -70, 60, -30), lobe(60, -30, 120, -90, 180, -20)},
		},
		"interruptedMollweide": []any{
			[]any{lobe(-180, 30, -90, 90, 0, 30), lobe(0, 30, 90, 90, 180, 30)},
			[]any{lobe(-180, 0, -90, -60, 0, 0), lobe(0, 0, 90, -90, 180, 0)},
		},
	}
	points := [][2]float64{
		{-170, 5}, {-170, 40}, {-60, 12}, {0, 8}, {0, 50}, {150, 14}, {-170, -10},
		{-170, -60}, {-50, -3}, {30, -50}, {100, -25}, {100, -70}, {0, 0},
	}
	for _, c := range []struct {
		typ    string
		rotate [3]float64
		// sphere is with a precision of 0.01.
		sphere detail
		xy     [][2]float64
	}{
		{"interruptedSinusoidal", [3]float64{0, 0, 0}, detail{288197.953505948, [2][2]float64{{0.500744612663993, 10.2493593051909}, {959.497272273988, 489.750640694809}}}, [][2]float64{
			{28.8609515303935, 236.680519813405}, {79.8410089500132, 143.444158507242}, {323.658988949531, 218.033247552173}, {480, 228.688831701448}, {507.333183134655, 116.805198134053}, {868.874796953602, 212.705455477535}, {34.0176699690668, 276.638960373189}, {77.8745223053951, 409.833762239137}, {346.987736980881, 257.991688111957}, {531.369580988451, 383.194801865947}, {721.430972247823, 316.597400932974}, {738.618074873298, 436.472722612326}, {480, 250},
		}},
		{"interruptedSinusoidal", [3]float64{20, -30, 10}, detail{288197.953505948, [2][2]float64{{0.500744612663993, 10.2493593051909}, {959.497272273988, 489.750640694809}}}, [][2]float64{
			{124.244637348148, 185.077048624911}, {198.430084827708, 106.317539968768}, {383.798736962457, 296.636277269278}, {541.869161805047, 293.42611593714}, {508.830972576109, 188.173358967269}, {834.536299999648, 130.340883502323}, {94.8595024138729, 221.389895464663}, {62.3602992466261, 342.348322303281}, {414.76728536182, 340.204138284122}, {709.820874602107, 384.443992878743}, {818.685188396234, 250.154024182103}, {808.347000262863, 361.199721059085}, {546.327419066815, 313.362663389374},
		}},
		{"interruptedMollweide", [3]float64{0, 0, 0}, detail{349069.245785619, [2][2]float64{{0.499580632794391, 10.2497889927375}, {959.500419367206, 489.750211007263}}}, [][2]float64{
			{28.2023828329379, 233.575772495732}, {80.0953008553871, 122.699183400593}, {322.33146871093, 210.673282306018}, {480, 223.741197602118}, {442.640106927169, 93.8848479948871}, {872.213672613935, 204.165512032007}, {29.1423385579259, 282.800454926193}, {102.340685336988, 432.78222549626}, {346.715332043591, 259.857606019339}, {598.44577673474, 406.115152005113}, {744.816492735532, 331.156434504526}, {733.25777143141, 456.64310881943}, {480, 250},
		}},
		{"interruptedMollweide", [3]float64{20, -30, 10}, detail{349069.245785619, [2][2]float64{{0.499580632794391, 10.2497889927375}, {959.500419367206, 489.750211007263}}}, [][2]float64{
			{126.54069201702, 170.835161908582}, {211.977445983678, 83.0978530594984}, {375.117423686111, 307.191129067205}, {548.846720436161, 303.297115325897}, {504.349430631543, 174.527766122381}, {884.599577825239, 108.291958999983}, {92.2456331572957, 214.783039517652}, {53.2593556775276, 361.235594230364}, {383.897608582349, 358.775119518545}, {724.309468173841, 407.418051537798}, {818.684719020345, 250.190019363175}, {843.903265528164, 382.454835022519}, {561.417914613668, 327.305635536462},
		}},
	} {
		p := mustNew(t, c.typ)
		if err := p.Set("lobes", lobes[c.typ]); err != nil {
			t.Fatal(err)
		}
		if err := p.Set("rotate", []any{c.rotate[0], c.rotate[1], c.rotate[2]}); err != nil {
			t.Fatal(err)
		}
		for i, q := range points {
			x, y, _ := p.Project(q[0], q[1])
			if want := c.xy[i]; math.Abs(x-want[0]) > 1e-6 || math.Abs(y-want[1]) > 1e-6 {
				t.Errorf("%s %v: %v = %v, %v, want %v, %v", c.typ, c.rotate, q, x, y, want[0], want[1])
			}
		}
		if err := p.Set("precision", 0.01); err != nil {
			t.Fatal(err)
		}
		checkDetail(t, c.typ+" sphere", p, map[string]any{"type": "Sphere"}, c.sphere, 0.01, 1e-5)
	}
}

// TestOutlineClipAngle sets a clip angle on armadillo and berghaus, then
// the parameter of the type, and compares the path of the sphere with d3.
// As in d3, the outline of the map stays the sphere, the torus or the
// star, and the clip cuts it. With no clip angle, the meridian at 180°
// runs to the south pole between two lobes of a star of 5 lobes, where d3
// and zuzu can pick different lobes by a rounding error, so the test
// takes 4 and 6 lobes there.
func TestOutlineClipAngle(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		typ string
		// param is the parallel of armadillo or the lobes of berghaus.
		param float64
		angle any
		want  detail
	}{
		{"armadillo", 20, 90.0, detail{232690.167357246, [2][2]float64{{170.732385964953, 53.4660455094285}, {789.267614035047, 499.499197212311}}}},
		{"armadillo", 20, 150.0, detail{337248.0422644, [2][2]float64{{57.5141145457141, 0.500396881047322}, {902.485851614487, 499.499197212311}}}},
		{"armadillo", 20, nil, detail{341772.935970072, [2][2]float64{{42.7880027600723, 0.500396881047322}, {917.035902211301, 499.499197212311}}}},
		{"armadillo", 35, 90.0, detail{216065.32147956, [2][2]float64{{170.730646756374, 7.95814041048163}, {789.269353243626, 442.411089163806}}}},
		{"berghaus", 5, 90.0, detail{59753.8154356416, [2][2]float64{{342.072144455324, 138.412533535037}, {617.927855544676, 414.268244624388}}}},
		{"berghaus", 5, 150.0, detail{106504.567628626, [2][2]float64{{255.304436860758, 46.4606304120649}, {704.695563139242, 475.03736854628}}}},
		{"berghaus", 4, 90.0, detail{59753.8154356416, [2][2]float64{{342.072144455324, 138.412533535037}, {617.927855544676, 414.268244624388}}}},
		{"berghaus", 4, nil, detail{107607.732373881, [2][2]float64{{204.159613749236, 0.500002829124981}, {755.840386250764, 552.1807753303}}}},
		{"berghaus", 6, nil, detail{114123.962842441, [2][2]float64{{241.115218987636, 0.500002829124981}, {718.884781012364, 552.1807753303}}}},
	} {
		p := mustNew(t, c.typ)
		if err := p.Set("clipAngle", c.angle); err != nil {
			t.Fatal(err)
		}
		param := "parallel"
		if c.typ == "berghaus" {
			param = "lobes"
		}
		if err := p.Set(param, c.param); err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("%s %s %v clipAngle %v", c.typ, param, c.param, c.angle)
		checkDetail(t, name, p, map[string]any{"type": "Sphere"}, c.want, 1e-5, 1e-8)
	}
}
