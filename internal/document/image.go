package document

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"

	// The decoders whose headers DecodeConfig reads. Nothing here decodes a
	// pixel.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// MaxPixels is the largest picture a read describes: 50 megapixels, judged
// from the header before anything decodes.
const MaxPixels = 50_000_000

// The image kinds. Each is its own row so a name and the bytes can disagree
// in words that name the format.
const (
	KindPNG  Kind = "png"
	KindJPEG Kind = "jpeg"
	KindGIF  Kind = "gif"
	KindWebP Kind = "webp"
)

func sniffPNG(head []byte) bool  { return bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")) }
func sniffJPEG(head []byte) bool { return bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}) }
func sniffGIF(head []byte) bool {
	return bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a"))
}
func sniffWebP(head []byte) bool {
	return len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP"))
}

// formatNames is how the description names each format, by the name the
// decoder registers.
var formatNames = map[string]string{"png": "a PNG", "jpeg": "a JPEG", "gif": "a GIF", "webp": "a WebP"}

// readImage describes the picture: its format and its size in pixels, from
// the header alone. A picture over MaxPixels is refused before any decoder
// runs past its header. The text in it and what it shows are asked of the
// app's Recogniser, as page 1; the notice says whether the app read it, found
// no text, could not, or was not open.
func readImage(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	if err := refusePages(o, name, "a picture"); err != nil {
		return Document{}, err
	}
	name = oneLine(name)
	cfg, format, err := image.DecodeConfig(io.NewSectionReader(f, 0, size))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return Document{}, fmt.Errorf("%s could not be read as a picture; it may be damaged or cut short", name)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return Document{}, fmt.Errorf("%s is %d × %d pixels, larger than the %d megapixels Hearth looks at", name, cfg.Width, cfg.Height, MaxPixels/1_000_000)
	}
	kind, ok := formatNames[format]
	if !ok {
		kind = "a picture"
	}
	more, notice := recogniseImage(ctx, name, o)
	return Document{
		Kind:    Kind(format),
		Text:    fmt.Sprintf("File %s is %s image, %d×%d.", name, kind, cfg.Width, cfg.Height) + more,
		Notices: []string{notice},
	}, nil
}
