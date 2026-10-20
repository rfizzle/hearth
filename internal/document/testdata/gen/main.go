//go:build ignore

// Command gen writes the PDF fixtures the document tests read, so no document
// is downloaded and each file can be rebuilt. From the repository root:
//
//	go run internal/document/testdata/gen/main.go
//
// text3.pdf has three pages of text. scan.pdf has one page with an image and
// no text operators. truncated.pdf is text3.pdf cut off before its trailer.
// empty-password.pdf is text3.pdf encrypted with RC4 (40-bit, revision 2)
// under an empty user password, which a reader opens without asking.
// locked.pdf is the same under the user password "hearth-test", which a
// reader that does not know it refuses. The encryption is the standard
// security handler of the PDF specification, section 7.6, written out here.
// prev-loop.pdf is text3.pdf with a trailer that points at its own table.
// long.pdf has twelve pages of about 8 KB of text each, deflated, so that a
// read has more than one window of text to fill.
package main

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/rc4"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// pad is the 32 bytes the standard security handler pads a password with.
var pad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

var pages = []string{
	"Page one of the fixture. Hearth reads this text layer.",
	"Second page: the quarterly figures are in the table below.",
	"Third page: nothing more to add.",
}

// id is the file identifier the encryption key is derived from.
var id = []byte("hearth-fixture-1")

// cipher encrypts the stream of object n; nil leaves the file plain.
type cipher func(n int, data []byte) []byte

func main() {
	out := flag.String("out", "internal/document/testdata", "the folder to write into")
	flag.Parse()
	plain := textPDF(pages, nil, "")
	files := map[string][]byte{
		"text3.pdf":          plain,
		"long.pdf":           textPDF(longPages(), nil, ""),
		"scan.pdf":           scanPDF(),
		"truncated.pdf":      plain[:len(plain)*6/10],
		"prev-loop.pdf":      selfPrev(plain),
		"empty-password.pdf": encrypted(""),
		"locked.pdf":         encrypted("hearth-test"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(*out, name), data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

// textPDF is the three-page document. encrypt, when set, is the text of the
// Encrypt dictionary, which becomes the last object.
func textPDF(pages []string, c cipher, encrypt string) []byte {
	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 4+2*i)
	}
	objs := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(pages))),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
	for i, text := range pages {
		n := 5 + 2*i
		objs = append(objs,
			[]byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 3 0 R >> >> >>", n)),
			textStream(text, n, c))
	}
	return assemble(objs, encrypt)
}

// selfPrev is a document whose trailer names its own cross-reference table as
// the previous one, which a reader that follows the chain without a check
// follows for ever. The trailer comes after the table, so no offset moves.
func selfPrev(doc []byte) []byte {
	i := bytes.LastIndex(doc, []byte("startxref\n"))
	var xref int
	if _, err := fmt.Sscanf(string(doc[i+len("startxref\n"):]), "%d", &xref); err != nil {
		log.Fatal(err)
	}
	root := []byte("/Root 1 0 R")
	j := bytes.LastIndex(doc, root) + len(root)
	out := append([]byte{}, doc[:j]...)
	out = append(out, fmt.Sprintf(" /Prev %d", xref)...)
	return append(out, doc[j:]...)
}

// textStream is a page's content, deflated when it is long.
func textStream(text string, n int, c cipher) []byte {
	content := []byte(fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text))
	if len(content) < 1000 {
		return stream("", content, n, c)
	}
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(content)
	w.Close()
	return stream("/Filter /FlateDecode", z.Bytes(), n, c)
}

// longPages are twelve pages of about 8 KB of words.
func longPages() []string {
	out := make([]string, 12)
	for i := range out {
		out[i] = fmt.Sprintf("Long page %d. ", i+1) + strings.Repeat(fmt.Sprintf("Words on page %d go on. ", i+1), 340)
	}
	return out
}

// scanPDF is one page that draws an 8 by 8 grey image and shows no text.
func scanPDF() []byte {
	img := bytes.Repeat([]byte{0x80, 0xC0}, 32)
	return assemble([][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /XObject << /Im1 5 0 R >> >> >>"),
		stream("", []byte("q 400 0 0 400 100 200 cm /Im1 Do Q"), 4, nil),
		stream("/Type /XObject /Subtype /Image /Width 8 /Height 8 /ColorSpace /DeviceGray /BitsPerComponent 8", img, 5, nil),
	}, "")
}

func stream(dict string, data []byte, n int, c cipher) []byte {
	if c != nil {
		data = c(n, data)
	}
	return []byte(fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data))
}

// assemble numbers the objects from 1, adds the Encrypt dictionary after
// them when there is one, and writes the cross-reference table.
func assemble(objs [][]byte, encrypt string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	var offs []int
	for i, o := range objs {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	trailer := fmt.Sprintf("/Size %d /Root 1 0 R", len(objs)+1)
	if encrypt != "" {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(objs)+1, encrypt)
		trailer = fmt.Sprintf("/Size %d /Root 1 0 R /Encrypt %d 0 R /ID [<%x> <%x>]", len(objs)+2, len(objs)+1, id, id)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< %s >>\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
	return b.Bytes()
}

// encrypted is the text document under RC4-40 with the given user password.
func encrypted(user string) []byte {
	pw := func(s string) []byte { return append([]byte(s), pad...)[:32] }
	ownerKey := md5.Sum(pw("owner"))
	o := rc4Sum(ownerKey[:5], pw(user))
	h := md5.New()
	h.Write(pw(user))
	h.Write(o)
	h.Write([]byte{0xFC, 0xFF, 0xFF, 0xFF}) // the permissions, -4, as a little-endian 32-bit integer
	h.Write(id)
	key := h.Sum(nil)[:5]
	u := rc4Sum(key, pad)
	c := func(n int, data []byte) []byte {
		k := md5.Sum(append(append([]byte{}, key...), byte(n), byte(n>>8), byte(n>>16), 0, 0))
		return rc4Sum(k[:10], data)
	}
	dict := fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /P -4 /O <%x> /U <%x> >>", o, u)
	return textPDF(pages, c, dict)
}

func rc4Sum(key, data []byte) []byte {
	c, err := rc4.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out
}
