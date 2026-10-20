package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rfizzle/hearth/internal/org"
)

// inbox is the folder files the owner drops in without a home land in.
const inbox = org.InboxFolder

// ErrOutside is returned when a requested path resolves outside the root.
var ErrOutside = errors.New("the path is outside the folder")

// Ensure creates root and its Inbox folder if they are missing.
func Ensure(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("workspace %q: not an absolute path", root)
	}
	if err := os.MkdirAll(Inbox(root), 0o755); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}
	return nil
}

// Inbox is the folder under root that files the owner drops in without a
// home land in.
func Inbox(root string) string { return filepath.Join(root, inbox) }

// BotFolder is the folder a bot named name keeps its work in under root.
func BotFolder(root, name string) string {
	return filepath.Join(root, org.FolderName(name))
}

// Resolve turns requested, relative to root or absolute, into the absolute
// path it names on disk with every existing symlink followed, and refuses it
// with ErrOutside when that path is not root or under it. The path need not
// exist yet: the nearest ancestor that does is resolved and the rest joined
// back on. root must exist.
func Resolve(root, requested string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("workspace %q: not an absolute path", root)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("workspace: %w", err)
	}
	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	real, err := resolveExisting(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if !within(realRoot, real) {
		return "", fmt.Errorf("%s: %w", requested, ErrOutside)
	}
	return real, nil
}

// resolveExisting follows the symlinks of the longest prefix of path that
// exists and joins the rest back on. A dangling symlink is an error rather
// than a missing element, because creating the file through it would land
// wherever it points.
func resolveExisting(path string) (string, error) {
	missing := ""
	for {
		if _, err := os.Lstat(path); err == nil {
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", fmt.Errorf("%s: %w", path, err)
			}
			return filepath.Join(real, missing), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("%s: no existing ancestor", path)
		}
		missing = filepath.Join(filepath.Base(path), missing)
		path = parent
	}
}

// within reports whether path is root or a path under it. Both are clean
// and absolute.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
