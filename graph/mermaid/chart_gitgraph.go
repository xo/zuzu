package mermaid

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
)

// gitType is the type of a commit of a git graph.
type gitType int

// The types of a commit. A merge and a cherry-pick take their type from
// their statement. A merge can draw as another type.
const (
	gitNormal gitType = iota
	gitReverse
	gitHighlight
	gitMerge
	gitCherryPick
)

// gitTypes are the types that the source can give to a commit or a merge.
var gitTypes = map[string]gitType{"NORMAL": gitNormal, "REVERSE": gitReverse, "HIGHLIGHT": gitHighlight}

// gitCommit is a commit of a git graph.
type gitCommit struct {
	id     string
	seq    int
	typ    gitType
	tags   []string
	branch string
	// parents are the ids of the parents: none for the first commit of a
	// branch with no commit under it, one for a commit, and two for a merge
	// or a cherry-pick, the head of the branch first.
	parents []string
	// drawAs is the type that a merge draws as, or -1 for its own type.
	drawAs gitType
	// customID reports whether the source gives the id of a merge.
	customID bool
}

// symbol returns the type that the commit draws as.
func (c *gitCommit) symbol() gitType {
	if c.drawAs >= 0 {
		return c.drawAs
	}
	return c.typ
}

// gitBranch is a branch of a git graph, with its order.
type gitBranch struct {
	name  string
	order float64
}

// gitGraph is a git graph as the compiler reads it: the commits, the
// branches, and the state of the statements, as Mermaid keeps them.
type gitGraph struct {
	c   *chart
	cfg gitConfig
	dir string
	// commits holds each commit by its id, and ids the ids in the order of
	// the source. A commit with the id of an earlier commit replaces it, in
	// its place, as in Mermaid.
	commits map[string]*gitCommit
	ids     []string
	// branches are the branches in the order of the source, and heads the
	// id of the last commit of each branch, or empty.
	branches []gitBranch
	heads    map[string]string
	cur      string
	head     *gitCommit
	seq      int
	title    string
	hasTitle bool
}

// gitConfig holds the settings of a git graph.
type gitConfig struct {
	showBranches, showCommitLabel, rotate, parallel bool
	mainName                                        string
	mainOrder                                       float64
	padding                                         float64
}

// gitRead are the settings of the config that a git graph reads.
var gitRead = []string{
	"gitGraph.showBranches", "gitGraph.showCommitLabel", "gitGraph.mainBranchName",
	"gitGraph.mainBranchOrder", "gitGraph.rotateCommitLabel", "gitGraph.parallelCommits",
	"gitGraph.diagramPadding",
}

// gitGraphChart compiles a git graph: the commits of each branch on its
// lane, with the links from each commit to its parents, into a Vega spec
// (D31). The compiler lays out the graph in Go as Mermaid does, and the spec
// holds rows of rules, paths and text.
func gitGraphChart(c *chart) (map[string]any, bool, error) {
	g, err := readGitGraph(c)
	if err != nil {
		return nil, false, err
	}
	spec := newGitLayout(g).spec()
	if g.hasTitle {
		spec["title"] = map[string]any{"text": g.title, "anchor": "start"}
	}
	return spec, false, nil
}

// readGitGraph reads the settings and the statements of a git graph.
func readGitGraph(c *chart) (*gitGraph, error) {
	cfg, err := c.gitSettings()
	if err != nil {
		return nil, err
	}
	g := &gitGraph{
		c: c, cfg: cfg, dir: "LR", commits: map[string]*gitCommit{},
		branches: []gitBranch{{cfg.mainName, cfg.mainOrder}}, heads: map[string]string{cfg.mainName: ""},
		cur: cfg.mainName,
	}
	if err := g.parse(); err != nil {
		return nil, err
	}
	return g, nil
}

