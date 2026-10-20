package office

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strings"
)

var (
	// ErrLimit is a file that unpacks to more than Limits allow, or nests
	// deeper than they allow.
	ErrLimit = errors.New("the file is larger or more deeply built inside than Hearth reads")
	// ErrDamaged is a file that is not a zip, or whose XML does not parse, or
	// that lists a part twice.
	ErrDamaged = errors.New("the file is damaged or not an Office file")
	// ErrPanic is a walker that panicked on the file.
	ErrPanic = errors.New("the file could not be walked")
	// errStop ends a walk early without an error.
	errStop = errors.New("stop")
)

// Limits are the caps on one archive. Default is what Hearth uses; tests use
// smaller ones.
type Limits struct {
	Entries  int   // entries in the zip
	Entry    int64 // decompressed bytes of one part
	Total    int64 // decompressed bytes of all the parts read from one archive
	Depth    int   // levels of XML nesting
	Elements int   // XML elements in one part
}

// Default is the plan's numbers: 10,000 entries, 64 MB a part, 256 MB in all,
// 100 levels, two million elements.
var Default = Limits{Entries: 10000, Entry: 64 << 20, Total: 256 << 20, Depth: 100, Elements: 2_000_000}

// checkEvery is how many tokens pass between looks at the context.
const checkEvery = 64

// Magic is how a zip begins.
var Magic = []byte("PK\x03\x04")

// Format is which Office file an archive is.
type Format string

// The formats Format tells apart. Excel is told apart only so a file named
// for one and built as another is refused in words.
const (
	Word       Format = "docx"
	PowerPoint Format = "pptx"
	Excel      Format = "xlsx"
)

// mainParts are the part and the content type that make each format.
var mainParts = []struct {
	format Format
	part   string
	typ    string
}{
	{Word, "/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"},
	{PowerPoint, "/ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"},
	{Excel, "/xl/workbook.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"},
}

// Archive is an open Office file.
type Archive struct {
	lim   Limits
	files map[string]*zip.File
	total int64
}

// Open reads the zip directory of f, size bytes long. It refuses a file with
// too many entries or a name twice.
func Open(f io.ReaderAt, size int64, lim Limits) (*Archive, error) {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return nil, ErrDamaged
	}
	if len(zr.File) > lim.Entries {
		return nil, ErrLimit
	}
	a := &Archive{lim: lim, files: make(map[string]*zip.File, len(zr.File))}
	for _, zf := range zr.File {
		if _, dup := a.files[zf.Name]; dup {
			return nil, ErrDamaged
		}
		a.files[zf.Name] = zf
	}
	return a, nil
}

// has reports whether the archive lists a part called name.
func (a *Archive) has(name string) bool { _, ok := a.files[name]; return ok }

// meter reads a part and stops, with crossed set, at the first byte past the
// part's cap or the archive's.
type meter struct {
	r       io.Reader
	a       *Archive
	n       int64
	crossed bool
}

func (m *meter) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	m.n += int64(n)
	m.a.total += int64(n)
	if m.n > m.a.lim.Entry || m.a.total > m.a.lim.Total {
		m.crossed = true
		return 0, ErrLimit
	}
	return n, err
}

// walk streams the XML of the part called name through fn. A header that
// claims more than the entry cap is refused without opening the part. fn is
// called for every token; the CharData it is given is valid only during the
// call. A walk ends early, without an error, when fn returns errStop.
func (a *Archive) walk(ctx context.Context, name string, fn func(xml.Token) error) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrPanic
		}
	}()
	if err := ctx.Err(); err != nil { // between parts
		return err
	}
	zf, ok := a.files[name]
	if !ok {
		return ErrDamaged
	}
	if zf.UncompressedSize64 > uint64(a.lim.Entry) {
		return ErrLimit
	}
	rc, err := zf.Open()
	if err != nil {
		return ErrDamaged
	}
	defer rc.Close()
	m := &meter{r: rc, a: a}
	d := xml.NewDecoder(m)
	var depth, elements, tokens int
	for {
		if tokens++; tokens%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		tok, err := d.Token()
		switch {
		case m.crossed:
			return ErrLimit
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return ErrDamaged
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			elements++
			if depth > a.lim.Depth || elements > a.lim.Elements {
				return ErrLimit
			}
		case xml.EndElement:
			depth--
		}
		if err := fn(tok); err != nil {
			if errors.Is(err, errStop) {
				return nil
			}
			return err
		}
	}
}

// Format is which Office file the archive is, from the content type its
// [Content_Types].xml gives the main part; "" when it is none of the three.
func (a *Archive) Format(ctx context.Context) (Format, error) {
	found := map[string]string{}
	err := a.each(ctx, "[Content_Types].xml", "Override", func(se xml.StartElement) {
		found[attr(se, "PartName")] = attr(se, "ContentType")
	})
	if err != nil {
		return "", err
	}
	for _, p := range mainParts {
		if found[p.part] == p.typ && a.has(strings.TrimPrefix(p.part, "/")) {
			return p.format, nil
		}
	}
	return "", nil
}

// each calls fn for every element called local in the part called name.
func (a *Archive) each(ctx context.Context, name, local string, fn func(xml.StartElement)) error {
	return a.walk(ctx, name, func(t xml.Token) error {
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == local {
			fn(se)
		}
		return nil
	})
}

// attr is the value of the attribute whose local name is local, or "".
func attr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// target is the part a relationship in the folder base points at, as a key
// into the archive.
func target(base, t string) string {
	if rest, ok := strings.CutPrefix(t, "/"); ok {
		return path.Clean(rest)
	}
	return path.Join(base, t)
}
