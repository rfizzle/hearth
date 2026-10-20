package deliverables

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/deliverable/template"
	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/render/html"
	"github.com/rfizzle/hearth/internal/render/paper"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/files"
	"github.com/rfizzle/hearth/internal/workspace"
)

// name is the tool's name, and the prefix of every refusal its executor
// returns.
const name = "deliverables.write"

// The extensions the three files take after the stem. The spec's is also
// taken off a path that ends in it, so weekly and weekly.deliverable.json
// name the same files.
const (
	specExt = ".deliverable.json"
	pageExt = ".html"
	pdfExt  = ".pdf"
)

// Measures reads the office's own figures by id; measure.Deps is one.
type Measures interface {
	Resolve(id measure.ID, s measure.Scope) (measure.ID, error)
	Read(ctx context.Context, ids []measure.ID) ([]measure.Value, error)
}

// Loader returns the templates a run is offered: the shipped ones and the
// owner's that load.
type Loader func() []template.Template

// Options are what one run's tool is built from. Templates is called once,
// when the definition is built. BotID and ProjectID are what the aliases me
// and this mean for the run; ProjectID is empty outside a project. Reserved
// are folders no output may land in, whatever is granted. With no Measures, a
// bound widget is refused.
type Options struct {
	Templates        Loader
	Measures         Measures
	BotID, ProjectID string
	Reserved         []string
}

// tool is one run's deliverables.write.
type tool struct {
	templates []template.Template
	measures  Measures
	scope     measure.Scope
	reserved  []string
}

// Definition is deliverables.write for one run. It is checked against the
// write grant, read tier as files.write is: its effect is the files it
// writes in a granted folder, and one approval covers all of them.
// See docs/decisions.md for D44.
func Definition(o Options) tools.Definition {
	t := tool{measures: o.Measures, scope: measure.Scope{BotID: o.BotID, ProjectID: o.ProjectID}, reserved: o.Reserved}
	if o.Templates != nil {
		t.templates = o.Templates()
	}
	return tools.Definition{
		Name:        name,
		Description: description(t.templates),
		Parameters:  parameters(t.templates),
		Capability:  org.FilesWrite,
		Tier:        tools.TierRead,
		Execute:     t.execute,
		Describe:    t.describe,
		Validate: func(args json.RawMessage) error {
			_, err := t.prepare(args)
			return err
		},
	}
}

type args struct {
	Path     string          `json:"path"`
	Template string          `json:"template"`
	Spec     json.RawMessage `json:"spec"`
}

// prepared is a call's spec once it has passed every check that needs no
// store: fitted to its template, valid, and each bound widget's id resolved.
type prepared struct {
	spec  deliverable.Spec
	tpl   *template.Template
	bound []binding
}

// binding is a bound widget, by position, and its resolved id.
type binding struct {
	section, widget int
	id              measure.ID
}

// prepare runs every check that needs no store, in order: the arguments,
// the stem, Parse, Authored, the template or Validate, and each measure id.
// It runs before the call is placed and again in the executor.
func (t tool) prepare(raw json.RawMessage) (prepared, error) {
	var a args
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return prepared{}, errors.New("the arguments are a JSON object with path, spec and, when you use one, template")
	}
	if err := checkStem(a.Path); err != nil {
		return prepared{}, err
	}
	spec, err := parseSpec(a.Spec)
	if err != nil {
		return prepared{}, err
	}
	if err := deliverable.Authored(spec); err != nil {
		return prepared{}, err
	}
	p := prepared{}
	if p.tpl, p.spec, err = t.fit(a.Template, spec); err != nil {
		return prepared{}, err
	}
	if p.bound, err = t.bind(p.spec); err != nil {
		return prepared{}, err
	}
	return p, nil
}

