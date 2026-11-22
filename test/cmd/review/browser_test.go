package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewestShell checks that the review tool takes the newest
// chrome-headless-shell of the cache of puppeteer, by its version and not
// by the order of its name, and finds none in an empty cache.
func TestNewestShell(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	if got := newestShell(cache); got != "" {
		t.Errorf("an empty cache gives %q, want none", got)
	}
	var want string
	for _, v := range []string{"linux-131.0.6778.204", "linux-157.0.8103.78", "linux-157.0.8103.9"} {
		dir := filepath.Join(cache, "chrome-headless-shell", v, "chrome-headless-shell-linux64")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(dir, "chrome-headless-shell")
		if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if v == "linux-157.0.8103.78" {
			want = f
		}
	}
	if got := newestShell(cache); got != want {
		t.Errorf("the newest shell is %q, want %q", got, want)
	}
}
