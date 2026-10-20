package document

import (
	"fmt"
	"path/filepath"
	"strings"
)

// row is one kind of document: what it is called in a refusal, the
// extensions that name it, the check its first bytes must pass, and its
// reader. A new kind is a row appended to rows, beside its constant.
type row struct {
	kind  Kind
	label string
	exts  []string
	// sniff reports whether head, the first bytes of the file, are this kind.
	sniff func(head []byte) bool
	read  reader
}

// rows is every kind, one line each. Append; do not reorder.
var rows = []row{
	{KindPDF, "a PDF", []string{".pdf"}, sniffPDF, readPDF},
}

// unsupported are formats that are not read; Detect says to export them.
var unsupported = map[string]bool{
	".doc": true, ".xls": true, ".ppt": true,
	".pages": true, ".numbers": true, ".key": true,
}

// refusal is a refusal written as a sentence for the model to read, so it
// begins with a capital and ends with a full stop as an error string does not.
type refusal string

func (r refusal) Error() string { return string(r) }

func lookup(k Kind) (row, bool) {
	for _, r := range rows {
		if r.kind == k {
			return r, true
		}
	}
	return row{}, false
}

// Detect is the kind of the file called name whose first bytes are head: the
// extension first, then the bytes must agree. A name with no extension is
// sniffed; any other file is text, as it always was. A file whose name and
// bytes disagree is refused in words the model can read.
func Detect(name string, head []byte) (Kind, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if unsupported[ext] {
		return "", refusal(fmt.Sprintf("Hearth can't read %s files yet; ask the owner to export it as PDF or Word.", ext))
	}
	for _, r := range rows {
		for _, e := range r.exts {
			if e == ext {
				if !r.sniff(head) {
					return "", refusal(fmt.Sprintf("%s does not look like %s.", name, r.label))
				}
				return r.kind, nil
			}
		}
	}
	if ext == "" {
		for _, r := range rows {
			if r.sniff(head) {
				return r.kind, nil
			}
		}
	}
	return KindText, nil
}