// gitSettings reads the config of a git graph. Each setting that it does
// not read is an issue.
func (c *chart) gitSettings() (gitConfig, error) {
	cfg := gitConfig{showBranches: true, showCommitLabel: true, rotate: true, mainName: "main", padding: 8}
	if err := c.unreadSettings(gitRead...); err != nil {
		return cfg, err
	}
	sec, _ := c.d.Config["gitGraph"].(map[string]any)
	for _, f := range []struct {
		key string
		to  *bool
	}{
		{"showBranches", &cfg.showBranches}, {"showCommitLabel", &cfg.showCommitLabel},
		{"rotateCommitLabel", &cfg.rotate}, {"parallelCommits", &cfg.parallel},
	} {
		v, ok := sec[f.key]
		if !ok {
			continue
		}
		b, ok := v.(bool)
		if !ok {
			if err := c.add("the setting gitGraph.%s %v, which is not true or false", f.key, v); err != nil {
				return cfg, err
			}
			continue
		}
		*f.to = b
	}
	if v, ok := sec["mainBranchName"]; ok {
		switch s := v.(type) {
		case string:
			cfg.mainName = s
		case float64:
			cfg.mainName = strconv.FormatFloat(s, 'f', -1, 64)
		default:
			if err := c.add("the setting gitGraph.mainBranchName %v", v); err != nil {
				return cfg, err
			}
		}
	}
	for _, f := range []struct {
		key string
		to  *float64
	}{{"mainBranchOrder", &cfg.mainOrder}, {"diagramPadding", &cfg.padding}} {
		v, ok := sec[f.key]
		if !ok {
			continue
		}
		n, ok := configNumber(c.d.Config, "gitGraph", f.key)
		if !ok {
			if err := c.add("the setting gitGraph.%s %v, which is not a number", f.key, v); err != nil {
				return cfg, err
			}
			continue
		}
		*f.to = n
	}
	return cfg, nil
}

// parse reads the header and the body of a git graph.
func (g *gitGraph) parse() error {
	g.c.line = g.c.d.N
	if err := g.header(g.c.d.Header); err != nil {
		return err
	}
	lines := g.c.d.Lines
	for i := 0; i < len(lines); i++ {
		g.c.line = lines[i].N
		n, ok, err := g.c.accessibility(lines, i)
		if err != nil {
			return err
		}
		if ok {
			i = n
			continue
		}
		t := lines[i].Text
		// A string in quotes can hold a new line.
		for openQuote(t) && i+1 < len(lines) {
			i++
			t += "\n" + lines[i].Text
		}
		if err := g.line(t); err != nil {
			return err
		}
	}
	return nil
}

// header reads the rest of the first line: a colon, a direction and a
// colon, or a statement, as in gitGraph LR: or gitGraph: commit.
func (g *gitGraph) header(h string) error {
	if rest, ok := strings.CutPrefix(h, ":"); ok {
		return g.line(rest)
	}
	if f := strings.Fields(h); len(f) > 0 {
		dir, _, _ := strings.Cut(f[0], ":")
		if dir == "LR" || dir == "TB" || dir == "BT" {
			after, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(h, dir)), ":")
			if !ok {
				return g.c.errorf("the direction %s with no colon after it. Write gitGraph %s:", dir, dir)
			}
			g.dir = dir
			return g.line(after)
		}
	}
	return g.line(h)
}

// openQuote reports whether a line ends inside a string in quotes.
func openQuote(s string) bool {
	var q byte
	for i := 0; i < len(s); i++ {
		switch {
		case q != 0 && s[i] == '\\':
			i++
		case q != 0 && s[i] == q:
			q = 0
		case q == 0 && (s[i] == '"' || s[i] == '\''):
			q = s[i]
		}
	}
	return q != 0
}

// gitTok is a token of a statement of a git graph.
type gitTok struct {
	kind gitTokKind
	text string
}

// gitTokKind is the kind of a token.
type gitTokKind int

// The kinds of a token: a keyword, a key such as id:, a string in quotes, an
// integer, a number with a fraction, and a name.
const (
	tokKeyword gitTokKind = iota
	tokKey
	tokString
	tokInt
	tokFloat
	tokName
)

// gitKeywords are the words that are keywords of a git graph, and so not a
// name of a branch.
var gitKeywords = []string{
	"gitGraph", "commit", "branch", "merge", "checkout", "switch", "cherry-pick",
	"NORMAL", "REVERSE", "HIGHLIGHT", "LR", "TB", "BT", "title", "accTitle", "accDescr",
}

