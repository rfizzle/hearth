package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/workspace"
)

// readCap is the most one files.read call returns. A longer file is read on
// from the offset the notice names.
const readCap = 64 << 10

// Definitions is files.read and files.write.
func Definitions() []tools.Definition {
	return []tools.Definition{
		{
			Name: string(org.FilesRead),
			Description: "Read a text file in your folder. Paths are relative to your folder. " +
				"Use it to look at a file before you change it, or to pick up work saved earlier. " +
				"It returns up to 64 KB at a time; when the file is longer, the result ends with the offset to pass to read on. " +
				"It reads text only, not images, PDFs or other binary files.",
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"path":{"type":"string","description":"The file, relative to your folder, for example brief.md or notes/sources.md."},` +
				`"offset":{"type":"integer","description":"The byte to start at. Leave it out to read from the start; pass the offset a previous result named to read on."}` +
				`},"required":["path"]}`),
			Capability: org.FilesRead,
			Tier:       tools.TierRead,
			Execute:    read,
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
	}
}

type readArgs struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
}

type writeArgs struct {
	Path    string  `json:"path"`
	Content *string `json:"content"`
}

func read(_ context.Context, call tools.Call) (tools.Result, error) {
	var a readArgs
	if err := json.Unmarshal(call.Args, &a); err != nil || a.Offset < 0 {
		return tools.Result{}, errors.New("files.read: offset must be a whole number of bytes, 0 or more")
	}
	f, err := os.Open(call.Path)
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	if info.IsDir() {
		return tools.Result{}, fmt.Errorf("files.read: %s is a folder, not a file", a.Path)
	}
	size := info.Size()
	if a.Offset > size {
		return tools.Result{}, fmt.Errorf("files.read: offset %d is past the end of %s (%d bytes)", a.Offset, a.Path, size)
	}
	buf := make([]byte, readCap)
	n, err := f.ReadAt(buf, a.Offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	start, page := wholeRunes(buf[:n], a.Offset+int64(n) < size)
	if !isText(page) {
		return tools.Result{}, fmt.Errorf("files.read: %s is not a text file; files.read reads text only", a.Path)
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
	if err := os.MkdirAll(filepath.Dir(call.Path), 0o755); err != nil {
		return tools.Result{}, fmt.Errorf("files.write: %w", err)
	}
	if err := stillInside(call); err != nil {
		return tools.Result{}, err
	}
	// O_NOFOLLOW refuses a symlink at the file itself, the one element
	// stillInside cannot hold steady between its check and this open.
	f, err := os.OpenFile(call.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.write: %s cannot be written: %w", a.Path, err)
	}
	_, err = io.WriteString(f, *a.Content)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.write: %w", err)
	}
	return tools.Result{
		Text:  fmt.Sprintf("Wrote %s to %s.", size(len(*a.Content)), call.Path),
		Files: []org.FileRef{{Path: call.Path, Name: filepath.Base(call.Path)}},
	}, nil
}

// stillInside resolves the path again now that its folders exist, and
// refuses it unless it is still the path the dispatcher checked: a folder
// swapped for a symlink in between would otherwise take the write with it.
func stillInside(call tools.Call) error {
	again, err := workspace.Resolve(call.Folder, call.Path)
	if err != nil || again != call.Path {
		return fmt.Errorf("files.write: %s no longer leads inside the folder; nothing was written", call.Path)
	}
	return nil
}

func describeRead(args json.RawMessage) string {
	var a readArgs
	_ = json.Unmarshal(args, &a) // a describer never fails; an unreadable call shows a bare verb
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
