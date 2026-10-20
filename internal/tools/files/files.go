package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/document"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/workspace"
)

// readCap is the most one files.read call returns. A longer file is read on
// from the offset the notice names.
const readCap = 64 << 10

// Definitions is files.read, files.write, files.render and files.delete.
func Definitions(o Options) []tools.Definition {
	return []tools.Definition{
		{
			Name: string(org.FilesRead),
			Description: "Read a file in your folder as text: plain text and Markdown, and PDF, Word (.docx), Excel (.xlsx), CSV, PowerPoint (.pptx) and RTF; " +
				"an image gives its kind and size, and its text when Hearth is open. Paths are relative to your folder. " +
				"Use it to look at a file before you change it, or to pick up work saved earlier. " +
				"It returns up to 64 KB at a time; a longer file ends with the offset or the pages to pass to read on.",
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"path":{"type":"string","description":"The file, relative to your folder, for example brief.md, notes/sources.md or report.pdf."},` +
				`"offset":{"type":"integer","description":"The byte to start at. Leave it out to read from the start; pass the offset a previous result named to read on."},` +
				`"pages":{"type":"string","description":"For a PDF or a deck: the pages to read, such as \"4-12\" or \"7\". At most 100 pages are read at a time; leave it out to start at page 1."},` +
				`"sheet":{"type":"string","description":"For an Excel workbook: the one sheet to read, by its name or its number from 1. Leave it out to read the first 20 sheets."}` +
				`},"required":["path"]}`),
			Capability: org.FilesRead,
			Tier:       tools.TierRead,
			Execute:    o.read,
			Describe:   describeRead,
		},
		{
			Name: string(org.FilesWrite),
			Description: "Create a file in your folder, or replace one, with the text you give. Paths are relative to your folder; " +
				"missing folders on the way are created. Use it to save a deliverable or notes you will need later, " +
				"then tell the owner where the file is. It replaces the whole file, so read the file first when you mean to change part of it.",
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"path":{"type":"string","description":"The file, relative to your folder, for example brief.md or drafts/plan.md."},` +
				`"content":{"type":"string","description":"The whole text of the file."}` +
				`},"required":["path","content"]}`),
			Capability: org.FilesWrite,
			// Writing inside a folder the owner granted is read tier: act tier is
			// for effects the core cannot see, and those wait for the owner's
			// approval.
			Tier:     tools.TierRead,
			Execute:  write,
			Describe: describeWrite,
		},
		o.renderDefinition(),
		deleteDefinition(),
	}
}

type readArgs struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Pages  string `json:"pages"`
	Sheet  string `json:"sheet"`
}

type writeArgs struct {
	Path    string  `json:"path"`
	Content *string `json:"content"`
}

func (o Options) read(ctx context.Context, call tools.Call) (tools.Result, error) {
	var a readArgs
	if err := json.Unmarshal(call.Args, &a); err != nil || a.Offset < 0 {
		return tools.Result{}, errors.New(`files.read: offset must be a whole number of bytes, 0 or more, and pages text such as "4-12"`)
	}
	name := shown(call)
	info, err := os.Lstat(call.Path)
	if err != nil {
		return tools.Result{}, failed("files.read", "read", name, err)
	}
	if err := regular(info, name); err != nil {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	f, err := os.Open(call.Path)
	if err != nil {
		return tools.Result{}, failed("files.read", "open", name, err)
	}
	defer f.Close()
	kind, err := kindOf(f, name)
	if err != nil {
		return tools.Result{}, err
	}
	if a.Sheet != "" && kind != document.KindXLSX {
		return tools.Result{}, errors.New("files.read: sheet applies to Excel files only")
	}
	if kind != document.KindText {
		return o.readDocument(ctx, call, a, name)
	}
	if a.Pages != "" {
		return tools.Result{}, errors.New("files.read: pages applies to PDF and PowerPoint files only")
	}
	size := info.Size()
	if a.Offset > size {
		return tools.Result{}, fmt.Errorf("files.read: offset %d is past the end of %s (%d bytes)", a.Offset, name, size)
	}
	buf := make([]byte, readCap)
	n, err := f.ReadAt(buf, a.Offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return tools.Result{}, failed("files.read", "read", name, err)
	}
	start, page := wholeRunes(buf[:n], a.Offset+int64(n) < size)
	if !isText(page) {
		return tools.Result{}, fmt.Errorf("files.read: %s is not a text file; files.read reads text only", name)
	}
	from := a.Offset + int64(start)
	end := from + int64(len(page))
	text := string(page)
	switch {
	case size == 0:
		text = "(the file is empty)"
	case end < size:
		text += fmt.Sprintf("\n\n[Showing bytes %d to %d of %d. To read on, call files.read with offset %d.]", from, end, size, end)
	case from > 0:
		text += fmt.Sprintf("\n\n[Showing bytes %d to %d of %d, the end of the file.]", from, end, size)
	}
	return tools.Result{Text: text}, nil
}

// wholeRunes trims page to whole UTF-8 characters: the continuation bytes an
// offset in the middle of a character starts on, and, when the file goes on,
// a character the cap cut in two. start is how many bytes were dropped from
// the front.
func wholeRunes(page []byte, more bool) (start int, trimmed []byte) {
	for start < len(page) && start < utf8.UTFMax-1 && !utf8.RuneStart(page[start]) {
		start++
	}
	page = page[start:]
	if more {
		for cut := 1; cut < utf8.UTFMax && cut <= len(page); cut++ {
			if utf8.RuneStart(page[len(page)-cut]) {
				if !utf8.FullRune(page[len(page)-cut:]) {
					page = page[:len(page)-cut]
				}
				break
			}
		}
	}
	return start, page
}

// isText reports whether b reads as text: valid UTF-8 with no NUL byte.
func isText(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return utf8.Valid(b)
}

func write(_ context.Context, call tools.Call) (tools.Result, error) {
	var a writeArgs
	if err := json.Unmarshal(call.Args, &a); err != nil || a.Content == nil {
		return tools.Result{}, errors.New("files.write: content is required")
	}
	name := shown(call)
	if err := writeFile("files.write", call.Folder, call.Path, name, []byte(*a.Content)); err != nil {
		return tools.Result{}, err
	}
	return tools.Result{
		Text:  fmt.Sprintf("Wrote %s to %s.", size(len(*a.Content)), name),
		Files: []org.FileRef{{Path: call.Path, Name: filepath.Base(call.Path)}},
	}, nil
}

// writeFile creates or replaces the file at path, inside folder, with data,
// through OpenForWrite. name is the path as the model reads it.
func writeFile(tool, folder, path, name string, data []byte) error {
	f, err := OpenForWrite(tool, folder, path, name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return failed(tool, "write", name, err)
	}
	return nil
}

// OpenForWrite opens path, inside folder, to be written from the start: it
// creates the folders above it, checks again that the path still leads
// inside the folder, replaces only a regular file, and refuses a link at the
// file itself and a file with a second name. Every file a tool writes in a
// granted folder is opened through it. path is resolved inside folder, as
// the dispatcher placed it; tool names the caller and name is the path as
// the model reads it, for the refusals. The caller closes the file.
func OpenForWrite(tool, folder, path, name string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, failed(tool, "create the folders above", name, err)
	}
	if err := stillInside(tool, folder, path, name); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil {
		if err := regular(info, name); err != nil {
			return nil, fmt.Errorf("%s: %w; nothing was written", tool, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, failed(tool, "write", name, err)
	}
	// O_NOFOLLOW refuses a symlink at the file itself, the one element
	// stillInside cannot hold steady between its check and this open.
	// It truncates only after the open file is checked: a file with a second
	// name, a hard link perhaps outside the folder, would change under that
	// name too.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return nil, failed(tool, "write", name, err)
	}
	info, err := f.Stat()
	if err == nil {
		err = regular(info, name)
	}
	if err == nil && links(info) > 1 {
		err = fmt.Errorf("%s has another name elsewhere (a hard link), and writing it would change that file too", name)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w; nothing was written", tool, err)
	}
	if err := f.Truncate(0); err != nil {
		f.Close()
		return nil, failed(tool, "write", name, err)
	}
	return f, nil
}

// links is how many names the file has, or 1 when the system does not say.
func links(info fs.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink) //nolint:unconvert // Nlink is narrower on darwin
	}
	return 1
}

