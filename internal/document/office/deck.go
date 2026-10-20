package office

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
)

// Deck is the slides of a PowerPoint file, in the order the deck shows them.
type Deck struct {
	a     *Archive
	parts []string
}

// Deck lists the slides: the ids in ppt/presentation.xml, in order, looked up
// in its relationships. A slide the relationships do not lead to a part for
// is dropped.
func (a *Archive) Deck(ctx context.Context) (*Deck, error) {
	targets := map[string]string{}
	err := a.each(ctx, "ppt/_rels/presentation.xml.rels", "Relationship", func(se xml.StartElement) {
		targets[attr(se, "Id")] = target("ppt", attr(se, "Target"))
	})
	if err != nil {
		return nil, err
	}
	d := &Deck{a: a}
	err = a.each(ctx, "ppt/presentation.xml", "sldId", func(se xml.StartElement) {
		for _, at := range se.Attr {
			// r:id, in the relationships namespace; the plain id is a number.
			if at.Name.Local == "id" && strings.HasSuffix(at.Name.Space, "/relationships") {
				if part := targets[at.Value]; strings.HasPrefix(part, "ppt/") && a.has(part) {
					d.parts = append(d.parts, part)
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Len is the number of slides.
func (d *Deck) Len() int { return len(d.parts) }

// Slide is the text of slide n, counted from 1: one line per paragraph, a
// paragraph at a lower level indented under a dash. Speaker notes live in
// another part and are not read. It stops once maxText bytes are written.
func (d *Deck) Slide(ctx context.Context, n, maxText int) (string, error) {
	if n < 1 || n > len(d.parts) {
		return "", ErrDamaged
	}
	var lines []string
	var line strings.Builder
	var level int
	var inT, indented bool
	size := 0
	err := d.a.walk(ctx, d.parts[n-1], func(t xml.Token) error {
		switch t := t.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				line.Reset()
				level, indented = 0, false
			case "pPr":
				level, _ = strconv.Atoi(attr(t, "lvl"))
				indented = level > 0
			case "t":
				inT = true
			case "br":
				line.WriteString("\n")
			}
		case xml.CharData:
			if inT {
				line.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inT = false
			case "p":
				text := strings.TrimSpace(strings.ReplaceAll(line.String(), "\n", " "))
				if text == "" {
					return nil
				}
				if indented {
					text = strings.Repeat("  ", min(level, 8)-1) + "- " + text
				}
				lines = append(lines, text)
				if size += len(text) + 1; size > maxText {
					return errStop
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return Cut(strings.Join(lines, "\n"), maxText), nil
}
