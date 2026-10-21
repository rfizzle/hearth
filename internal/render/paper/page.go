package paper

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-pdf/fpdf"

	"github.com/rfizzle/hearth/internal/render/fonts"
)

// Paper is the sheet a document is set on.
type Paper string

// The papers. Letter is the default; an empty Paper is Letter.
const (
	Letter Paper = "letter"
	A4     Paper = "a4"
)

// ParsePaper is s as a Paper: "letter", "a4", or empty for Letter.
func ParsePaper(s string) (Paper, error) {
	switch p := Paper(strings.ToLower(s)); p {
	case "", Letter:
		return Letter, nil
	case A4:
		return A4, nil
	}
	return "", fmt.Errorf("paper must be letter or a4, not %q", s)
}

// MaxPages is the most pages a document is drawn on, unless Limit sets
// fewer. A longer one stops at the end of the last page.
const MaxPages = 200

// ErrRender is a document fpdf could not finish. It says nothing of the
// document's text.
var ErrRender = errors.New("the PDF could not be made from this document")

// Colour is an sRGB colour, each channel 0 to 255.
type Colour struct{ R, G, B int }

// Style is the print form of docs/design.md: its type roles and spacing
// steps in points, and its light neutral colours. Every size and gap a page
// draws with comes from here.
type Style struct {
	Margin    float64    // all four edges: 1 in
	Body      float64    // body text
	Leading   float64    // body line height
	Headings  [6]float64 // # to ######: large title, title, headline, then body size in bold
	Code      float64    // code, at callout size
	CodeLeft  float64    // code line height
	Caption   float64    // the page number
	Gap       float64    // between blocks
	Indent    float64    // a list level, a quote
	Pad       float64    // inside the code box and a table cell
	Hairline  float64    // a rule, a quote's edge, a table's lines
	Ink       Colour     // text
	Secondary Colour     // a link's address, a placeholder, the page number
	Fill      Colour     // behind code
	Rule      Colour     // hairlines
}

// Print is the style a document is set in.
// See docs/design.md#type.
func Print() Style {
	return Style{
		Margin: 72, Body: 11, Leading: 15.4,
		Headings: [6]float64{26, 20, 14, 11, 11, 11},
		Code:     9.5, CodeLeft: 13, Caption: 9,
		Gap: 8, Indent: 16, Pad: 8, Hairline: 0.5,
		Ink:       Colour{0x1D, 0x1D, 0x1F},
		Secondary: Colour{0x5E, 0x5E, 0x63},
		Fill:      Colour{0xF2, 0xF2, 0xF4},
		Rule:      Colour{0xC7, 0xC7, 0xCC},
	}
}

// Document is one PDF being drawn: an fpdf page set up with the embedded
// fonts, the style's margins, a "1 of 3" footer and the page cap. A
// renderer draws through PDF and the helpers here, then calls Finish.
type Document struct {
	Style Style
	pdf   *fpdf.Fpdf
	full  bool
	limit int
	bars  []bar
}

// bar is a quote's left edge, open from y on the current page.
type bar struct{ x, y float64 }

// New is an empty document on paper, portrait or landscape, with its first
// page added.
func New(p Paper, landscape bool, s Style) (*Document, error) {
	size, orient := "Letter", "P"
	if p == A4 {
		size = "A4"
	}
	if landscape {
		orient = "L"
	}
	f := fpdf.New(orient, "pt", size, "")
	for _, face := range fonts.Faces() {
		if len(face.Data) == 0 {
			return nil, ErrRender
		}
		f.AddUTF8FontFromBytes(face.Family, face.Style, face.Data)
	}
	if f.Err() {
		return nil, ErrRender
	}
	d := &Document{Style: s, pdf: f, limit: MaxPages}
	f.SetMargins(s.Margin, s.Margin, s.Margin)
	f.SetAutoPageBreak(true, s.Margin)
	f.SetCellMargin(0)
	f.SetCatalogSort(true) // the same order every time: see Finish
	f.SetCompression(true)
	f.SetProducer("Hearth", false)
	f.SetCreator("Hearth", false)
	f.AliasNbPages("{nb}")
	f.SetAcceptPageBreakFunc(d.accept)
	f.SetFooterFunc(d.footer)
	f.AddPage()
	d.Font(fonts.Sans, "", s.Body)
	d.Ink(s.Ink)
	return d, nil
}

// PDF is the fpdf document, for drawing what the helpers here do not.
func (d *Document) PDF() *fpdf.Fpdf { return d.pdf }

// Full reports whether the document reached its page limit and refused
// another. A renderer stops drawing once it is.
func (d *Document) Full() bool { return d.full }

// Limit sets the most pages the document is drawn on, at most MaxPages.
func (d *Document) Limit(pages int) { d.limit = min(max(pages, 1), MaxPages) }

// Width is the width between the current left margin and the right one.
func (d *Document) Width() float64 {
	w, _ := d.pdf.GetPageSize()
	l, _, r, _ := d.pdf.GetMargins()
	return w - l - r
}