// stillInside resolves the path again now that its folders exist, and
// refuses it unless it is still the path the dispatcher checked: a folder
// swapped for a symlink in between would otherwise take the write with it.
func stillInside(tool, folder, path, name string) error {
	again, err := workspace.Resolve(folder, path)
	if err != nil || again != path {
		return fmt.Errorf("%s: %s no longer leads inside the folder; nothing was written", tool, name)
	}
	return nil
}

// regular refuses anything but a regular file. A named pipe or a device
// would block the open until something else answered, and the call would
// never return.
func regular(info fs.FileInfo, name string) error {
	switch {
	case info.IsDir():
		return fmt.Errorf("%s is a folder, not a file", name)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s is not a regular file", name)
	}
	return nil
}

// shown is the path as the model reads it: relative to the granted folder.
// The absolute path names the owner's account, so it stays out of anything
// the model reads and travels only in the result's files, for the owner.
func shown(call tools.Call) string {
	if rel, err := filepath.Rel(call.Folder, call.Path); err == nil && call.Folder != "" {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(call.Path)
}

// failed is a file system error in words the model reads: the operation and
// the relative path, without the absolute path an *os.PathError carries.
func failed(tool, op, name string, err error) error {
	reason := "the system refused"
	switch {
	case errors.Is(err, fs.ErrNotExist):
		reason = "no such file"
	case errors.Is(err, fs.ErrPermission):
		reason = "permission denied"
	case errors.Is(err, syscall.EISDIR):
		reason = "it is a folder"
	case errors.Is(err, syscall.ENOTDIR):
		reason = "a part of the path is a file, not a folder"
	}
	return fmt.Errorf("%s: could not %s %s: %s", tool, op, name, reason)
}

func describeRead(args json.RawMessage) string {
	var a readArgs
	_ = json.Unmarshal(args, &a) // a describer never fails; an unreadable call shows a bare verb
	if a.Pages != "" {
		return fmt.Sprintf("Read %s, pages %s", a.Path, a.Pages)
	}
	if a.Sheet != "" {
		return fmt.Sprintf("Read %s, sheet %s", a.Path, a.Sheet)
	}
	if a.Offset > 0 {
		return fmt.Sprintf("Read %s from byte %d", a.Path, a.Offset)
	}
	return "Read " + a.Path
}

func describeWrite(args json.RawMessage) string {
	var a writeArgs
	_ = json.Unmarshal(args, &a) // as describeRead
	if a.Content == nil {
		return "Write " + a.Path
	}
	return fmt.Sprintf("Write %s (%s)", a.Path, size(len(*a.Content)))
}

// size is n bytes as the owner reads a file size.
func size(n int) string {
	switch {
	case n == 1:
		return "1 byte"
	case n < 1<<10:
		return fmt.Sprintf("%d bytes", n)
	case n < 1<<20:
		return fmt.Sprintf("%d KB", (n+1<<9)>>10)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}