// gitKeys are the keys of the options of a statement.
var gitKeys = []string{"id:", "msg:", "tag:", "type:", "order:", "parent:"}

// tokens splits a statement into its tokens, as the grammar of Mermaid
// reads them.
func (g *gitGraph) tokens(s string) ([]gitTok, error) {
	var out []gitTok
	for i := 0; i < len(s); {
		ch := s[i]
		switch ch {
		case ' ', '\t':
			i++
			continue
		case '"', '\'':
			text, n, ok := gitString(s[i:])
			if !ok {
				return nil, g.c.errorf("a string with no end quote")
			}
			out = append(out, gitTok{tokString, text})
			i += n
			continue
		}
		key := ""
		for _, k := range gitKeys {
			if strings.HasPrefix(s[i:], k) {
				key = k
				break
			}
		}
		if key != "" {
			out = append(out, gitTok{tokKey, key})
			i += len(key)
			continue
		}
		if ch >= '0' && ch <= '9' {
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			kind := tokInt
			if j+1 < len(s) && s[j] == '.' && s[j+1] >= '0' && s[j+1] <= '9' {
				kind = tokFloat
				j++
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			}
			out = append(out, gitTok{kind, s[i:j]})
			i = j
			continue
		}
		if !isWordByte(ch) {
			return nil, g.c.errorf("the character %q", ch)
		}
		// A name is a word character, then word characters, dashes, dots
		// and slashes, and it ends in a word character or a dash.
		j := i + 1
		for j < len(s) && (isWordByte(s[j]) || s[j] == '-' || s[j] == '.' || s[j] == '/') {
			j++
		}
		for j > i+1 && (s[j-1] == '.' || s[j-1] == '/') {
			j--
		}
		w := s[i:j]
		if slices.Contains(gitKeywords, w) {
			out = append(out, gitTok{tokKeyword, w})
		} else {
			out = append(out, gitTok{tokName, w})
		}
		i = j
	}
	return out, nil
}

// isWordByte reports whether a byte is a word character of a regular
// expression of JavaScript: a letter or a digit of ASCII, or an underscore.
func isWordByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// gitString reads a string in double or single quotes at the start of s.
// A backslash escapes the next character. It returns the text and the
// length of the string in s.
func gitString(s string) (string, int, bool) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case q:
			return b.String(), i + 1, true
		case '\\':
			i++
			if i >= len(s) {
				return "", 0, false
			}
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'v':
				b.WriteByte('\v')
			case '0':
				b.WriteByte(0)
			default:
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return "", 0, false
}

// line reads one statement.
func (g *gitGraph) line(t string) error {
	t = strings.TrimSpace(t)
	if t == "" {
		return nil
	}
	if rest, ok := keyword(t, "title"); ok {
		g.title, g.hasTitle = rest, true
		return nil
	}
	toks, err := g.tokens(t)
	if err != nil {
		return err
	}
	if toks[0].kind != tokKeyword {
		return g.c.errorf("the statement %q", t)
	}
	args := toks[1:]
	switch toks[0].text {
	case "commit":
		return g.commit(args)
	case "branch":
		return g.branch(args)
	case "merge":
		return g.merge(args)
	case "checkout", "switch":
		if len(args) != 1 || args[0].kind != tokName && args[0].kind != tokString {
			return g.c.errorf("%s with no name of a branch", toks[0].text)
		}
		return g.checkout(args[0].text)
	case "cherry-pick":
		return g.cherryPick(args)
	}
	return g.c.errorf("the statement %q", t)
}

// gitOptions are the options of a statement, such as id: "a".
type gitOptions struct {
	id, msg, parent  string
	hasID, hasParent bool
	tags             []string
	typ              gitType
	hasType          bool
	order            float64
}