// Font sets the face and size. A style the fonts lack falls back to the
// nearest one they have: bold for bold italic, regular for anything in
// code. U and S (underline, strikeout) pass through.
func (d *Document) Font(family, style string, size float64) {
	deco := strings.Map(func(r rune) rune {
		if r == 'U' || r == 'S' {
			return r
		}
		return -1
	}, style)
	face := ""
	switch {
	case family == fonts.Mono:
	case strings.Contains(style, "B"):
		face = "B"
	case strings.Contains(style, "I"):
		face = "I"
	}
	d.pdf.SetFont(family, face+deco, size)
}

// Ink sets the colour text is drawn in.
func (d *Document) Ink(c Colour) { d.pdf.SetTextColor(c.R, c.G, c.B) }

// Space moves down by h, or to the top of the next page when h would cross
// the bottom margin. At the top of a page it does nothing, so a page never
// begins with a gap.
func (d *Document) Space(h float64) {
	_, top, _, _ := d.pdf.GetMargins()
	if d.pdf.GetY() <= top {
		return
	}
	d.Need(h)
	if d.pdf.GetY() > top {
		d.pdf.SetY(d.pdf.GetY() + h)
	}
}

// Need starts a new page unless h more points fit above the bottom margin,
// the one fpdf breaks pages at.
func (d *Document) Need(h float64) {
	_, ph := d.pdf.GetPageSize()
	_, _, _, bottom := d.pdf.GetMargins()
	if d.pdf.GetY()+h > ph-bottom && d.accept() {
		d.pdf.AddPage()
	}
}

// Rule draws a hairline across the column at the current height.
func (d *Document) Rule() {
	l, _, _, _ := d.pdf.GetMargins()
	y := d.pdf.GetY()
	d.hairline(l, y, l+d.Width(), y)
}

func (d *Document) hairline(x1, y1, x2, y2 float64) {
	c := d.Style.Rule
	d.pdf.SetDrawColor(c.R, c.G, c.B)
	d.pdf.SetLineWidth(d.Style.Hairline)
	d.pdf.Line(x1, y1, x2, y2)
}

// OpenBar starts a quote's left edge at x from the current height; CloseBar
// ends the innermost one. An edge that crosses a page is drawn to the bottom
// of each page it leaves.
func (d *Document) OpenBar(x float64) { d.bars = append(d.bars, bar{x, d.pdf.GetY()}) }

// CloseBar ends the innermost edge at the current height.
func (d *Document) CloseBar() {
	if len(d.bars) == 0 {
		return
	}
	b := d.bars[len(d.bars)-1]
	d.bars = d.bars[:len(d.bars)-1]
	if y := d.pdf.GetY(); y > b.y {
		d.hairline(b.x, b.y, b.x, y)
	}
}

// accept is fpdf's page-break check: it closes the open edges on this page
// and refuses a page past the limit.
func (d *Document) accept() bool {
	if d.pdf.PageCount() >= d.limit {
		d.full = true
		return false
	}
	_, ph := d.pdf.GetPageSize()
	_, top, _, bottom := d.pdf.GetMargins()
	for i, b := range d.bars {
		d.hairline(b.x, b.y, b.x, ph-bottom)
		d.bars[i].y = top
	}
	return true
}

// footer is "1 of 3", centred in the bottom margin.
func (d *Document) footer() {
	_, ph := d.pdf.GetPageSize()
	d.pdf.SetY(ph - d.Style.Margin/2)
	d.Font(fonts.Sans, "", d.Style.Caption)
	d.Ink(d.Style.Secondary)
	d.pdf.CellFormat(0, d.Style.Caption, fmt.Sprintf("%d of {nb}", d.pdf.PageNo()), "", 0, "C", false, 0, "")
}

// Finish closes the document and returns its bytes and page count. The
// title is the document's, and modified its source's modification time,
// which becomes both of the PDF's dates: fpdf would otherwise read the
// clock, and the same source must give the same bytes.
// See docs/decisions.md for D39.
func (d *Document) Finish(title string, modified time.Time) ([]byte, int, error) {
	if modified.IsZero() {
		modified = time.Unix(0, 0)
	}
	modified = modified.UTC().Truncate(time.Second)
	d.pdf.SetCreationDate(modified)
	d.pdf.SetModificationDate(modified)
	if title = Clean(title); title != "" {
		d.pdf.SetTitle(title, true)
	}
	var buf bytes.Buffer
	if err := d.pdf.Output(&buf); err != nil {
		return nil, 0, ErrRender
	}
	return buf.Bytes(), d.pdf.PageCount(), nil
}

// Clean is s as the fonts can draw it on one line: a tab is a space, other
// control and format characters and line breaks are dropped, and a
// character past the Basic Multilingual Plane, which fpdf's width tables do
// not reach, is the replacement character.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), r == ' ', r == ' ':
			return -1
		case r > 0xFFFD:
			return '�'
		}
		return r
	}, s)
}
