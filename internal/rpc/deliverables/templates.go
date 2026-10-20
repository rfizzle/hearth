package deliverables

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/deliverable/template"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/workspace"
)

const (
	folder = "Templates"
	suffix = ".deliverable.json"
)

// ListDeliverableTemplates is the shipped five, the owner's after them, and
// each file that could not be used with its reason.
// See docs/model.md#work
func (s Templates) ListDeliverableTemplates(context.Context, *connect.Request[hearthv1.ListDeliverableTemplatesRequest]) (*connect.Response[hearthv1.ListDeliverableTemplatesResponse], error) {
	root := s.root()
	loaded := template.Load(root)
	out := make([]*hearthv1.DeliverableTemplate, 0, len(loaded.Templates)+len(loaded.Problems))
	for _, t := range loaded.Templates {
		out = append(out, templateToProto(root, t))
	}
	for _, p := range loaded.Problems {
		out = append(out, &hearthv1.DeliverableTemplate{Id: p.ID, Path: filepath.Join(root, folder, p.File), Problem: p.Reason})
	}
	return connect.NewResponse(&hearthv1.ListDeliverableTemplatesResponse{Templates: out}), nil
}

// SaveDeliverableTemplate writes a template the template package accepts as
// Templates/<id>.deliverable.json, replacing the owner's of that id.
func (s Templates) SaveDeliverableTemplate(_ context.Context, req *connect.Request[hearthv1.SaveDeliverableTemplateRequest]) (*connect.Response[hearthv1.SaveDeliverableTemplateResponse], error) {
	root := s.root()
	if root == "" {
		return nil, noFolder()
	}
	t, err := template.Parse([]byte(req.Msg.GetJson()))
	if err != nil {
		return nil, refused(err)
	}
	if isShipped(t.ID) {
		return nil, wire.Invalid("%s is a shipped template; pick another id", t.ID)
	}
	path, err := s.path(root, t.ID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, wire.Fail(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return nil, wire.Invalid("the template could not be written as a plain file: %v", err)
	}
	_, err = f.WriteString(req.Msg.GetJson())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, wire.Fail(err)
	}
	return connect.NewResponse(&hearthv1.SaveDeliverableTemplateResponse{Template: templateToProto(root, t)}), nil
}

// DeleteDeliverableTemplate removes one of the owner's templates. A shipped
// one, a name that is not an id and a link or folder are refused; one that is
// not there is not found.
func (s Templates) DeleteDeliverableTemplate(_ context.Context, req *connect.Request[hearthv1.DeleteDeliverableTemplateRequest]) (*connect.Response[hearthv1.DeleteDeliverableTemplateResponse], error) {
	root, id := s.root(), req.Msg.GetId()
	if root == "" {
		return nil, noFolder()
	}
	if !validID(id) {
		return nil, wire.Invalid("%q is not a template id: 1 to 40 letters a to z, digits and hyphens", id)
	}
	if isShipped(id) {
		return nil, wire.Invalid("%s is a shipped template and cannot be deleted", id)
	}
	path, err := s.path(root, id)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, connect.NewError(connect.CodeNotFound, errors.New("there is no template "+id))
	case err != nil:
		return nil, wire.Fail(err)
	case !info.Mode().IsRegular():
		return nil, wire.Invalid("%s is a link or a folder; only a template file is removed here", id)
	}
	return wire.Reply(&hearthv1.DeleteDeliverableTemplateResponse{}, os.Remove(path))
}

func (s Deps) root() string {
	if s.Root == nil {
		return ""
	}
	return s.Root()
}

func noFolder() error {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New("there is no Hearth folder yet; choose one in Settings first"))
}

// path is where the template id lives: directly in the Templates folder,
// resolved through the Hearth folder so no link leads out of it.
func (s Templates) path(root, id string) (string, error) {
	dir, err := workspace.Resolve(root, folder)
	if err != nil {
		return "", wire.Invalid("the Templates folder cannot be used: %v", err)
	}
	path, err := workspace.Resolve(root, filepath.Join(folder, id+suffix))
	if err != nil || filepath.Dir(path) != dir {
		return "", wire.Invalid("%s cannot be placed in the Templates folder", id)
	}
	return path, nil
}

func isShipped(id string) bool {
	return slices.ContainsFunc(template.Shipped(), func(t template.Template) bool { return t.ID == id })
}

// validID is the template id rule, so a request cannot name a path.
func validID(id string) bool {
	if id == "" || len(id) > 40 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// templateToProto is a template; an owner's carries its file and the
// document as written.
func templateToProto(root string, t template.Template) *hearthv1.DeliverableTemplate {
	out := &hearthv1.DeliverableTemplate{Id: t.ID, Name: t.Name, When: t.When, Shipped: t.Shipped}
	if !t.Shipped {
		out.Path = filepath.Join(root, folder, t.ID+suffix)
		if data, err := os.ReadFile(out.Path); err == nil {
			out.Json = string(data)
		}
	}
	return out
}
