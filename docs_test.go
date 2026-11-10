package zuzu_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// These tests read the repository rather than the package. The documents
// point at each other and at the decisions by number, and these tests keep
// those references true as the documents move and the decisions change. They
// come from dbmeta, which wrote them first, and they check the layout that D2
// and D4 set.

// markdownLink matches a relative link to a document, and leaves an absolute
// one alone.
var markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:]+\.(?:md|html|txt|json|mjs|py))(#[^)]*)?\)`)

// decisionRef matches a bare reference to a decision, such as D14.
var decisionRef = regexp.MustCompile(`\bD([1-9][0-9]*)\b`)

// foreignRef matches a decision of another project, such as dbmeta D110 or
// okage D36. Those projects number their own decisions, so a number after
// their name says nothing about docs/decisions here.
var foreignRef = regexp.MustCompile(`\b(?:dbmeta|okage|transit|usql)(?:'s)?\s+D[1-9][0-9]*` +
	`(?:(?:,\s*|\s+and\s+|\s+to\s+|\s+or\s+)D[1-9][0-9]*)*`)

// bareMention matches a document named in running text, which is how a Go
// comment points at one, as in: See docs/PLAN.md for the plan.
var bareMention = regexp.MustCompile(`(?:^|[\s` + "`" + `(])((?:docs/)?[A-Z][A-Z_]*\.md)`)

// TestEveryMarkdownLinkResolves walks every document and every Go comment and
// checks that each file it names exists.
func TestEveryMarkdownLinkResolves(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md", ".go", ".yml") {
		body := read(t, path)
		dir := filepath.Dir(path)
		for _, m := range markdownLink.FindAllStringSubmatch(body, -1) {
			target := filepath.Join(dir, m[1])
			if generated[filepath.ToSlash(target)] {
				continue
			}
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: link to %s does not resolve to %s", path, m[1], target)
			}
		}
		if strings.HasSuffix(path, ".md") {
			continue
		}
		for _, m := range bareMention.FindAllStringSubmatch(body, -1) {
			if _, err := os.Stat(filepath.Join(".", m[1])); err != nil {
				t.Errorf("%s: names %s, which is not a file. Moving a document "+
					"means fixing the comments that point at it.", path, m[1])
			}
		}
	}
}

// generated holds the files that a script writes and git ignores, which a
// document can link to although a fresh checkout does not hold them. See
// docs/targets/README.md.
var generated = map[string]bool{
	"docs/targets/index.html": true,
}

// TestEveryDecisionReferenceExists checks that a decision named by number is a
// decision that was written. A reference to a number that nobody wrote is how
// a reader ends up trusting a rule that does not exist.
func TestEveryDecisionReferenceExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
	}
	for _, path := range repoFiles(t, ".md", ".go") {
		body := foreignRef.ReplaceAllString(read(t, path), "")
		for _, m := range decisionRef.FindAllStringSubmatch(body, -1) {
			if !written[m[1]] {
				t.Errorf("%s: refers to D%s, which is not in docs/decisions", path, m[1])
			}
		}
	}
}

// TestEveryTestNameInTheDocsExists checks that a test the documentation names
// is a test that is written. A rule is often paired with the test that holds
// it, and a renamed test breaks that pair without a word.
func TestEveryTestNameInTheDocsExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, path := range repoFiles(t, ".go") {
		for _, m := range regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9]+)\(`).
			FindAllStringSubmatch(read(t, path), -1) {
			written[m[1]] = true
		}
	}
	for _, path := range repoFiles(t, ".md") {
		for _, m := range regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9]*`).
			FindAllString(read(t, path), -1) {
			if !written[m] {
				t.Errorf("%s: names %s, which no test defines. Renaming a test "+
					"means fixing the documents that tell a reader to trust it.",
					path, m)
			}
		}
	}
}

// decision is one file in docs/decisions.
type decision struct {
	num    string
	title  string
	status string
	file   string
}

// decisionFile names a decision file: D, the number in three digits, and the
// title in lower case words joined by hyphens.
var decisionFile = regexp.MustCompile(`^D(\d{3})-[a-z0-9-]+\.md$`)

// decisions reads every decision in docs/decisions, in order. Each opens with
// its number, its title and its status (D4):
//
//	# D14. A theme is a mode and a color scheme, as one Vega config
//
//	Status: Decided.
func decisions(t *testing.T) []decision {
	t.Helper()
	dir := filepath.Join("docs", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := regexp.MustCompile(`\A# D(\d+)\. (.+)\n\nStatus: (.+)\.\n`)
	var out []decision
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		name := decisionFile.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named D, three digits, a hyphen and the"+
				" title in lower case words, such as D014-a-theme.md", e.Name())
			continue
		}
		m := head.FindStringSubmatch(read(t, filepath.Join(dir, e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with \"# D<n>. <title>\", a blank line and"+
				" \"Status: <status>.\"", e.Name())
			continue
		}
		if n, _ := strconv.Atoi(name[1]); strconv.Itoa(n) != m[1] {
			t.Errorf("%s holds D%s. The file name and the heading name one decision.", e.Name(), m[1])
		}
		out = append(out, decision{num: m[1], title: m[2], status: m[3], file: e.Name()})
	}
	if len(out) == 0 {
		t.Fatalf("found no decision in %s", dir)
	}
	return out
}

