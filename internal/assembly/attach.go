package assembly

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/workspace"
)

// ErrAttachment is wrapped by the refusal of a path the owner dropped that
// cannot be attached: not absolute, missing, a folder, not a plain file, or
// inside a folder hearth keeps for itself.
var ErrAttachment = errors.New("that cannot be attached")

// maxCopies is how many files of one name the Inbox may hold before a drop
// of another is refused.
const maxCopies = 1000

// InboxGrant is the read every bot is given on the Hearth folder's Inbox,
// where the files the owner drops land: it is not stored and allows only
// files.read, so it neither writes nor deletes there. It is false when the
// org has no Hearth folder.
// See docs/model.md#access.
func InboxGrant(o org.Org, botID string) (org.Grant, bool) {
	if !filepath.IsAbs(o.WorkspacePath) {
		return org.Grant{}, false
	}
	return org.Grant{BotID: botID, Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: workspace.Inbox(o.WorkspacePath)}, true
}

// Attach is the files the owner dropped, at paths, as the message carries
// them. Each path is resolved with every link followed before anything is
// checked; a path that is not absolute, does not exist, is a folder or is
// not a plain file, or lies inside reserved, is refused with ErrAttachment.
// A file inside one of readable, the folders the bot may read, or inside
// the Inbox, is attached where it is; any other is copied into the Inbox of
// the Hearth folder root, under its own name or the first free of "name 2",
// "name 3" and on, never through a link at the destination.
// See docs/model.md#conversations.
func Attach(root string, readable, reserved, paths []string) ([]org.FileRef, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if !filepath.IsAbs(root) {
		return nil, errors.New("the Hearth folder is not set, so nothing can be attached")
	}
	if err := workspace.Ensure(root); err != nil {
		return nil, err
	}
	// The Inbox as it is on disk, which must be inside the Hearth folder: a
	// link that takes it elsewhere is not followed with a copy.
	inbox, err := workspace.Resolve(root, workspace.Inbox(root))
	if err != nil {
		return nil, fmt.Errorf("the Hearth folder's Inbox: %w", err)
	}
	refs := make([]org.FileRef, 0, len(paths))
	for _, p := range paths {
		ref, err := attach(inbox, append(slices.Clone(readable), inbox), reserved, p)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// attach checks the one path p and attaches it, copying it into inbox
// unless it is inside one of readable.
func attach(inbox string, readable, reserved []string, p string) (org.FileRef, error) {
	refuse := func(why string) (org.FileRef, error) {
		return org.FileRef{}, fmt.Errorf("%s %s: %w", p, why, ErrAttachment)
	}
	if !filepath.IsAbs(p) {
		return refuse("is not an absolute path")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return refuse("does not exist")
	}
	info, err := os.Stat(real)
	switch {
	case err != nil:
		return refuse("does not exist")
	case info.IsDir():
		return refuse("is a folder; attach the files in it")
	case !info.Mode().IsRegular():
		return refuse("is not a file")
	}
	for _, r := range reserved {
		if inside(r, real) {
			return refuse("is in a folder hearth keeps for itself")
		}
	}
	name := filepath.Base(filepath.Clean(p))
	for _, f := range readable {
		if inside(f, real) {
			return org.FileRef{Path: real, Name: name}, nil
		}
	}
	dest, err := copyInto(inbox, real, name)
	if err != nil {
		return org.FileRef{}, fmt.Errorf("copy %s into the Inbox: %w", p, err)
	}
	return org.FileRef{Path: dest, Name: filepath.Base(dest)}, nil
}

// inside reports whether the resolved path real is folder, as it resolves
// on disk, or under it. A folder that does not exist holds nothing.
func inside(folder, real string) bool {
	if !filepath.IsAbs(folder) {
		return false
	}
	_, err := workspace.Resolve(folder, real)
	return err == nil
}

// copyInto copies the plain file src into the folder dir as name, or as the
// first free "stem 2.ext", "stem 3.ext" and on, and returns where it went.
// The source is opened without following a link, and the destination is
// created only if nothing, a link included, is there already.
func copyInto(dir, src, name string) (string, error) {
	in, err := os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer in.Close()
	if info, err := in.Stat(); err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a file: %w", name, ErrAttachment)
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}
	for i := 1; i <= maxCopies; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s %d%s", stem, i, ext)
		}
		dest := filepath.Join(dir, candidate)
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = io.Copy(out, in)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(dest)
			return "", err
		}
		return dest, nil
	}
	return "", fmt.Errorf("the Inbox already holds %d files named like %s", maxCopies, name)
}
