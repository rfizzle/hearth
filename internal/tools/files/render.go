package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/render/docx"
	"github.com/rfizzle/hearth/internal/render/paper"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/workspace"
)

// renderDefinition is files.render. It is checked against the write grant:
// its one effect is the file it writes, and a bot that may write in a
// folder may read the Markdown it wrote there, so the source is read under
// that grant too.
// See docs/decisions.md for D37.
func (o Options) renderDefinition() tools.Definition {
	return tools.Definition{
		Name: "files.render",
		Description: "Turn a Markdown file in your folder into a PDF or a Word file beside it: report.md becomes report.pdf or report.docx, " +
			"replacing an older one. A PDF is set in Hearth's print style; a Word file uses Word's own styles, so the person can edit it. " +
			"Paths are relative to your folder. Save the Markdown first; headings, lists, tables, code, quotes and links carry over, " +
			"and so do PNG or JPEG images saved in the same folder and named by their path. " +
			"Then name both files in your reply.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"The Markdown file, relative to your folder, for example report.md."},` +
			`"to":{"type":"string","enum":["pdf","docx"],"description":"The kind of file to make: pdf, or docx for a Word file."},` +
			`"paper":{"type":"string","enum":["letter","a4"],"description":"The paper size. Leave it out for US Letter."}` +
			`},"required":["path","to"]}`),
		Capability: org.FilesWrite,
		Tier:       tools.TierRead, // as files.write: the effect stays in a granted folder
		Execute:    o.render,
		Describe:   describeRender,
		Validate:   validateRender,
	}
}

type renderArgs struct {
	Path  string `json:"path"`
	To    string `json:"to"`
	Paper string `json:"paper"`
}

// validateRender refuses a source that is not Markdown, a kind that is not
// a PDF or Word file and an unknown paper, before the call is placed or put to the owner.
func validateRender(args json.RawMessage) error {
	var a renderArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return errors.New("the arguments are not a JSON object with a path and to")
	}
	if !markdown(a.Path) {
		return fmt.Errorf("%s is not a Markdown file; save the text as a .md file first, then render that", a.Path)
	}
	switch strings.ToLower(a.To) {
	case "pdf", "docx":
	default:
		return fmt.Errorf("to must be pdf or docx, not %q", a.To)
	}
	_, err := paper.ParsePaper(a.Paper)
	return err
}

// markdown reports whether name ends in .md or .markdown.
func markdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// render draws the Markdown at call.Path as a PDF or a Word file with the
// same stem beside it. The output's name is derived from the source's resolved path, never
// from anything else the model sent, and it is written through
// openForWrite like any file files.write makes.
func (o Options) render(ctx context.Context, call tools.Call) (tools.Result, error) {
	var a renderArgs
	_ = json.Unmarshal(call.Args, &a) // Validate has accepted these arguments
	name := shown(call)
	if !markdown(call.Path) {
		return tools.Result{}, fmt.Errorf("files.render: %s leads to a file that is not Markdown; nothing was written", name)
	}
	src, modified, err := readSource(call.Path, name)
	if err != nil {
		return tools.Result{}, err
	}
	sheet, _ := paper.ParsePaper(a.Paper)
	out, err := o.draw(ctx, strings.ToLower(a.To), src, modified, sheet, name, o.images(call))
	if err != nil {
		return tools.Result{}, err
	}
	target := strings.TrimSuffix(call.Path, filepath.Ext(call.Path)) + out.ext
	targetName := strings.TrimSuffix(name, filepath.Ext(name)) + out.ext
	if err := writeFile("files.render", call.Folder, target, targetName, out.data); err != nil {
		return tools.Result{}, err
	}
	text := fmt.Sprintf("Rendered %s to %s (%s).", name, targetName, out.summary)
	for _, n := range out.notices {
		text += "\n" + n
	}
	return tools.Result{
		Text: text,
		Files: []org.FileRef{
			{Path: call.Path, Name: filepath.Base(call.Path)},
			{Path: target, Name: filepath.Base(target)},
		},
	}, nil
}

