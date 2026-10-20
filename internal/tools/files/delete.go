package files

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

// deleteDefinition is files.delete. It is act tier with no standing: every
// call waits for the owner, who may allow it once or for the conversation
// but never for good.
// See docs/model.md#access.
func deleteDefinition() tools.Definition {
	return tools.Definition{
		Name: string(org.FilesDelete),
		Description: "Delete one file in your folder. Paths are relative to your folder. " +
			"Delete only a file the owner asked you to delete, never to tidy up on your own. " +
			"Every call waits for the owner to allow it, and they see which file it is. " +
			"It removes one file, never a folder, and refuses a link. " +
			"Afterwards, name the file in your reply so the owner knows what went.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"The file, relative to your folder, for example old/brief.md."}` +
			`},"required":["path"]}`),
		Capability: org.FilesDelete,
		Tier:       tools.TierAct,
		NoStanding: true,
		Execute:    remove,
		Describe:   describeDelete,
	}
}

// remove deletes the one regular file the call resolved to. The file is
// opened through the folder as a root, so a folder on the way swapped for a
// link since the dispatcher's check cannot carry the delete outside it, and
// it must still be the file the call named.
func remove(_ context.Context, call tools.Call) (tools.Result, error) {
	name := shown(call)
	rel, named, err := target(call)
	if err != nil {
		return tools.Result{}, err
	}
	root, err := os.OpenRoot(call.Folder)
	if err != nil {
		return tools.Result{}, failed("files.delete", "open the folder of", name, err)
	}
	defer root.Close()
	info, err := root.Lstat(rel)
	if err != nil {
		return tools.Result{}, failed("files.delete", "delete", name, err)
	}
	if !os.SameFile(info, named) {
		return tools.Result{}, fmt.Errorf("files.delete: %s changed since it was checked; nothing was deleted", name)
	}
	if err := regular(info, name); err != nil {
		return tools.Result{}, fmt.Errorf("files.delete: %w; nothing was deleted", err)
	}
	if err := root.Remove(rel); err != nil {
		return tools.Result{}, failed("files.delete", "delete", name, err)
	}
	return tools.Result{Text: "Deleted " + name + "."}, nil
}

// target is the file a delete removes, as a path inside its folder, and
// what the call's own path names on disk. A delete never follows a link: a
// call whose path ends in one is refused, since the file it points at is
// not the one the call named, and so is one whose path no longer leads to
// the file the dispatcher resolved, or that names the folder itself.
// See docs/model.md#access.
func target(call tools.Call) (string, fs.FileInfo, error) {
	name := shown(call)
	named, err := os.Lstat(call.Requested)
	if err != nil {
		return "", nil, failed("files.delete", "delete", name, err)
	}
	if named.Mode()&fs.ModeSymlink != 0 {
		return "", nil, fmt.Errorf("files.delete: %s is a link; delete only removes a file named by its own path", filepath.Base(call.Requested))
	}
	resolved, err := os.Lstat(call.Path)
	if err != nil || !os.SameFile(named, resolved) {
		return "", nil, fmt.Errorf("files.delete: %s no longer leads to the file that was checked; nothing was deleted", name)
	}
	rel, err := filepath.Rel(call.Folder, call.Path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("files.delete: %s is not a file inside the folder", name)
	}
	return rel, named, nil
}

// describeDelete names the file and the folder it is in. Through the
// dispatcher the path starts with the granted folder's name and the folder
// argument repeats it, so the card reads "Delete brief.md from Dana's
// folder"; a path without it reads as the bare path.
func describeDelete(args json.RawMessage) string {
	var a struct {
		Path   string `json:"path"`
		Folder string `json:"folder"`
	}
	_ = json.Unmarshal(args, &a) // as describeRead
	file, ok := strings.CutPrefix(filepath.ToSlash(a.Path), a.Folder+"/")
	if a.Folder == "" || !ok || file == "" {
		return "Delete " + a.Path
	}
	return fmt.Sprintf("Delete %s from %s's folder", file, a.Folder)
}