// TestTheDecisionIndexIsComplete checks the table in docs/decisions/README.md
// against the decision files. A reader finds a decision by its number in that
// table, so a missing row or a stale status there hides it.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	index := read(t, filepath.Join("docs", "decisions", "README.md"))
	rows := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(.*$`).FindAllStringSubmatch(index, -1) {
		rows[m[1]] = m[0]
	}
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
		want := fmt.Sprintf("| [D%s](%s) | %s | %s |", d.num, d.file, d.title, d.status)
		switch got, ok := rows[d.num]; {
		case !ok:
			t.Errorf("D%s has no row in docs/decisions/README.md. Add:\n%s", d.num, want)
		case got != want:
			t.Errorf("D%s: the row in docs/decisions/README.md is\n%s\nand the file says\n%s", d.num, got, want)
		}
	}
	for num := range rows {
		if !written[num] {
			t.Errorf("docs/decisions/README.md has a row for D%s, and no file holds it", num)
		}
	}
}

// TestTheCountOfDecisionsIsRight checks the number of decisions that AGENTS.md
// quotes. A number written in prose goes stale the next time somebody adds a
// decision.
func TestTheCountOfDecisionsIsRight(t *testing.T) {
	t.Parallel()
	want := len(decisions(t))
	m := regexp.MustCompile(`from D1 to D(\d+)`).FindStringSubmatch(read(t, "AGENTS.md"))
	if m == nil {
		t.Fatal("AGENTS.md no longer says \"from D1 to D<n>\". Fix the pattern or drop the test, " +
			"because a guard that matches nothing guards nothing.")
	}
	if m[1] != strconv.Itoa(want) {
		t.Errorf("AGENTS.md says the decisions run from D1 to D%s, and there are %d", m[1], want)
	}
}

// TestAnAmendmentPointsBothWays checks that a decision that amends or replaces
// another says so in its status, and that the other one says it back. A
// reader who finds the older decision first then learns that it changed.
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	status := make(map[string]string)
	for _, d := range decisions(t) {
		status[d.num] = d.title + ". " + d.status
	}
	naming := regexp.MustCompile(`(?i)\b(?:amends|supersedes|superseded by|replaced by|amended by) D(\d+)`)
	for num, head := range status {
		for _, m := range naming.FindAllStringSubmatch(head, -1) {
			other := m[1]
			if _, ok := status[other]; !ok {
				t.Errorf("D%s names D%s, which is not a decision", num, other)
				continue
			}
			if !strings.Contains(status[other], "D"+num) {
				t.Errorf("D%s says %q, and D%s does not mention D%s. An amendment is "+
					"visible from both sides, or a reader of the older decision gets a "+
					"rule that no longer holds.", num, head, other, num)
			}
		}
	}
}

// TestTheRootHoldsFourDocuments holds the layout of D2. A document that
// appears in the root is one that nobody filed.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !allowed[e.Name()] {
			t.Errorf("%s is in the repository root. Only README, AGENTS, CLAUDE and CONTRIBUTING belong there, "+
				"and everything else goes in docs/. See D2.", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the repository root", name)
		}
	}
}

// TestClaudeImportsAgents holds D2. AGENTS.md holds the rules, because Codex
// and the other agents read it, and CLAUDE.md imports it, so that Claude Code
// reads the same rules. A symbolic link does not work, because a Windows
// checkout writes a link as a small text file.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	info, err := os.Lstat("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("CLAUDE.md is a symbolic link. Make it a file that holds @AGENTS.md. See D2")
	}
	if got := strings.TrimSpace(read(t, "CLAUDE.md")); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q. It holds only @AGENTS.md, and the rules go in"+
			" AGENTS.md. See D2", got)
	}
}

// TestNoSectionHeadingIsRepeated checks that a level two heading appears once
// in the document that holds it. Two sections with one name means that a
// section landed in the wrong place.
func TestNoSectionHeadingIsRepeated(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		seen := make(map[string]bool)
		for _, m := range regexp.MustCompile(`(?m)^## (.+)$`).
			FindAllStringSubmatch(read(t, path), -1) {
			if seen[m[1]] {
				t.Errorf("%s has two sections called %q", path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// TestEveryDocumentIsInBothTables checks that a document in docs/ is named in
// the table at the top of AGENTS.md and in the one in README.md. AGENTS.md
// says that a document that is not in its table does not exist, which is only
// true if something holds it.
func TestEveryDocumentIsInBothTables(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]string{
		"AGENTS.md": read(t, "AGENTS.md"),
		"README.md": read(t, "README.md"),
	}
	var found int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		found++
		for name, body := range tables {
			if !strings.Contains(body, "docs/"+e.Name()) {
				t.Errorf("%s does not name docs/%s. A document that nobody can find "+
					"is a document that nobody reads.", name, e.Name())
			}
		}
	}
	if found == 0 {
		t.Error("docs/ holds no document, so this guards nothing")
	}
}

// personName matches the names of the maintainers, which a document never
// writes (D22). The copyright line of LICENSE is not a document, so it is
// not read.
var personName = regexp.MustCompile(`\b(?:Ken|Kenneth|Shaw|kenshaw)\b`)

// TestTheDocumentsNameNoPerson holds D22. A document writes "the
// maintainers" for a choice or a request.
func TestTheDocumentsNameNoPerson(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		for i, line := range strings.Split(read(t, path), "\n") {
			if m := personName.FindString(line); m != "" {
				t.Errorf("%s:%d: names %q. Write \"the maintainers\". See D22", path, i+1, m)
			}
		}
	}
}

// repoFiles returns every file with one of the given extensions. It skips the
// folders whose name starts with a dot, other than .github, the packages that
// npm installs, and the test data, which holds the files of other projects.
func repoFiles(t *testing.T, exts ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != "." && name != ".github") ||
				name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				out = append(out, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// read returns the content of a file of the repository.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
