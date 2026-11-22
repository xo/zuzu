package main

import (
	"cmp"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// puppeteerConfig returns the file of a config of puppeteer that mmdc reads
// with -p: the file in PUPPETEER_CONFIG when it is set, or else a file in
// out that names the newest chrome-headless-shell in the cache of
// puppeteer. mmdc asks for one exact version of chrome-headless-shell,
// and an update of mermaid-cli can ask for a version that the cache does
// not hold, so that every original of Mermaid fails. A shell of another
// version draws the same SVG. It returns empty when it finds no shell, and
// mmdc then looks for its own.
func puppeteerConfig(out string) (string, error) {
	if pc := os.Getenv("PUPPETEER_CONFIG"); pc != "" {
		return pc, nil
	}
	shell := newestShell(puppeteerCache())
	if shell == "" {
		return "", nil
	}
	b, err := json.Marshal(map[string]string{"executablePath": shell})
	if err != nil {
		return "", err
	}
	file := filepath.Join(out, "puppeteer.json")
	if err := os.WriteFile(file, b, 0o644); err != nil {
		return "", err
	}
	return file, nil
}

// puppeteerCache returns the folder of the browsers that puppeteer
// installs: PUPPETEER_CACHE_DIR, or .cache/puppeteer in the home folder.
func puppeteerCache() string {
	if dir := os.Getenv("PUPPETEER_CACHE_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "puppeteer")
}

// newestShell returns the program of the newest chrome-headless-shell in
// the cache of puppeteer, such as
// chrome-headless-shell/linux-157.0.8103.78/chrome-headless-shell-linux64/chrome-headless-shell,
// or empty when the cache holds none.
func newestShell(cache string) string {
	if cache == "" {
		return ""
	}
	found, _ := filepath.Glob(filepath.Join(cache, "chrome-headless-shell", "*", "chrome-headless-shell-*", "chrome-headless-shell*"))
	var shells []string
	for _, f := range found {
		if st, err := os.Stat(f); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
			shells = append(shells, f)
		}
	}
	if len(shells) == 0 {
		return ""
	}
	// The folder of each version names it after the platform, such as
	// linux-157.0.8103.78.
	version := func(f string) []int {
		dir := filepath.Base(filepath.Dir(filepath.Dir(f)))
		_, v, _ := strings.Cut(dir, "-")
		var out []int
		for p := range strings.SplitSeq(v, ".") {
			n, _ := strconv.Atoi(p)
			out = append(out, n)
		}
		return out
	}
	return slices.MaxFunc(shells, func(a, b string) int {
		return cmp.Or(slices.Compare(version(a), version(b)), strings.Compare(a, b))
	})
}
