package mermaid

import (
	"fmt"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/scene"
)

// click is a statement click, link or callback of Mermaid (D32): the ids of
// the nodes that it names, the URL of a link or the name of a callback, a
// tooltip and the target of the link.
type click struct {
	ids                             []string
	href, callback, tooltip, target string
	line                            int
}

// clickTokens splits the rest of a statement click into its words. A string
// in double quotes is one word, with its quotes. A word with a ( runs to its
// ), so that the arguments of a callback are one word.
func clickTokens(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		switch s[i] {
		case ' ', '\t':
			i++
		case '"':
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				return nil, fmt.Errorf("a string with no end quote: %w", ErrSyntax)
			}
			out = append(out, s[i:i+end+2])
			i += end + 2
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '\t' {
				if s[j] == '(' {
					end := strings.IndexByte(s[j:], ')')
					if end < 0 {
						return nil, fmt.Errorf("a callback with no closing parenthesis: %w", ErrSyntax)
					}
					j += end
				}
				j++
			}
			out = append(out, s[i:j])
			i = j
		}
	}
	return out, nil
}

// quoted reports whether a word is a string in double quotes.
func quoted(w string) bool {
	return len(w) >= 2 && w[0] == '"' && w[len(w)-1] == '"'
}

// parseClick reads a statement click, link or callback, after its first
// word:
//
//	click id "url" "tooltip" _blank
//	click id href "url" "tooltip" _blank
//	click id callback "tooltip"
//	click id call callback(args) "tooltip"
//	link id "url" "tooltip" _blank
//	callback id "callback" "tooltip"
//
// The id can name several nodes, as a,b. The tooltip and the target are
// optional.
func parseClick(word, rest string, line int) (click, error) {
	c := click{line: line}
	w, err := clickTokens(rest)
	if err != nil {
		return c, err
	}
	if len(w) < 2 || quoted(w[0]) {
		return c, fmt.Errorf("the %s %q: %w", word, rest, ErrSyntax)
	}
	for id := range strings.SplitSeq(w[0], ",") {
		if id = strings.TrimSpace(id); id != "" {
			c.ids = append(c.ids, id)
		}
	}
	w = w[1:]
	switch {
	case word == "link" && quoted(w[0]):
		c.href, w = unquote(w[0]), w[1:]
	case word == "callback" && quoted(w[0]):
		c.callback, w = unquote(w[0]), w[1:]
	case word != "click":
		return c, fmt.Errorf("the %s %q: %w", word, rest, ErrSyntax)
	case quoted(w[0]):
		c.href, w = unquote(w[0]), w[1:]
	case w[0] == "href" && len(w) > 1 && quoted(w[1]):
		c.href, w = unquote(w[1]), w[2:]
	case w[0] == "call" && len(w) > 1 && !quoted(w[1]):
		c.callback, w = w[1], w[2:]
	case w[0] != "href" && w[0] != "call":
		c.callback, w = w[0], w[1:]
	default:
		return c, fmt.Errorf("the %s %q: %w", word, rest, ErrSyntax)
	}
	if len(w) > 0 && quoted(w[0]) {
		c.tooltip, w = label(w[0]), w[1:]
	}
	if len(w) > 0 && !quoted(w[0]) && c.href != "" {
		c.target, w = w[0], w[1:]
	}
	if len(w) > 0 {
		return c, fmt.Errorf("the %s %q: %w", word, rest, ErrSyntax)
	}
	return c, nil
}

// applyClicks gives each node that a click names its link and its tooltip.
// node returns the node of an id, or nil. A callback, a target, a link with
// a scheme that zuzu does not write and an id that names no node are
// issues.
func applyClicks(is *graph.Issues, clicks []click, node func(id string) *graph.Node) error {
	for _, c := range clicks {
		path := fmt.Sprintf("line %d", c.line)
		if c.callback != "" {
			if err := is.Add(path, "the callback %s, since no script runs", c.callback); err != nil {
				return err
			}
		}
		if c.target != "" {
			if err := is.Add(path, "the target %s of a link", c.target); err != nil {
				return err
			}
		}
		href := c.href
		if href != "" && !scene.LinkAllowed(href) {
			if err := is.Add(path, "the link %s, whose scheme zuzu does not write", href); err != nil {
				return err
			}
			href = ""
		}
		for _, id := range c.ids {
			n := node(id)
			if n == nil {
				if err := is.Add(path, "a click on %s, which is not a node", id); err != nil {
					return err
				}
				continue
			}
			if href != "" {
				n.Href = href
			}
			if c.tooltip != "" {
				n.Tooltip = c.tooltip
			}
		}
	}
	return nil
}