// checkStem refuses a path that names a folder or carries an extension of
// its own: the path is the name the files take, and Hearth adds theirs.
func checkStem(path string) error {
	stem := trimSpecExt(path)
	base := filepath.Base(stem)
	if strings.TrimSpace(path) == "" || strings.HasSuffix(stem, "/") || base == "." || base == ".." || base == "/" {
		return errors.New("path is the name the files take, such as weekly-digest or reports/october; give a name, not a folder")
	}
	switch strings.ToLower(filepath.Ext(stem)) {
	case ".html", ".htm", ".pdf", ".json", ".md", ".markdown":
		return fmt.Errorf("path is the name the files take without an extension, such as weekly-digest; Hearth adds .deliverable.json, .html and .pdf, so leave out %s", filepath.Ext(stem))
	}
	return nil
}

// trimSpecExt is path without a trailing .deliverable.json, in any case.
func trimSpecExt(path string) string {
	if len(path) > len(specExt) && strings.EqualFold(path[len(path)-len(specExt):], specExt) {
		return path[:len(path)-len(specExt)]
	}
	return path
}

// parseSpec reads the spec argument: the object, or the same object sent as
// a string of JSON.
func parseSpec(raw json.RawMessage) (deliverable.Spec, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return deliverable.Spec{}, errors.New("spec is required: the deliverable as a JSON object")
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return deliverable.Spec{}, errors.New("spec is the deliverable as a JSON object")
		}
		raw = []byte(s)
	}
	return deliverable.Parse(raw)
}

// fit applies the template named id, or validates the spec as it is when
// there is none.
func (t tool) fit(id string, spec deliverable.Spec) (*template.Template, deliverable.Spec, error) {
	if id == "" {
		return nil, spec, deliverable.Validate(spec)
	}
	for i := range t.templates {
		if t.templates[i].ID == id {
			out, err := template.Apply(t.templates[i], spec)
			return &t.templates[i], out, err
		}
	}
	ids := make([]string, len(t.templates))
	for i, tpl := range t.templates {
		ids[i] = tpl.ID
	}
	if len(ids) == 0 {
		return nil, spec, fmt.Errorf("there is no template %q here; leave template out and build the sections yourself", id)
	}
	return nil, spec, fmt.Errorf("there is no template %q; use one of %s, or leave template out", id, strings.Join(ids, ", "))
}

// bind parses each bound widget's measure id, checks that its family reads
// the kind of value the widget shows, and resolves its aliases for the
// caller. this in a conversation with no project is refused here, before
// the call is placed.
func (t tool) bind(spec deliverable.Spec) ([]binding, error) {
	var bound []binding
	for i, sec := range spec.Sections {
		for j, w := range sec.Widgets {
			if w.Measure == "" {
				continue
			}
			id, err := t.resolve(w)
			if err != nil {
				return nil, fmt.Errorf("section %d, widget %d: %w", i+1, j+1, err)
			}
			bound = append(bound, binding{section: i, widget: j, id: id})
		}
	}
	return bound, nil
}

func (t tool) resolve(w deliverable.Widget) (measure.ID, error) {
	id, err := measure.ParseID(w.Measure)
	if err != nil {
		return measure.ID{}, err
	}
	kind, err := id.Kind()
	if err != nil {
		return measure.ID{}, err
	}
	if want := valueKind(w.Kind); kind != want {
		return measure.ID{}, fmt.Errorf("%s reads a %s, and a %s widget shows a %s; bind a %s measure", id.Family, kind, w.Kind, want, want)
	}
	if t.measures == nil {
		return measure.ID{}, errors.New("the office's figures cannot be read here; give the value yourself and leave measure out")
	}
	return t.measures.Resolve(id, t.scope)
}

// valueKind is the kind of value a bindable widget shows.
func valueKind(k deliverable.Kind) measure.Kind {
	switch k {
	case deliverable.Progress:
		return measure.Progress
	case deliverable.Light:
		return measure.Light
	}
	return measure.Number
}

// output is one file the call writes: its resolved path, the path as the
// model reads it, and its bytes.
type output struct {
	path, name string
	data       []byte
}

