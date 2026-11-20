package math_test

import (
	"strings"
	"testing"

	"github.com/xo/okage/texmath/stixtwomath"

	"github.com/xo/zuzu/text/math"
)

// TestTheFontIsUnderTheOpenFontLicense checks that the font names the SIL
// Open Font License in its table name, as zuzu checks each bundled font
// (D16, D36).
func TestTheFontIsUnderTheOpenFontLicense(t *testing.T) {
	t.Parallel()
	if !strings.Contains(string(stixtwomath.Data()), "S\x00I\x00L\x00 \x00O\x00p\x00e\x00n\x00 \x00F\x00o\x00n\x00t\x00 \x00L\x00i\x00c\x00e\x00n\x00s\x00e") &&
		!strings.Contains(string(stixtwomath.Data()), "SIL Open Font License") {
		t.Error("STIX Two Math does not name the SIL Open Font License")
	}
	if math.Font() == nil {
		t.Fatal("no font")
	}
	if _, err := math.Font().Layout("x", 16); err != nil {
		t.Errorf("the font does not lay out math: %v", err)
	}
}