// drawn is a rendered file: its extension, its bytes, what the result says of
// it in brackets, and the notices for what was left out.
type drawn struct {
	ext     string
	data    []byte
	summary string
	notices []string
}

// draw renders src as a PDF or a Word file. A deadline is the same refusal
// for both.
func (o Options) draw(ctx context.Context, to string, src []byte, modified time.Time, sheet paper.Paper, name string, open paper.Opener) (drawn, error) {
	var out drawn
	var err error
	switch to {
	case "docx":
		var rep docx.Report
		out.ext = ".docx"
		out.data, rep, err = docx.Write(ctx, src, docx.Options{A4: sheet == paper.A4, Name: name, Image: open})
		out.summary, out.notices = size(rep.Bytes), rep.Notices
	default:
		var rep paper.Report
		out.ext = ".pdf"
		out.data, rep, err = paper.Markdown(ctx, src, paper.Options{Paper: sheet, Name: name, Modified: modified, Image: open})
		out.summary, out.notices = plural(rep.Pages, "page")+", "+size(rep.Bytes), rep.Notices
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return drawn{}, fmt.Errorf("files.render: rendering %s took longer than a minute and stopped; nothing was written", name)
	case err != nil:
		return drawn{}, fmt.Errorf("files.render: %s: %w; nothing was written", name, err)
	}
	return out, nil
}

// readSource reads the Markdown at path, a regular file of at most
// paper.MaxSource bytes, and its modification time. A link at the file
// itself and anything but a regular file are refused, and the open does not
// wait on a named pipe.
func readSource(path, name string) ([]byte, time.Time, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, time.Time{}, failed("files.render", "read", name, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, failed("files.render", "read", name, err)
	}
	if err := regular(info, name); err != nil {
		return nil, time.Time{}, fmt.Errorf("files.render: %w", err)
	}
	if info.Size() > paper.MaxSource {
		return nil, time.Time{}, fmt.Errorf("files.render: %s is over 1 MB; split it into smaller files and render each", name)
	}
	src, err := io.ReadAll(io.LimitReader(f, paper.MaxSource+1))
	if err != nil {
		return nil, time.Time{}, failed("files.render", "read", name, err)
	}
	if len(src) > paper.MaxSource {
		return nil, time.Time{}, fmt.Errorf("files.render: %s is over 1 MB; split it into smaller files and render each", name)
	}
	return src, info.ModTime(), nil
}

// images opens an image a rendered document names: a path relative to the
// document's folder, resolved with its links followed, that must still lie
// inside the granted folder and outside every reserved one, as the
// dispatcher places a call's own path. The file itself must be a regular
// file reached without a link.
func (o Options) images(call tools.Call) paper.Opener {
	dir := filepath.Dir(call.Path)
	return func(rel string) (io.ReadCloser, error) {
		p, err := workspace.Resolve(call.Folder, filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		for _, r := range o.Reserved {
			if workspace.Inside(r, p) {
				return nil, workspace.ErrOutside
			}
		}
		f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
			f.Close()
			return nil, errors.New("not a regular file")
		}
		return f, nil
	}
}

// describeRender names the file and what it becomes. Through the
// dispatcher the path starts with the granted folder's name and the folder
// argument repeats it, so the card reads "Render report.md as PDF in Dana's
// folder".
func describeRender(args json.RawMessage) string {
	var a struct {
		renderArgs
		Folder string `json:"folder"`
	}
	_ = json.Unmarshal(args, &a) // as describeRead
	kind := "PDF"
	if strings.EqualFold(a.To, "docx") {
		kind = "Word"
	}
	file, ok := strings.CutPrefix(filepath.ToSlash(a.Path), a.Folder+"/")
	if a.Folder == "" || !ok || file == "" {
		return fmt.Sprintf("Render %s as %s", a.Path, kind)
	}
	return fmt.Sprintf("Render %s as %s in %s's folder", file, kind, a.Folder)
}