// options reads the options of a statement. allowed names the keys that
// the statement takes. A commit takes a string with no key as its
// message.
func (g *gitGraph) options(stmt string, args []gitTok, allowed ...string) (gitOptions, error) {
	var o gitOptions
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a.kind == tokString && slices.Contains(allowed, "msg:") {
			o.msg = a.text
			continue
		}
		if a.kind != tokKey || !slices.Contains(allowed, a.text) {
			return o, g.c.errorf("the %s %q in a statement %s", gitTokName(a.kind), a.text, stmt)
		}
		if i+1 >= len(args) {
			return o, g.c.errorf("%s with no value in a statement %s", a.text, stmt)
		}
		i++
		v := args[i]
		switch a.text {
		case "type:":
			t, ok := gitTypes[v.text]
			if v.kind != tokKeyword || !ok {
				return o, g.c.errorf("the type %q, which is not NORMAL, REVERSE or HIGHLIGHT", v.text)
			}
			o.typ, o.hasType = t, true
			continue
		case "order:":
			if v.kind != tokInt {
				return o, g.c.errorf("the order %q, which is not a whole number", v.text)
			}
			n, _ := strconv.ParseFloat(v.text, 64)
			o.order = n
			continue
		}
		if v.kind != tokString {
			return o, g.c.errorf("the value %q of %s, which is not in quotes", v.text, a.text)
		}
		switch a.text {
		case "id:":
			o.id, o.hasID = v.text, true
		case "msg:":
			o.msg = v.text
		case "tag:":
			o.tags = append(o.tags, v.text)
		case "parent:":
			o.parent, o.hasParent = v.text, true
		}
	}
	return o, nil
}

// gitTokName names a kind of token in an error.
func gitTokName(k gitTokKind) string {
	switch k {
	case tokKeyword:
		return "keyword"
	case tokKey:
		return "key"
	case tokString:
		return "string"
	case tokInt, tokFloat:
		return "number"
	}
	return "word"
}

// randomID returns the id that Mermaid gives to a commit with no id: its
// number, a dash and seven hexadecimal digits. Mermaid picks the digits at
// random. zuzu takes them from a hash of the number, the branch and the
// message, so that the same source gives the same ids.
func (g *gitGraph) randomID(msg string) string {
	h := fnv.New32a()
	// A hash never fails to write.
	_, _ = h.Write([]byte(strconv.Itoa(g.seq) + "\x00" + g.cur + "\x00" + msg))
	return fmt.Sprintf("%d-%07x", g.seq, h.Sum32()&0xfffffff)
}

// put adds a commit and makes it the head of the current branch.
func (g *gitGraph) put(c *gitCommit) {
	if _, ok := g.commits[c.id]; !ok {
		g.ids = append(g.ids, c.id)
	}
	g.commits[c.id] = c
	g.heads[g.cur] = c.id
	g.head = c
	g.seq++
}

// parentsOfHead returns the parent of a new commit: the head, if there is
// one.
func (g *gitGraph) parentsOfHead() []string {
	if g.head == nil {
		return nil
	}
	return []string{g.head.id}
}

// commit reads commit, with an id, a message, tags and a type.
func (g *gitGraph) commit(args []gitTok) error {
	o, err := g.options("commit", args, "id:", "msg:", "tag:", "type:")
	if err != nil {
		return err
	}
	id := o.id
	if !o.hasID || id == "" {
		id = g.randomID(o.msg)
	}
	g.put(&gitCommit{id: id, seq: g.seq, typ: o.typ, tags: o.tags, branch: g.cur, parents: g.parentsOfHead(), drawAs: -1})
	return nil
}

// branch reads branch, with an order, and checks out the new branch.
func (g *gitGraph) branch(args []gitTok) error {
	if len(args) == 0 || args[0].kind != tokName && args[0].kind != tokString {
		return g.c.errorf("branch with no name")
	}
	name := args[0].text
	o, err := g.options("branch", args[1:], "order:")
	if err != nil {
		return err
	}
	if _, ok := g.heads[name]; ok {
		return g.c.errorf("the branch %s exists. Use a new name, or checkout %s", name, name)
	}
	head := ""
	if g.head != nil {
		head = g.head.id
	}
	g.heads[name] = head
	g.branches = append(g.branches, gitBranch{name, o.order})
	return g.checkout(name)
}

// checkout makes a branch the current branch.
func (g *gitGraph) checkout(name string) error {
	id, ok := g.heads[name]
	if !ok {
		return g.c.errorf("a checkout of the branch %s, which does not exist. Make it first with branch %s", name, name)
	}
	g.cur = name
	g.head = g.commits[id]
	return nil
}

