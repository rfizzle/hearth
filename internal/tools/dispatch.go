package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/workspace"
)

// ErrRefused is wrapped by every error the dispatcher returns instead of
// running a call.
var ErrRefused = errors.New("refused")

// Dispatcher runs one bot's calls. Defs is every tool configured for the
// run, Grants the bot's grants, Allowances what the owner has allowed for
// the conversation the run is in, and Root the folder a relative path in a
// files call starts from.
type Dispatcher struct {
	Defs       *Registry
	Grants     []org.Grant
	Allowances []inbox.Allowance
	Root       string
	// Reserved are folders no files call may reach whatever is granted, such
	// as the folder hearth keeps its own files in.
	Reserved []string
}

// Verdict is what the grants say about a call that may run.
type Verdict int

// The verdicts. A call the grants refuse has none; Check returns the refusal.
const (
	// Auto runs the call now.
	Auto Verdict = iota + 1
	// Gated waits for the owner to decide.
	Gated
)

// Checked is a call the grants let through, as the dispatcher placed it.
// Granted is the folder of the grant the call falls under, as that grant
// names it, and empty for a capability with no folder.
type Checked struct {
	Def     Definition
	Verdict Verdict
	Call    Call
	Granted string
}

// Describe is the call in the owner's words with its path as the dispatcher
// resolved it: the name of the granted folder it fell under, then the path
// inside it, and that name again as the folder argument for a describer
// that words the two apart. A path the model wrote through .. or a link
// reads as the folder it lands in, never as the bot's paraphrase of it.
// See docs/model.md#8-approvals.
func (c Checked) Describe() string {
	if c.Def.Describe == nil {
		return ""
	}
	args := c.Call.Args
	if rel, err := filepath.Rel(c.Call.Folder, c.Call.Path); c.Call.Path != "" && err == nil {
		var fields map[string]json.RawMessage
		if json.Unmarshal(args, &fields) == nil {
			fields["path"], _ = json.Marshal(filepath.Join(filepath.Base(c.Granted), rel))
			fields["folder"], _ = json.Marshal(filepath.Base(c.Granted))
			if b, err := json.Marshal(fields); err == nil {
				args = b
			}
		}
	}
	return c.Def.Describe(args)
}

// Tools is the definitions the grants offer, in registry order: the toolset
// the model is offered. A capability granted in ask mode is offered too; a
// call to it waits for the owner. A tool it is not offered is refused if
// called.
func (d Dispatcher) Tools() []Definition {
	var offered []Definition
	for _, def := range d.Defs.All() {
		if d.offered(def) {
			offered = append(offered, def)
		}
	}
	return offered
}

