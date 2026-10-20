package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

func pngFile(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRender(t *testing.T) {
	b := newBot(t, org.FilesWrite)
	b.put(t, "drafts/report.md", []byte("# Report\n\nText.\n\n![Chart](chart.png)\n\n<p>html</p>\n"))
	b.put(t, "drafts/chart.png", pngFile(t))
	when := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(b.folder, "drafts/report.md"), when, when); err != nil {
		t.Fatal(err)
	}
	res, err := b.call("files.render", map[string]string{"path": "drafts/report.md", "to": "pdf"})
	if err != nil {
		t.Fatalf("files.render: %v", err)
	}
	src, out := filepath.Join(b.folder, "drafts/report.md"), filepath.Join(b.folder, "drafts/report.pdf")
	want := []org.FileRef{{Path: src, Name: "report.md"}, {Path: out, Name: "report.pdf"}}
	if !slices.Equal(res.Files, want) {
		t.Errorf("Files = %+v, want %+v", res.Files, want)
	}
	first, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(first, []byte("%PDF-")) {
		t.Fatalf("report.pdf: %v, %q", err, first[:min(len(first), 8)])
	}
	for _, w := range []string{"Rendered drafts/report.md to drafts/report.pdf (1 page, ", "[drafts/report.md had HTML in it; it was left out of the PDF.]"} {
		if !strings.Contains(res.Text, w) {
			t.Errorf("result %q is missing %q", res.Text, w)
		}
	}
	if strings.Contains(res.Text, "could not be used") || strings.Contains(res.Text, b.folder) {
		t.Errorf("result %q: the image was refused, or an absolute path reached the model", res.Text)
	}
	// Again, over the first: the same bytes, the file replaced.
	if _, err := b.call("files.render", map[string]string{"path": "drafts/report.md", "to": "pdf", "paper": "letter"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(out); !bytes.Equal(first, again) {
		t.Error("rendering the same file twice gave different bytes")
	}
}

func TestRender_Refusals(t *testing.T) {
	b := newBot(t, org.FilesWrite)
	b.put(t, "notes.txt", []byte("plain"))
	b.put(t, "big.md", bytes.Repeat([]byte("a"), 1<<20+1))
	b.put(t, "real.txt", []byte("secret"))
	if err := os.Symlink(filepath.Join(b.folder, "real.txt"), filepath.Join(b.folder, "alias.md")); err != nil {
		t.Fatal(err)
	}
	b.put(t, "linked.md", []byte("# Linked"))
	if err := os.Symlink(filepath.Join(b.outside, "linked.pdf"), filepath.Join(b.folder, "linked.pdf")); err != nil {
		t.Fatal(err)
	}
	b.fifo(t, "pipe.md")
	cases := []struct {
		args map[string]string
		want string
	}{
		{map[string]string{"path": "notes.txt", "to": "pdf"}, "is not a Markdown file"},
		{map[string]string{"path": "a.md", "to": "docx"}, "not yet available"},
		{map[string]string{"path": "a.md", "to": "html"}, "to must be pdf"},
		{map[string]string{"path": "a.md", "to": "pdf", "paper": "legal"}, "paper must be letter or a4"},
		{map[string]string{"path": "big.md", "to": "pdf"}, "is over 1 MB"},
		{map[string]string{"path": "alias.md", "to": "pdf"}, "not Markdown"},
		{map[string]string{"path": "linked.md", "to": "pdf"}, "no longer leads inside the folder"},
		{map[string]string{"path": "pipe.md", "to": "pdf"}, "not a regular file"},
		{map[string]string{"path": "../elsewhere/x.md", "to": "pdf"}, "outside the folders"},
	}
	for _, tc := range cases {
		if _, err := b.call("files.render", tc.args); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: %v, want an error with %q", tc.args, err, tc.want)
		}
	}
	if _, err := os.Lstat(filepath.Join(b.outside, "linked.pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a render wrote through a link out of the folder")
	}
	for _, name := range []string{"notes.pdf", "big.pdf", "real.pdf", "alias.pdf", "pipe.pdf"} {
		if _, err := os.Lstat(filepath.Join(b.folder, name)); err == nil {
			t.Errorf("%s was written by a refused render", name)
		}
	}
}

func TestRender_Images(t *testing.T) {
	b := newBot(t, org.FilesWrite)
	reserved := filepath.Join(b.folder, "kept")
	reg, err := tools.NewRegistry(Definitions(Options{Reserved: []string{reserved}})...)
	if err != nil {
		t.Fatal(err)
	}
	b.d.Defs = reg
	img := pngFile(t)
	b.put(t, "kept/inside.png", img)
	if err := os.WriteFile(filepath.Join(b.outside, "out.png"), img, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(b.outside, "out.png"), filepath.Join(b.folder, "link.png")); err != nil {
		t.Fatal(err)
	}
	b.put(t, "r.md", []byte("![a](../elsewhere/out.png) ![b](link.png) ![c](kept/inside.png) ![d](https://example.com/x.png)"))
	res, err := b.call("files.render", map[string]string{"path": "r.md", "to": "pdf"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"[r.md has images that could not be used", "[r.md has images from the web"} {
		if !strings.Contains(res.Text, w) {
			t.Errorf("result %q is missing %q", res.Text, w)
		}
	}
	opened := Options{Reserved: []string{reserved}}.images(tools.Call{Folder: b.folder, Path: filepath.Join(b.folder, "r.md")})
	for _, rel := range []string{"../elsewhere/out.png", "link.png", "kept/inside.png"} {
		if f, err := opened(rel); err == nil {
			f.Close()
			t.Errorf("%s was opened", rel)
		}
	}
}

func TestRender_Describe(t *testing.T) {
	cases := []struct{ args, want string }{
		{`{"path":"report.md","to":"pdf"}`, "Render report.md as PDF"},
		{`{"path":"Dana/report.md","to":"pdf","folder":"Dana"}`, "Render report.md as PDF in Dana's folder"},
		{`nope`, "Render  as PDF"},
	}
	for _, tc := range cases {
		if got := describeRender(json.RawMessage(tc.args)); got != tc.want {
			t.Errorf("describeRender(%s) = %q, want %q", tc.args, got, tc.want)
		}
	}

	// Under an ask-mode write grant the call waits, and the card names the
	// granted folder, not the bot's paraphrase of the path.
	reg, err := tools.NewRegistry(Definitions(Options{})...)
	if err != nil {
		t.Fatal(err)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(base, "Dana")
	if err := os.MkdirAll(filepath.Join(folder, "drafts"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := tools.Dispatcher{Defs: reg, Root: folder, Grants: []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: folder}}}
	c, err := d.Check("files.render", json.RawMessage(`{"path":"drafts/../drafts/report.md","to":"pdf"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Verdict != tools.Gated || c.Describe() != "Render drafts/report.md as PDF in Dana's folder" {
		t.Errorf("verdict %v, card %q", c.Verdict, c.Describe())
	}
	if c.Def.Capability != org.FilesWrite || !c.Def.Standing() {
		t.Errorf("files.render is checked as %s, standing %v", c.Def.Capability, c.Def.Standing())
	}
}
