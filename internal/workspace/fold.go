package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

// foldCache remembers, per root, whether its file system folds case.
var foldCache sync.Map

// Inside reports whether path is root or lies under it, for a reserved or
// granted folder check. root is an absolute path resolved with its links
// followed; a root that does not exist yet is resolved through its nearest
// existing parent, so a reserved folder counts before it is made and a
// write cannot be the one that makes it. path is taken as given, already
// resolved. Letter case is ignored when root's file system ignores it, so
// Templates/x and templates/x are the same place on a Mac.
func Inside(root, path string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false
	}
	real, err := resolveExisting(filepath.Clean(root))
	if err != nil {
		return false
	}
	return Within(real, path)
}

// Within is Inside on the text alone: root and path are absolute, cleaned
// here, and no link is followed. The case rule is Inside's; a root that does
// not exist takes the platform's default.
func Within(root, path string) bool {
	return containsText(root, path, foldsCase(root))
}

func containsText(root, path string, fold bool) bool {
	root, path = filepath.Clean(root), filepath.Clean(path)
	if fold {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}

// foldsCase reports whether the file system holding root treats names that
// differ only in letter case as one. On darwin it always does; elsewhere root
// is probed once by statting itself with its last cased letter flipped.
func foldsCase(root string) bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	if v, ok := foldCache.Load(root); ok {
		return v.(bool)
	}
	info, err := os.Stat(root)
	if err != nil {
		return false
	}
	folds := false
	if flipped := flipCase(root); flipped != root {
		if other, err := os.Stat(flipped); err == nil && os.SameFile(info, other) {
			folds = true
		}
	}
	foldCache.Store(root, folds)
	return folds
}

// flipCase swaps the case of the last letter in p that has another case.
func flipCase(p string) string {
	r := []rune(p)
	for i := len(r) - 1; i >= 0; i-- {
		switch {
		case unicode.IsUpper(r[i]):
			r[i] = unicode.ToLower(r[i])
		case unicode.IsLower(r[i]):
			r[i] = unicode.ToUpper(r[i])
		default:
			continue
		}
		return string(r)
	}
	return p
}