// merge reads merge, with an id, tags and a type, and checks it as
// Mermaid does.
func (g *gitGraph) merge(args []gitTok) error {
	if len(args) == 0 || args[0].kind != tokName && args[0].kind != tokString {
		return g.c.errorf("merge with no name of a branch")
	}
	other := args[0].text
	o, err := g.options("merge", args[1:], "id:", "tag:", "type:")
	if err != nil {
		return err
	}
	cur := g.commits[g.heads[g.cur]]
	otherID, exists := g.heads[other]
	oc := g.commits[otherID]
	switch {
	case cur != nil && oc != nil && cur.branch == other:
		return g.c.errorf("a merge of the branch %s into itself", other)
	case g.cur == other:
		return g.c.errorf("a merge of the branch %s into itself", other)
	case cur == nil:
		return g.c.errorf("a merge into the branch %s, which has no commits", g.cur)
	case !exists:
		return g.c.errorf("a merge of the branch %s, which does not exist", other)
	case oc == nil:
		return g.c.errorf("a merge of the branch %s, which has no commits", other)
	case cur == oc:
		return g.c.errorf("a merge of the branch %s, which has the same head as %s", other, g.cur)
	}
	if o.hasID && o.id != "" {
		if _, ok := g.commits[o.id]; ok {
			return g.c.errorf("a merge with the id %s, which another commit has", o.id)
		}
	}
	id := o.id
	if id == "" {
		id = g.randomID("merged branch " + other + " into " + g.cur)
	}
	c := &gitCommit{
		id: id, seq: g.seq, typ: gitMerge, tags: o.tags, branch: g.cur, drawAs: -1, customID: o.id != "",
		parents: []string{g.head.id, otherID},
	}
	if o.hasType {
		c.drawAs = o.typ
	}
	g.put(c)
	return nil
}

// cherryPick reads cherry-pick, with the id of a commit, tags and a
// parent, and checks it as Mermaid does.
func (g *gitGraph) cherryPick(args []gitTok) error {
	o, err := g.options("cherry-pick", args, "id:", "tag:", "parent:")
	if err != nil {
		return err
	}
	src, ok := g.commits[o.id]
	switch {
	case !o.hasID || o.id == "" || !ok:
		return g.c.errorf("a cherry-pick of the commit %q, which does not exist", o.id)
	case o.parent != "" && !slices.Contains(src.parents, o.parent):
		return g.c.errorf("a cherry-pick with the parent %s, which is not a parent of the commit %s", o.parent, o.id)
	case src.typ == gitMerge && o.parent == "":
		return g.c.errorf("a cherry-pick of the merge %s with no parent. Give the parent with parent:", o.id)
	case src.branch == g.cur:
		return g.c.errorf("a cherry-pick of the commit %s, which is on the branch %s already", o.id, g.cur)
	case g.commits[g.heads[g.cur]] == nil:
		return g.c.errorf("a cherry-pick into the branch %s, which has no commits", g.cur)
	}
	// A cherry-pick with no tag takes the tag cherry-pick:id. An empty tag
	// draws no tag.
	var tags []string
	for _, t := range o.tags {
		if t != "" {
			tags = append(tags, t)
		}
	}
	if len(o.tags) == 0 {
		tag := "cherry-pick:" + src.id
		if src.typ == gitMerge {
			tag += "|parent:" + o.parent
		}
		tags = []string{tag}
	}
	g.put(&gitCommit{
		id: g.randomID("cherry-picked " + src.id + " into " + g.cur), seq: g.seq, typ: gitCherryPick,
		tags: tags, branch: g.cur, drawAs: -1, parents: []string{g.head.id, src.id},
	})
	return nil
}

// sortedBranches returns the branches by their order. Branches of the
// same order keep the order of the source.
func (g *gitGraph) sortedBranches() []gitBranch {
	out := slices.Clone(g.branches)
	slices.SortStableFunc(out, func(a, b gitBranch) int { return cmp.Compare(a.order, b.order) })
	return out
}
