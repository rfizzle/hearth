package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/workspace"
)

// ErrRefused is wrapped by every error the dispatcher returns instead of
// running a call.
var ErrRefused = errors.New("refused")

// Dispatcher runs one bot's calls. Defs is every tool configured for the
// run, Grants the bot's grants and Root the folder a relative path in a
// files call starts from.
type Dispatcher struct {
	Defs   *Registry
	Grants []org.Grant
	Root   string
}

// Tools is the definitions the grants allow, in registry order: the toolset
// the model is offered. A tool it is not offered is refused if called.
func (d Dispatcher) Tools() []Definition {
	var allowed []Definition
	for _, def := range d.Defs.All() {
		if d.permitted(def) {
			allowed = append(allowed, def)
		}
	}
	return allowed
}

// permitted reports whether the grants let def run without asking anyone.
func (d Dispatcher) permitted(def Definition) bool {
	if def.Tier != TierRead {
		return false
	}
	if folderScoped(def.Capability) {
		return len(org.FolderGrants(d.Grants, def.Capability)) > 0
	}
	return org.Allowed(d.Grants, def.Capability) == org.ModeAllowed
}

// folderScoped reports whether capability is granted per folder, so a call
// to it names a path that has to be inside one of them.
func folderScoped(capability org.Capability) bool {
	switch capability {
	case org.FilesRead, org.FilesWrite, org.FilesDelete:
		return true
	}
	return false
}

// Execute checks the call named name against the grants and runs it. A
// refusal is a one-line error wrapping ErrRefused, for the model to read.
//
// Every call is checked here, in the core, before its executor runs: a bot
// is a permission boundary, not a process boundary, so nothing else stands
// between a model and the owner's files or network.
// See docs/architecture.md#5-security-model.
func (d Dispatcher) Execute(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	def, ok := d.Defs.Lookup(name)
	if !ok {
		return Result{}, refuse(name, "there is no such tool; call only the tools you were given")
	}
	if !d.permitted(def) {
		return Result{}, refuse(name, "this bot is not allowed to use it")
	}
	call := Call{Args: args}
	if folderScoped(def.Capability) {
		requested, err := pathArg(args)
		if err != nil {
			return Result{}, refuse(name, err.Error())
		}
		folders := org.FolderGrants(d.Grants, def.Capability)
		call.Path, call.Folder, err = containPath(d.Root, folders, requested)
		if err != nil {
			return Result{}, refuse(name, err.Error())
		}
	}
	return def.Execute(ctx, call)
}

func refuse(name, reason string) error {
	return fmt.Errorf("%s %w: %s", name, ErrRefused, reason)
}

// pathArg is the path argument every files tool takes.
func pathArg(args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", errors.New("the arguments are not a JSON object with a path")
	}
	if a.Path == "" {
		return "", errors.New("path is required")
	}
	return a.Path, nil
}

// containPath resolves requested, relative to root or absolute, inside the
// first of folders that holds it, and returns the resolved path and the
// resolved folder. Each folder goes through workspace.Resolve as well as the
// path: a grant stores a folder as it was given, and on macOS a folder under
// /var is really under /private/var, so comparing the stored text with a
// resolved path would refuse every call.
func containPath(root string, folders []string, requested string) (path, folder string, err error) {
	abs := requested
	if !filepath.IsAbs(abs) {
		if root == "" {
			return "", "", fmt.Errorf("%s is relative and this bot has no folder", requested)
		}
		abs = filepath.Join(root, requested)
	}
	for _, f := range folders {
		realFolder, err := workspace.Resolve(f, ".")
		if err != nil {
			continue
		}
		if path, err := workspace.Resolve(realFolder, abs); err == nil {
			return path, realFolder, nil
		}
	}
	return "", "", fmt.Errorf("%s is outside the folders this bot may use", requested)
}
