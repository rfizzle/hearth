package deliverable

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gtext "github.com/yuin/goldmark/text"
)

// md reads Markdown with tables, strikethrough and task lists. It has no
// linkify and no footnotes, and it never renders: raw HTML stays a node here so
// that markdown can refuse it, and no renderer is given the means to write it.
var md = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList)).Parser()

// ParseMarkdown parses src with the strict reader every renderer shares. A
// renderer walks the result and writes only the elements the catalogue allows.
// Text the caller did not write passes CheckMarkdown first.
func ParseMarkdown(src []byte) ast.Node {
	return md.Parse(gtext.NewReader(src))
}

// DemoteHeading is the level a heading in a text widget is drawn at: the
// document's title is the only h1, so a # reads as ##.
func DemoteHeading(level int) int {
	return max(level, 2)
}

// markdown checks the length and characters of a text or callout's Markdown and
// walks its tree for what is refused: raw HTML, an image, a link that is not a
// web or mail address.
func markdown(noun, src string, hi int) error {
	if err := plain("the "+noun, src, true); err != nil {
		return err
	}
	if err := count("the "+noun, utf8.RuneCountInString(src), 1, hi, "characters"); err != nil {
		return err
	}
	b := []byte(src)
	if err := CheckMarkdown(b); err != nil {
		return refuse("the %s: %v; write it more simply", noun, err)
	}
	return ast.Walk(ParseMarkdown(b), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		return ast.WalkContinue, checkNode(noun, n, b)
	})
}

func checkNode(noun string, n ast.Node, src []byte) error {
	switch n := n.(type) {
	case *ast.HTMLBlock:
		if n.Lines().Len() > 0 {
			seg := n.Lines().At(0)
			return htmlRefusal(noun, seg.Value(src))
		}
		return htmlRefusal(noun, nil)
	case *ast.RawHTML:
		if n.Segments.Len() > 0 {
			seg := n.Segments.At(0)
			return htmlRefusal(noun, seg.Value(src))
		}
		return htmlRefusal(noun, nil)
	case *ast.Image:
		return refuse("images are not part of a shaped deliverable yet; describe it or put it in a Markdown file")
	case *ast.Link:
		return checkLink(string(n.Destination))
	case *ast.AutoLink:
		if n.AutoLinkType == ast.AutoLinkEmail {
			return checkLink("mailto:" + string(n.URL(src)))
		}
		return checkLink(string(n.URL(src)))
	}
	return nil
}

func htmlRefusal(noun string, seen []byte) error {
	return refuse("the %s has HTML in it (`%s`); write Markdown only", noun, shorten(string(seen), 24))
}

// checkLink accepts a destination with an http, https or mailto scheme and
// nothing a browser might read differently: no space, tab or control
// character, and an address after the scheme.
func checkLink(dest string) error {
	u, err := url.Parse(dest)
	ok := err == nil && !strings.ContainsFunc(dest, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) })
	if ok {
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			ok = u.Host != ""
		case "mailto":
			ok = u.Opaque != ""
		default:
			ok = false
		}
	}
	if !ok {
		return refuse("the link %q is not a web address; links are http, https or mailto", shorten(dest, 24))
	}
	return nil
}

// shorten cuts s to n characters on one line without backticks, marking a cut.
func shorten(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '`' {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
