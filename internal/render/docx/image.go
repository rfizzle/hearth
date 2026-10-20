package docx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/url"
	"strings"

	_ "image/jpeg" // image.DecodeConfig reads a JPEG's header

	"github.com/yuin/goldmark/ast"
)

// The limits on images in one document.
// See docs/decisions.md for D41.
const (
	maxImages     = 20
	maxPixels     = 20_000_000
	maxImageBytes = 20 << 20
	// maxImageTotal bounds the image data one file carries, as written into it.
	maxImageTotal = 40 << 20
	// The widest and tallest a picture is set, in English Metric Units: 6.5
	// and 8 inches.
	maxWidthEMU  = 5943600
	maxHeightEMU = 7315200
	emuPerPixel  = 9525 // 96 pixels an inch
)

// media is an image kept for the package.
type media struct {
	name string
	data []byte
}

// errUnusable is an image left out for a reason the second notice names.
var errUnusable = errors.New("unusable image")

// errRemote is an image named by an address rather than a path.
var errRemote = errors.New("remote image")

// image writes the image n names as an inline picture, or an italic
// placeholder with its description and a notice.
func (w *walker) image(n *ast.Image, l look) {
	drawing, err := w.place(string(n.Destination), plain(n, w.src))
	switch {
	case err == nil:
		w.put(drawing)
		return
	case errors.Is(err, errRemote):
		w.notice("[%s has images from the web; they were left out of the Word file. Save each image in the folder and name it by its path.]")
	default:
		w.notice("[%s has images that could not be used, marked in the Word file; an image must be a PNG or JPEG file inside this folder, at most 20 megapixels and 20 to a document.]")
	}
	label := "[Image]"
	if alt := plain(n, w.src); alt != "" {
		label = "[Image: " + alt + "]"
	}
	l.italic = true
	w.run(label, l)
}

// place loads the image dest names and keeps it as a part of the package,
// returning the run that shows it. Only a relative path is tried, and only
// through the Opener; an address with a scheme or a host, an absolute path
// and a path with a backslash are refused before anything is opened.
func (w *walker) place(dest, alt string) (string, error) {
	if u, err := url.Parse(dest); err == nil && (u.Scheme != "" || u.Host != "") {
		return "", errRemote
	}
	rel, err := url.PathUnescape(dest)
	if err != nil || rel == "" || strings.HasPrefix(rel, "/") || strings.ContainsAny(rel, "\\\x00") {
		return "", errUnusable
	}
	if w.o.Image == nil || w.images >= maxImages || w.ctx.Err() != nil {
		return "", errUnusable
	}
	data, ext, cfg, err := load(w.o.Image, rel)
	if err != nil {
		return "", err
	}
	if w.bytes += len(data); w.bytes > maxImageTotal {
		return "", errUnusable
	}
	w.images++
	w.docPr++
	name := fmt.Sprintf("image%d.%s", w.images, ext)
	w.media = append(w.media, media{name: name, data: data})
	cx, cy := int64(cfg.Width)*emuPerPixel, int64(cfg.Height)*emuPerPixel
	if cx > maxWidthEMU {
		cx, cy = maxWidthEMU, cy*maxWidthEMU/cx
	}
	if cy > maxHeightEMU {
		cx, cy = cx*maxHeightEMU/cy, maxHeightEMU
	}
	cx, cy = max(cx, 1), max(cy, 1)
	return fmt.Sprintf(`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%[1]d" cy="%[2]d"/>`+
		`<wp:docPr id="%[3]d" name="Picture %[3]d" descr="%[4]s"/>`+
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>`+
		`<a:graphic><a:graphicData uri="%[5]s"><pic:pic><pic:nvPicPr><pic:cNvPr id="%[3]d" name="%[6]s"/><pic:cNvPicPr/></pic:nvPicPr>`+
		`<pic:blipFill><a:blip r:embed="rId%[7]d"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%[1]d" cy="%[2]d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`+
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`,
		cx, cy, w.docPr, escape(alt), nsPic, name, firstImage+len(w.media)-1), nil
}

// load reads the image at rel through open, at most maxImageBytes, and checks
// its header: a PNG or JPEG of at most maxPixels. A JPEG goes into the file as
// it is; a PNG is decoded and written again, so the Word reader never inflates
// a file from the folder, and the decode allocates what the checked header
// says.
func load(open Opener, rel string) ([]byte, string, image.Config, error) {
	data, err := fetch(open, rel)
	if err != nil {
		return nil, "", image.Config{}, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	ok := err == nil && cfg.Width > 0 && cfg.Height > 0 && cfg.Width*cfg.Height <= maxPixels
	switch {
	case !ok:
		return nil, "", cfg, errUnusable
	case format == "jpeg":
		return data, "jpg", cfg, nil
	case format != "png":
		return nil, "", cfg, errUnusable
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", cfg, errUnusable
	}
	var out bytes.Buffer
	if err := png.Encode(&out, src); err != nil {
		return nil, "", cfg, errUnusable
	}
	return out.Bytes(), "png", cfg, nil
}

// fetch reads the file rel names through open, at most maxImageBytes.
func fetch(open Opener, rel string) ([]byte, error) {
	rc, err := open(rel)
	if err != nil {
		return nil, errUnusable
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		return nil, errUnusable
	}
	return data, nil
}
