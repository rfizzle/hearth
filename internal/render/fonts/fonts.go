package fonts

import "embed"

// The families a face is registered under.
const (
	Sans = "sans"
	Mono = "mono"
)

//go:embed SourceSans3-Regular.ttf SourceSans3-Bold.ttf SourceSans3-It.ttf SourceCodePro-Regular.ttf OFL.txt
var files embed.FS

// A Face is one embedded font: the family and the style ("", "B" or "I") it
// is registered as, the file it came from and its bytes.
type Face struct {
	Family string
	Style  string
	File   string
	Data   []byte
}

// Faces is every embedded face with its bytes, in the order a renderer
// registers them.
func Faces() []Face {
	out := []Face{
		{Family: Sans, Style: "", File: "SourceSans3-Regular.ttf"},
		{Family: Sans, Style: "B", File: "SourceSans3-Bold.ttf"},
		{Family: Sans, Style: "I", File: "SourceSans3-It.ttf"},
		{Family: Mono, Style: "", File: "SourceCodePro-Regular.ttf"},
	}
	for i := range out {
		// The names are fixed above and embedded at build time, so the read
		// cannot fail; a face that somehow did would be empty, and the
		// renderer refuses an empty face.
		out[i].Data, _ = files.ReadFile(out[i].File)
	}
	return out
}

// License is the text of the SIL Open Font License the faces are under, with
// each family's copyright notice.
func License() string {
	b, _ := files.ReadFile("OFL.txt") // as in Faces
	return string(b)
}
