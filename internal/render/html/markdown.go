package html

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
)

// markdown writes the Markdown of a text or callout widget as HTML. The tree
// comes from deliverable.ParseMarkdown, the strict reader validation used. The
// walker writes only the elements a deliverable allows, from constant strings;
// every piece of text from the source passes through template.HTMLEscapeString
// on its way out (esc below), and a link's address is written only when it is
// an http, https or mailto address. Raw HTML and images, which validation
// refuses, are dropped here too, so a spec that skipped validation still
// renders inert.
func markdown(src string) string {
	w := walker{src: []byte(src)}
	w.children(deliverable.ParseMarkdown(w.src))
	return w.b.String()
}

type walker struct {
	src []byte
	b   strings.Builder
}

// esc is the one way source text reaches the output.
func (w *walker) esc(s []byte) {
	w.b.WriteString(template.HTMLEscapeString(string(s)))
}

func (w *walker) children(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		w.node(c)
	}
}

// wrap writes children between an opening and a closing tag, both constants.
func (w *walker) wrap(open string, n ast.Node, closing string) {
	w.b.WriteString(open)
	w.children(n)
	w.b.WriteString(closing)
}

func (w *walker) node(n ast.Node) {
	switch n := n.(type) {
	case *ast.Heading:
		h := strconv.Itoa(min(deliverable.DemoteHeading(n.Level), 6))
		w.wrap("<h"+h+">", n, "</h"+h+">\n")
	case *ast.Paragraph:
		w.wrap("<p>", n, "</p>\n")
	case *ast.TextBlock:
		w.children(n)
		if n.NextSibling() != nil {
			w.b.WriteString("\n")
		}
	case *ast.Text:
		if n.IsRaw() {
			w.esc(n.Segment.Value(w.src))
		} else {
			w.esc(decode(n.Segment.Value(w.src)))
		}
		switch {
		case n.HardLineBreak():
			w.b.WriteString("<br>\n")
		case n.SoftLineBreak():
			w.b.WriteString("\n")
		}
	case *ast.String:
		w.esc(n.Value)
	case *ast.Emphasis:
		if n.Level >= 2 {
			w.wrap("<strong>", n, "</strong>")
		} else {
			w.wrap("<em>", n, "</em>")
		}
	case *east.Strikethrough:
		w.wrap("<del>", n, "</del>")
	case *ast.CodeSpan:
		w.code(n)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		w.b.WriteString("<pre><code>")
		lines := n.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			w.esc(seg.Value(w.src))
		}
		w.b.WriteString("</code></pre>\n")
	case *ast.List:
		w.list(n)
	case *ast.ListItem:
		w.wrap("<li>", n, "</li>\n")
	case *east.TaskCheckBox:
		if n.IsChecked {
			w.b.WriteString("☑ ")
		} else {
			w.b.WriteString("☐ ")
		}
	case *ast.Blockquote:
		w.wrap("<blockquote>\n", n, "</blockquote>\n")
	case *ast.ThematicBreak:
		w.b.WriteString("<hr>\n")
	case *ast.Link:
		w.link(decode(n.Destination), func() { w.children(n) })
	case *ast.AutoLink:
		dest := n.URL(w.src)
		if n.AutoLinkType == ast.AutoLinkEmail {
			dest = append([]byte("mailto:"), dest...)
		}
		w.link(dest, func() { w.esc(n.Label(w.src)) })
	case *east.Table:
		w.wrap("<table>\n", n, "</tbody>\n</table>\n")
	case *east.TableHeader:
		w.wrap("<thead>\n<tr>", n, "</tr>\n</thead>\n<tbody>\n")
	case *east.TableRow:
		w.wrap("<tr>", n, "</tr>\n")
	case *east.TableCell:
		w.cell(n)
	case *ast.Image, *ast.RawHTML, *ast.HTMLBlock:
		// Refused by validation; never written, not even as text.
	default:
		w.children(n)
	}
}

func (w *walker) list(n *ast.List) {
	if !n.IsOrdered() {
		w.wrap("<ul>\n", n, "</ul>\n")
		return
	}
	open := "<ol>\n"
	if n.Start != 1 {
		open = `<ol start="` + strconv.Itoa(n.Start) + `">` + "\n"
	}
	w.wrap(open, n, "</ol>\n")
}

// link writes text, the link's words, inside an anchor when dest is a web or
// mail address and bare otherwise. The address is escaped as an attribute
// value, so the browser reads back exactly the bytes checked here.
func (w *walker) link(dest []byte, text func()) {
	if !webAddress(string(dest)) {
		text()
		return
	}
	w.b.WriteString(`<a href="`)
	w.esc(dest)
	w.b.WriteString(`" rel="noopener noreferrer">`)
	text()
	w.b.WriteString("</a>")
}

// webAddress holds for an http, https or mailto address with something after
// the scheme and no space or control character a browser would strip before
// it reads the scheme. Validation applies the stricter rule; this one stands
// on its own so the walker never writes another scheme whatever it is given.
func webAddress(dest string) bool {
	if strings.ContainsFunc(dest, func(r rune) bool { return r <= ' ' || (r >= 0x7f && r <= 0x9f) }) {
		return false
	}
	lower := strings.ToLower(dest)
	for _, scheme := range []string{"https://", "http://", "mailto:"} {
		if rest, ok := strings.CutPrefix(lower, scheme); ok {
			return rest != "" && !strings.HasPrefix(rest, "/")
		}
	}
	return false
}

// cell is a th in the header and a td below it, aligned by one of three
// constant class names.
func (w *walker) cell(n *east.TableCell) {
	tag := "td"
	if _, head := n.Parent().(*east.TableHeader); head {
		tag = "th"
	}
	align := "text-left"
	switch n.Alignment {
	case east.AlignCenter:
		align = "text-center"
	case east.AlignRight:
		align = "text-right"
	}
	w.wrap("<"+tag+` class="`+align+`">`, n, "</"+tag+">")
}

// code writes a code span's text as it stands, a line end read as a space.
func (w *walker) code(n *ast.CodeSpan) {
	w.b.WriteString("<code>")
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			v := t.Segment.Value(w.src)
			if n := len(v); n > 0 && v[n-1] == '\n' {
				v = append(v[:n-1:n-1], ' ')
			}
			w.esc(v)
		}
	}
	w.b.WriteString("</code>")
}

// decode reads Markdown's two escapes in one pass, as CommonMark does: a
// backslash before punctuation is the punctuation, and an entity or numeric
// reference is its character. What comes out is still escaped on the way to
// the page; decoding first only makes "&amp;" read as "&" rather than as
// itself.
func decode(v []byte) []byte {
	out := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c == '\\' && i+1 < len(v) && util.IsPunct(v[i+1]):
			i++
			out = append(out, v[i])
		case c == '&':
			end := strings.IndexByte(string(v[i:min(len(v), i+40)]), ';')
			if end < 0 {
				out = append(out, c)
				continue
			}
			ref := v[i : i+end+1]
			if r := util.ResolveNumericReferences(util.ResolveEntityNames(ref)); string(r) != string(ref) {
				out = append(out, r...)
				i += end
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}