// execute checks the call again, fills the bound widgets, draws every file
// and then writes them, spec first.
// See docs/model.md#work.
func (t tool) execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	p, err := t.prepare(call.Args)
	if err == nil {
		err = t.fill(ctx, &p)
	}
	if err != nil {
		return tools.Result{}, fmt.Errorf("%s: %w; nothing was written", name, err)
	}
	outs, notYet, err := t.draw(call, p.spec)
	if err != nil {
		return tools.Result{}, fmt.Errorf("%s: %w; nothing was written", name, err)
	}
	refs := make([]org.FileRef, 0, len(outs))
	written := make([]string, 0, len(outs))
	for _, o := range outs {
		if err := put(call.Folder, o); err != nil {
			if len(written) > 0 {
				err = fmt.Errorf("%w. Already written: %s", err, strings.Join(written, ", "))
			}
			return tools.Result{}, err
		}
		refs = append(refs, org.FileRef{Path: o.path, Name: filepath.Base(o.path)})
		written = append(written, o.name)
	}
	text := fmt.Sprintf("Wrote %s (%s).", joinAnd(written), t.summary(p))
	if notYet {
		text += " The PDF is not made yet: this version of Hearth writes the spec and the web page."
	}
	return tools.Result{Text: text, Files: refs}, nil
}

// fill reads the bound measures and writes each value, and the time it was
// read, over whatever the bot gave. A subject that does not exist is a
// refusal; a figure that cannot be known yet leaves the widget without a
// value and its caption saying why.
func (t tool) fill(ctx context.Context, p *prepared) error {
	if len(p.bound) == 0 {
		return nil
	}
	ids := make([]measure.ID, len(p.bound))
	for i, b := range p.bound {
		ids[i] = b.id
	}
	values, err := t.measures.Read(ctx, ids)
	if err != nil || len(values) != len(ids) {
		return errors.New("the office's figures could not be read; try again")
	}
	for i, b := range p.bound {
		v := values[i]
		if v.Missing == measure.NoSuchProject || v.Missing == measure.NoSuchBot {
			return fmt.Errorf("section %d, widget %d: %s: %s; bind a project or a bot that exists", b.section+1, b.widget+1, v.ID, strings.ToLower(v.Missing))
		}
		setValue(&p.spec.Sections[b.section].Widgets[b.widget], v)
	}
	if err := deliverable.Validate(p.spec); err != nil {
		return fmt.Errorf("a figure the office keeps does not fit its widget: %w", err)
	}
	return nil
}

// setValue writes v over the widget's own value: the resolved id, the time
// it was read in UTC to the minute, and the number, the progress or the
// light, or nothing and the reason in the caption.
func setValue(w *deliverable.Widget, v measure.Value) {
	w.Measure = v.ID
	w.AsOf = v.AsOf.UTC().Format("2006-01-02T15:04Z")
	w.Value, w.Done, w.Total = nil, nil, nil
	if w.Kind == deliverable.Light {
		w.Tone, w.Word = "", ""
	}
	if v.Missing != "" {
		w.Caption = v.Missing
		return
	}
	switch w.Kind {
	case deliverable.Metric:
		n := v.Number
		switch v.Unit {
		case "usd_micros":
			n, w.Format = n/1e6, "currency"
		case "count", "tokens":
			w.Format = "integer"
		}
		w.Value = &n
	case deliverable.Progress:
		// A budget can be overspent; a progress shows at most all of it.
		done, total := float64(min(v.Done, v.Total)), float64(v.Total)
		w.Done, w.Total = &done, &total
	case deliverable.Light:
		w.Tone, w.Word = string(v.Tone), v.Word
	}
}

