package content

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// PlainText renders Trilium HTML as a read-only plain-text view: one line
// per block, list markers, "a | b" table rows, [image: src] for images and
// "text [noteId]" for internal links. It is lossy by design and must never
// be written back.
func PlainText(src string) string {
	nodes, err := html.ParseFragment(strings.NewReader(src), &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"})
	if err != nil {
		return src
	}
	w := &textWriter{}
	for _, n := range nodes {
		w.node(n)
	}
	return w.result()
}

type listCtx struct {
	ordered bool
	n       int
}

type textWriter struct {
	b           strings.Builder
	lineHasText bool // something other than indentation/marker is on the line
	marker      bool // a list marker was written and no text followed yet
	atLineStart bool // nothing at all written on the current line yet
	lists       []listCtx
	pre         int
	cell        int   // depth inside td/th
	cellIndex   []int // per open tr: number of cells written
}

func (w *textWriter) indent() string {
	if len(w.lists) == 0 {
		return ""
	}
	return strings.Repeat("  ", len(w.lists))
}

func (w *textWriter) raw(s string) {
	if s == "" {
		return
	}
	w.b.WriteString(s)
	w.atLineStart = strings.HasSuffix(s, "\n")
}

// newline ends the current line if it holds any text. Inside table cells
// blocks are joined with a space instead, keeping one row per line.
func (w *textWriter) newline() {
	if w.cell > 0 {
		if w.lineHasText {
			w.text(" ")
		}
		return
	}
	if w.lineHasText {
		w.raw("\n")
		w.lineHasText = false
	}
}

// closeMarker ends a line that holds only a list marker.
func (w *textWriter) closeMarker() {
	if w.marker && !w.lineHasText {
		w.raw("\n")
	}
	w.marker = false
}

// blank ensures an empty line before what follows.
func (w *textWriter) blank() {
	if w.cell > 0 {
		w.newline()
		return
	}
	w.newline()
	if w.b.Len() > 0 && !strings.HasSuffix(w.b.String(), "\n\n") {
		w.raw("\n")
	}
}

func (w *textWriter) startLine() {
	if w.atLineStart || w.b.Len() == 0 {
		w.raw(w.indent())
		w.atLineStart = false
	}
}

var spaceRE = regexp.MustCompile(`[ \t\n\r\f]+`)

func (w *textWriter) text(s string) {
	if w.pre > 0 {
		w.preText(s)
		return
	}
	s = spaceRE.ReplaceAllString(s, " ")
	if !w.lineHasText {
		s = strings.TrimLeft(s, " ")
	} else if strings.HasPrefix(s, " ") && strings.HasSuffix(w.b.String(), " ") {
		s = s[1:]
	}
	if s == "" {
		return
	}
	w.startLine()
	w.raw(s)
	w.lineHasText = true
	w.marker = false
}

func (w *textWriter) preText(s string) {
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			w.raw("\n")
			w.lineHasText = false
		}
		if line != "" {
			w.startLine()
			w.raw(line)
			w.lineHasText = true
		}
	}
}

func (w *textWriter) children(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.node(c)
	}
}

func (w *textWriter) node(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.text(n.Data)
		return
	case html.ElementNode:
	default:
		w.children(n)
		return
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Head, atom.Title:
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		w.blank()
		w.children(n)
		w.blank()
	case atom.Br:
		if w.cell > 0 {
			w.text(" ")
			return
		}
		w.raw("\n")
		w.lineHasText = false
	case atom.Hr:
		w.newline()
		w.startLine()
		w.raw("---")
		w.lineHasText = true
		w.newline()
	case atom.Ul, atom.Ol:
		w.newline()
		w.lists = append(w.lists, listCtx{ordered: n.DataAtom == atom.Ol})
		w.children(n)
		w.lists = w.lists[:len(w.lists)-1]
		w.newline()
	case atom.Li:
		w.li(n)
	case atom.Input:
		// Checkboxes are rendered by li as "[ ]" / "[x]".
	case atom.Pre:
		w.newline()
		w.pre++
		w.children(n)
		w.pre--
		w.newline()
	case atom.Tr:
		w.newline()
		w.cellIndex = append(w.cellIndex, 0)
		w.children(n)
		w.cellIndex = w.cellIndex[:len(w.cellIndex)-1]
		w.newline()
	case atom.Td, atom.Th:
		if k := len(w.cellIndex); k > 0 {
			if w.cellIndex[k-1] > 0 {
				w.startLine()
				w.raw(" | ")
				w.lineHasText = true
			}
			w.cellIndex[k-1]++
		}
		w.cell++
		w.children(n)
		w.cell--
	case atom.Img:
		w.text("[image: " + attr(n, "src") + "]")
	case atom.A:
		w.children(n)
		if id, ok := internalLink(attr(n, "href")); ok {
			w.text(" [" + id + "]")
		}
	case atom.P, atom.Div, atom.Blockquote, atom.Figure, atom.Aside, atom.Section,
		atom.Article, atom.Header, atom.Footer, atom.Nav, atom.Main, atom.Table,
		atom.Thead, atom.Tbody, atom.Tfoot, atom.Figcaption, atom.Details, atom.Summary,
		atom.Dl, atom.Dt, atom.Dd, atom.Caption:
		w.newline()
		w.children(n)
		w.newline()
	default:
		w.children(n)
	}
}

func (w *textWriter) li(n *html.Node) {
	w.closeMarker()
	w.newline()
	depth := len(w.lists)
	marker := "- "
	if depth > 0 {
		l := &w.lists[depth-1]
		if l.ordered {
			l.n++
			marker = strconv.Itoa(l.n) + ". "
		}
	}
	if cb := findCheckbox(n); cb != nil {
		if hasAttr(cb, "checked") {
			marker += "[x] "
		} else {
			marker += "[ ] "
		}
	}
	// The marker sits one level left of the item's continuation lines.
	w.raw(strings.Repeat("  ", max(depth-1, 0)) + marker)
	w.atLineStart = false
	w.lineHasText = false
	w.marker = true
	w.children(n)
	w.closeMarker()
	w.newline()
}

// findCheckbox finds the item's own checkbox, not one of a nested list.
func findCheckbox(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if c.DataAtom == atom.Ul || c.DataAtom == atom.Ol {
			continue
		}
		if c.DataAtom == atom.Input && strings.EqualFold(attr(c, "type"), "checkbox") {
			return c
		}
		if cb := findCheckbox(c); cb != nil {
			return cb
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// internalLink extracts the target note ID from "#root/a/b/noteId".
func internalLink(href string) (string, bool) {
	rest, ok := strings.CutPrefix(href, "#root")
	if !ok {
		return "", false
	}
	rest = strings.Trim(rest, "/")
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" {
		return "root", true
	}
	return rest[strings.LastIndexByte(rest, '/')+1:], true
}

var blankLines = regexp.MustCompile(`\n{3,}`)

func (w *textWriter) result() string {
	lines := strings.Split(w.b.String(), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	s := strings.Join(lines, "\n")
	s = blankLines.ReplaceAllString(s, "\n\n")
	s = strings.Trim(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}
