// Copied from github.com/d2lang/dagro, file version_test.go, at commit
// 30476a97eb11 (D39, D44, docs/UPSTREAM.md). Its MIT license is in
// internal/dagre/LICENSE, and its notice in internal/dagre/NOTICE.

package dagro

import "testing"

func TestVersionMatchesPortedDagreRelease(t *testing.T) {
	if Version != "3.1.1" {
		t.Fatalf("Version = %q, want %q", Version, "3.1.1")
	}
	if GraphlibVersion != "4.0.5" {
		t.Fatalf("GraphlibVersion = %q, want %q", GraphlibVersion, "4.0.5")
	}
}