// offered reports whether any grant gives def's capability, asking or not.
func (d Dispatcher) offered(def Definition) bool {
	if folderScoped(def.Capability) {
		return len(org.Folders(d.Grants, def.Capability, org.ModeAsk)) > 0
	}
	return org.Allowed(d.Grants, def.Capability) >= org.ModeAsk
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

// Decide is whether the call named name runs now or waits for the owner. A
// call the grants refuse is the refusal Execute would return.
func (d Dispatcher) Decide(name string, args json.RawMessage) (Verdict, error) {
	c, err := d.Check(name, args)
	return c.Verdict, err
}

// Gated reports whether the call named name waits for the owner: the
// approval gate a run is handed. A call the grants refuse is not gated;
// Execute gives the model the refusal.
// See docs/model.md#8-approvals.
func (d Dispatcher) Gated(name string, args json.RawMessage) bool {
	v, err := d.Decide(name, args)
	return err == nil && v == Gated
}

// Check places the call named name against the grants: the definition, the
// resolved path for a files capability, the grant it falls under and the
// verdict. A call waits for the owner when the grant it falls under asks,
// or when its tool may never be allowed for good, unless an allowance for
// the conversation covers it. Arguments the tool's own check refuses are
// refused here, before any verdict, so a malformed request never waits for
// the owner. A refusal is a one-line error wrapping ErrRefused, for the
// model to read.
// See docs/model.md#8-approvals.
func (d Dispatcher) Check(name string, args json.RawMessage) (Checked, error) {
	def, ok := d.Defs.Lookup(name)
	if !ok {
		return Checked{}, refuse(name, "there is no such tool; call only the tools you were given")
	}
	if !d.offered(def) {
		return Checked{}, refuse(name, "this bot is not allowed to use it")
	}
	if def.Validate != nil {
		if err := def.Validate(args); err != nil {
			return Checked{}, refuse(name, err.Error())
		}
	}
	c := Checked{Def: def, Call: Call{Args: args}}
	mode := org.Allowed(d.Grants, def.Capability)
	if folderScoped(def.Capability) {
		requested, err := pathArg(args)
		if err != nil {
			return Checked{}, refuse(name, err.Error())
		}
		var g org.Grant
		g, c.Call.Path, c.Call.Folder, err = place(d.Root, d.Reserved, d.Grants, def.Capability, requested)
		if err != nil {
			return Checked{}, refuse(name, err.Error())
		}
		mode, c.Granted = g.Mode, g.Folder
		c.Call.Requested = absolute(d.Root, requested)
	}
	c.Verdict = verdict(def, mode, d.allowanceCovers(def.Capability, c.Call.Path))
	return c, nil
}

// verdict is Auto for a call an allowance covers, Gated for one whose grant
// asks or whose tool has no standing, and Auto otherwise.
func verdict(def Definition, mode org.Mode, allowance bool) Verdict {
	if !allowance && (mode == org.ModeAsk || !def.Standing()) {
		return Gated
	}
	return Auto
}

// allowanceCovers reports whether an allowance lets capability run at path:
// one with no folder for a capability with none, or one whose folder holds
// the resolved path.
func (d Dispatcher) allowanceCovers(capability org.Capability, path string) bool {
	for _, a := range d.Allowances {
		if a.Capability != capability {
			continue
		}
		if !folderScoped(capability) {
			return true
		}
		if a.Folder == "" || path == "" {
			continue
		}
		if real, err := workspace.Resolve(a.Folder, "."); err == nil {
			if _, err := workspace.Resolve(real, path); err == nil {
				return true
			}
		}
	}
	return false
}

// Execute checks the call named name against the grants and runs it when
// it may run now. A call that waits for the owner is refused here: it runs
// through ExecuteApproved once they have decided. A refusal is a one-line
// error wrapping ErrRefused, for the model to read.
//
// Every call is checked here, in the core, before its executor runs: a bot
// is a permission boundary, not a process boundary, so nothing else stands
// between a model and the owner's files or network.
// See docs/architecture.md#5-security-model.
func (d Dispatcher) Execute(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	c, err := d.Check(name, args)
	if err != nil {
		return Result{}, err
	}
	if c.Verdict != Auto {
		return Result{}, refuse(name, "it needs the owner's approval, and none was given")
	}
	return c.Def.Execute(ctx, c.Call)
}

// ExecuteApproved runs a call the owner has approved. It skips the verdict
// and keeps every other check: the tool must still be granted, and a files
// call's path must still lie in a folder its capability is granted in.
// See docs/model.md#8-approvals.
func (d Dispatcher) ExecuteApproved(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	c, err := d.Check(name, args)
	if err != nil {
		return Result{}, err
	}
	return c.Def.Execute(ctx, c.Call)
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

// absolute is requested as a clean absolute path, joined to root when
// relative, with no link followed.
func absolute(root, requested string) string {
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(root, requested)
	}
	return filepath.Clean(requested)
}

// errOutside ends a refusal for a path in none of the granted folders.
var errOutside = errors.New("is outside the folders this bot may use")

// place finds the grant a files call to capability falls under, trying the
// folders granted without asking before those granted in ask mode, so a path
// in both runs without asking. It returns the grant, the resolved path and
// the resolved folder.
func place(root string, reserved []string, grants []org.Grant, capability org.Capability, requested string) (org.Grant, string, string, error) {
	for _, mode := range []org.Mode{org.ModeAllowed, org.ModeAsk} {
		var scoped []org.Grant
		var folders []string
		for _, g := range grants {
			if g.Capability == capability && g.Mode == mode && g.Folder != "" {
				scoped = append(scoped, g)
				folders = append(folders, g.Folder)
			}
		}
		path, folder, i, err := containPath(root, reserved, folders, requested)
		if err == nil {
			return scoped[i], path, folder, nil
		}
		if !errors.Is(err, errOutside) {
			return org.Grant{}, "", "", err
		}
	}
	return org.Grant{}, "", "", fmt.Errorf("%s %w", requested, errOutside)
}

// containPath resolves requested, relative to root or absolute, inside the
// most specific of folders that holds it, the one deepest on disk, and
// returns the resolved path, the resolved folder and its index: a decision
// on a call in a nested folder is about that folder, not one around it,
// whichever the owner granted first. Each folder goes through workspace.Resolve
// as well as the path: a grant stores a folder as it was given, and on macOS
// a folder under /var is really under /private/var, so comparing the stored
// text with a resolved path would refuse every call.
func containPath(root string, reserved, folders []string, requested string) (path, folder string, index int, err error) {
	// A relative root would be joined on again under each granted folder by
	// workspace.Resolve, so the bot's files would land somewhere the owner
	// never picked.
	if root != "" && !filepath.IsAbs(root) {
		return "", "", 0, errors.New("this bot's folder is not an absolute path, so no file call can run")
	}
	if root == "" && !filepath.IsAbs(requested) {
		return "", "", 0, fmt.Errorf("%s is relative and this bot has no folder", requested)
	}
	abs := absolute(root, requested)
	index = -1
	for i, f := range folders {
		realFolder, err := workspace.Resolve(f, ".")
		if err != nil || len(realFolder) <= len(folder) {
			continue
		}
		if p, err := workspace.Resolve(realFolder, abs); err == nil {
			path, folder, index = p, realFolder, i
		}
	}
	if index < 0 {
		return "", "", 0, fmt.Errorf("%s %w", requested, errOutside)
	}
	for _, r := range reserved {
		if _, err := workspace.Resolve(r, path); err == nil {
			return "", "", 0, fmt.Errorf("%s is in a folder hearth keeps for itself", requested)
		}
	}
	return path, folder, index, nil
}