// draw makes every file before any is written: the canonical spec, the web
// page and the PDF, named from the call's resolved path and never from the
// spec. notYet reports that the PDF could not be made yet and is left out.
func (t tool) draw(call tools.Call, spec deliverable.Spec) (outs []output, notYet bool, err error) {
	stem := trimSpecExt(call.Path)
	if call.Folder == "" || stem == call.Folder || filepath.Dir(stem) == stem {
		return nil, false, errors.New("the path names the folder itself; give the files a name, such as weekly-digest")
	}
	shown := filepath.Base(stem)
	if rel, err := filepath.Rel(call.Folder, stem); err == nil {
		shown = filepath.ToSlash(rel)
	}
	// Within as well as Inside: a reserved folder that does not exist yet,
	// such as the owner's templates before the first is saved, has nothing
	// on disk for Inside to resolve, and a write here would create it.
	for _, ext := range []string{specExt, pageExt, pdfExt} {
		for _, r := range t.reserved {
			if workspace.Inside(r, stem+ext) || workspace.Within(r, stem+ext) {
				return nil, false, fmt.Errorf("%s is in a folder Hearth keeps for itself", shown+ext)
			}
		}
	}
	data, err := deliverable.Canonical(spec)
	if err != nil {
		return nil, false, err
	}
	page, err := html.Render(spec)
	if err != nil {
		return nil, false, fmt.Errorf("the web page could not be made: %w", err)
	}
	outs = []output{{stem + specExt, shown + specExt, data}, {stem + pageExt, shown + pageExt, page}}
	pdf, _, err := paper.Render(spec, paper.Options{Name: shown + pdfExt})
	switch {
	case errors.Is(err, paper.ErrNotYet):
		return outs, true, nil
	case err != nil:
		return nil, false, fmt.Errorf("the PDF could not be made: %w", err)
	}
	return append(outs, output{stem + pdfExt, shown + pdfExt, pdf}), false, nil
}

// put writes o through files.OpenForWrite, inside folder: a regular file
// replaced, or a new one; never a link, a folder or a file with a second
// name.
func put(folder string, o output) error {
	f, err := files.OpenForWrite(name, folder, o.path, o.name)
	if err != nil {
		return err
	}
	_, err = f.Write(o.data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("%s: could not write %s", name, o.name)
	}
	return nil
}

// summary is what was written, in words: "a weekly digest, 6 widgets, 2
// bound to live measures".
func (t tool) summary(p prepared) string {
	n := 0
	for _, sec := range p.spec.Sections {
		n += len(sec.Widgets)
	}
	s := what(p.tpl, p.spec.Kind) + ", " + plural(n, "widget")
	switch len(p.bound) {
	case 0:
	case 1:
		s += ", 1 bound to a live measure"
	default:
		s += fmt.Sprintf(", %d bound to live measures", len(p.bound))
	}
	return s
}

// what is the deliverable with its article: the template's name, else the
// spec's kind.
func what(tpl *template.Template, kind string) string {
	noun := kind
	if tpl != nil && tpl.Name != "" {
		first, size := utf8.DecodeRuneInString(tpl.Name)
		noun = string(unicode.ToLower(first)) + tpl.Name[size:]
	}
	if noun == "" {
		return "a deliverable"
	}
	if strings.ContainsRune("aeiou", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// joinAnd is "a", "a and b" or "a, b and c".
func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// describe is the card: what is written, its title and size, and the files,
// "Write a weekly digest "Office this week" (6 widgets) as
// weekly-digest.deliverable.json and .html in Dana's folder". Through the
// dispatcher the path starts with the granted folder's name and the folder
// argument repeats it.
func (t tool) describe(raw json.RawMessage) string {
	var a struct {
		args
		Folder string `json:"folder"`
	}
	_ = json.Unmarshal(raw, &a) // a describer never fails; an unreadable call shows less
	file, ok := strings.CutPrefix(filepath.ToSlash(a.Path), a.Folder+"/")
	if a.Folder == "" || !ok {
		file = a.Path
	}
	var tpl *template.Template
	for i := range t.templates {
		if t.templates[i].ID == a.Template {
			tpl = &t.templates[i]
		}
	}
	s := "Write "
	if spec, err := parseSpec(a.Spec); err == nil {
		n := 0
		for _, sec := range spec.Sections {
			n += len(sec.Widgets)
		}
		s += fmt.Sprintf("%s %q (%s)", what(tpl, spec.Kind), spec.Title, plural(n, "widget"))
	} else {
		s += what(tpl, "")
	}
	stem := trimSpecExt(file)
	s += fmt.Sprintf(" as %s%s and %s", stem, specExt, pageExt)
	if a.Folder != "" && ok {
		s += " in " + a.Folder + "'s folder"
	}
	return s
}
