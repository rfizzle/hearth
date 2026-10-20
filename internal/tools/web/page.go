package web

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// droppedTags are the elements whose content is not the page's text: code,
// styling, and the navigation, header and footer repeated on every page.
const droppedTags = " script style noscript template nav header footer "

// blockTags are the elements that start a new line of text.
const blockTags = " address article aside blockquote br dd div dl dt figcaption figure " +
	"h1 h2 h3 h4 h5 h6 hr li main ol p pre section table td th title tr ul "

// isElement reports whether n is an element named in tags, in any
// namespace, so a script inside an SVG is dropped like any other.
func isElement(n *html.Node, tags string) bool {
	return n.Type == html.ElementNode && n.Data != "" && strings.Contains(tags, " "+n.Data+" ")
}

// pageText is the readable text of p: HTML reduced to its text, other text
// as it came. A page that is not text is refused.
func pageText(p Page) (string, error) {
	media, _, err := mime.ParseMediaType(p.ContentType)
	if err != nil || media == "" {
		media, _, _ = mime.ParseMediaType(http.DetectContentType(p.Body))
	}
	switch {
	case media == "text/html" || media == "application/xhtml+xml":
		return reduceHTML(decode(p)), nil
	case strings.HasPrefix(media, "text/"):
		return strings.ToValidUTF8(string(decode(p)), ""), nil
	case media == "application/json" || media == "application/xml" ||
		strings.HasSuffix(media, "+json") || strings.HasSuffix(media, "+xml"):
		return strings.ToValidUTF8(string(p.Body), ""), nil
	}
	return "", fmt.Errorf("%s is %s, not a page of text; web.fetch reads text only", p.URL, media)
}

// decode is p's body in UTF-8, read by the charset the response names, then
// the page's own declaration, and UTF-8 when neither says. A body that
// cannot be decoded is left as it came.
func decode(p Page) []byte {
	r, err := charset.NewReader(bytes.NewReader(p.Body), p.ContentType)
	if err != nil {
		return p.Body
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return p.Body
	}
	return out
}

// reduceHTML is the text of an HTML document, one line per block, with the
// dropped elements left out.
func reduceHTML(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var b strings.Builder
	writeText(&b, doc)
	return tidy(strings.ToValidUTF8(b.String(), ""))
}

// writeText writes n's text to b. The parser caps how deep a document
// nests, so the recursion is bounded.
func writeText(b *strings.Builder, n *html.Node) {
	switch {
	case n.Type == html.TextNode:
		b.WriteString(collapse(n.Data))
		return
	case n.Type == html.CommentNode, isElement(n, droppedTags):
		return
	}
	block := isElement(n, blockTags)
	if block {
		b.WriteByte('\n')
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeText(b, c)
	}
	if block {
		b.WriteByte('\n')
	}
}

// collapse folds each run of white space in s to one space, keeping one at
// either end if s had one there, so inline text still reads as words.
func collapse(s string) string {
	words := strings.Join(strings.Fields(s), " ")
	if words == "" {
		if s == "" {
			return ""
		}
		return " "
	}
	if strings.TrimLeft(s, " \t\n\r\f") != s {
		words = " " + words
	}
	if strings.TrimRight(s, " \t\n\r\f") != s {
		words += " "
	}
	return words
}

// tidy trims every line and keeps at most one blank line between two lines
// of text.
func tidy(s string) string {
	var out []string
	blank := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			blank = len(out) > 0
			continue
		}
		if blank {
			out = append(out, "")
			blank = false
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// cut is s cut to at most limit bytes at a character boundary, and whether
// it was cut. s is valid UTF-8.
func cut(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	s = s[:limit]
	i := len(s) - 1
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	if i >= 0 && !utf8.FullRuneInString(s[i:]) {
		s = s[:i]
	}
	return s, true
}
