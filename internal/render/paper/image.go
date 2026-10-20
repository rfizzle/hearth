package paper

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/yuin/goldmark/ast"
)

// The limits on images in one document.
// See docs/decisions.md for D41.
const (
	maxImages     = 20
	maxPixels     = 20_000_000
	maxImageBytes = 20 << 20
	// maxImageTotal bounds the image data one PDF carries, as written into
	// it, so twenty large images cannot make a file of hundreds of megabytes.
	maxImageTotal = 40 << 20
)

// errUnusable is an image left out for a reason the second notice names.
var errUnusable = errors.New("unusable image")

// errRemote is an image named by an address rather than a path.
var errRemote = errors.New("remote image")

// image draws the image n names as a block of its own, scaled to the column,
// or a grey placeholder with its description and a notice.
func (w *walker) image(n *ast.Image) {
	err := w.place(string(n.Destination))
	switch {
	case err == nil:
		return
	case errors.Is(err, errRemote):
		w.notice("[%s has images from the web; they were left out of the PDF. Save each image in the folder and name it by its path.]")
	default:
		w.notice("[%s has images that could not be used, marked in the PDF; an image must be a PNG or JPEG file inside this folder, at most 20 megapixels and 20 to a document.]")
	}
	label := "[Image]"
	if alt := plain(n, w.src); alt != "" {
		label = "[Image: " + alt + "]"
	}
	w.d.Ink(w.d.Style.Secondary)
	w.text(label, "I")
	w.d.Ink(w.d.Style.Ink)
}

// place loads and draws the image dest names. Only a relative path is
// tried, and only through the Opener; an address with a scheme or a host,
// an absolute path and a path with a backslash are refused before anything
// is opened.
func (w *walker) place(dest string) error {
	if u, err := url.Parse(dest); err == nil && (u.Scheme != "" || u.Host != "") {
		return errRemote
	}
	rel, err := url.PathUnescape(dest)
	if err != nil || rel == "" || strings.HasPrefix(rel, "/") || strings.ContainsAny(rel, "\\\x00") {
		return errUnusable
	}
	if w.o.Image == nil || w.images >= maxImages || w.ctx.Err() != nil {
		return errUnusable
	}
	if w.widths == nil {
		w.widths = map[int]bool{}
	}
	data, kind, cfg, err := load(w.o.Image, rel, w.widths)
	if err != nil {
		return err
	}
	if w.imageBytes += len(data); w.imageBytes > maxImageTotal {
		return errUnusable
	}
	w.widths[cfg.Width] = true
	w.images++
	p := w.d.pdf
	name := fmt.Sprintf("image%d", w.images)
	opt := fpdf.ImageOptions{ImageType: kind}
	if p.RegisterImageOptionsReader(name, opt, bytes.NewReader(data)); p.Err() {
		p.ClearError()
		return errUnusable
	}
	l, _, _, _ := p.GetMargins()
	_, ph := p.GetPageSize()
	wd := min(float64(cfg.Width)*0.75, w.d.Width()) // 96 pixels an inch
	ht := wd * float64(cfg.Height) / float64(cfg.Width)
	if room := ph - 2*w.d.Style.Margin - w.d.Style.Gap; ht > room {
		wd, ht = wd*room/ht, room
	}
	if p.GetX() > l {
		p.Ln(w.lead)
	}
	w.d.Need(ht)
	if w.d.Full() {
		return nil
	}
	y := p.GetY()
	p.ImageOptions(name, l, y, wd, ht, false, opt, 0, "")
	p.SetXY(l, y+ht)
	w.d.Space(w.d.Style.Gap)
	return nil
}

// load reads the image at rel through open, at most maxImageBytes, and
// checks its header: a PNG or JPEG of at most maxPixels. A JPEG goes into
// the PDF as it is; a PNG is decoded, laid on white and written again as a
// plain eight-bit PNG, so fpdf's own PNG reader, which inflates without a
// limit, never sees a file from the folder.
//
// fpdf writes a document's images in the order of their widths and leaves
// two of one width in map order, which changes from run to run. So an image
// as wide as one already placed is widened by white columns on its right,
// invisible on the page, until its width is its own; a JPEG that needs it is
// encoded again.
func load(open Opener, rel string, widths map[int]bool) ([]byte, string, image.Config, error) {
	rc, err := open(rel)
	if err != nil {
		return nil, "", image.Config{}, errUnusable
	}
	data, err := io.ReadAll(io.LimitReader(rc, maxImageBytes+1))
	rc.Close()
	if err != nil || len(data) > maxImageBytes {
		return nil, "", image.Config{}, errUnusable
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, "", image.Config{}, errUnusable
	}
	width := cfg.Width
	for widths[width] {
		width++
	}
	if format == "jpeg" && width == cfg.Width {
		return data, "JPG", cfg, nil
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", image.Config{}, errUnusable
	}
	flat := image.NewRGBA(image.Rect(0, 0, width, cfg.Height))
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, src.Bounds().Sub(src.Bounds().Min), src, src.Bounds().Min, draw.Over)
	var out bytes.Buffer
	kind := "PNG"
	if format == "jpeg" {
		kind, err = "JPG", jpeg.Encode(&out, flat, &jpeg.Options{Quality: 92})
	} else {
		err = png.Encode(&out, flat)
	}
	if err != nil {
		return nil, "", image.Config{}, errUnusable
	}
	cfg.Width = width
	return out.Bytes(), kind, cfg, nil
}
