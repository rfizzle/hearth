package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// recorder is a definition's executor that remembers the last call it ran.
type recorder struct{ last *Call }

func (r *recorder) def(name string, capability org.Capability, tier Tier) Definition {
	return Definition{
		Name:       name,
		Capability: capability,
		Tier:       tier,
		Execute: func(_ context.Context, call Call) (Result, error) {
			r.last = &call
			return Result{Text: "ran " + name}, nil
		},
		Describe: func(json.RawMessage) string { return name },
	}
}

func registry(t *testing.T, r *recorder) *Registry {
	t.Helper()
	reg, err := NewRegistry(
		r.def("files.read", org.FilesRead, TierRead),
		r.def("files.write", org.FilesWrite, TierRead),
		r.def("web.fetch", org.WebFetch, TierRead),
		r.def("calendar.write", "calendar.write", TierAct),
	)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcher_Execute(t *testing.T) {
	// base is deliberately not resolved: on macOS t.TempDir is under /var,
	// which is a symlink to /private/var, the trap grants fall into.
	base := t.TempDir()
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	bot := filepath.Join(base, "Hearth", "Writer")
	outside := filepath.Join(base, "elsewhere")
	mkdir(t, bot)
	mkdir(t, outside)
	symlink(t, outside, filepath.Join(bot, "out"))
	// A granted folder reached through a symlinked parent.
	symlink(t, filepath.Join(base, "Hearth"), filepath.Join(base, "alias"))
	aliased := filepath.Join(base, "alias", "Writer")

	readWrite := func(folder string) []org.Grant {
		return []org.Grant{
			{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.WebFetch, Mode: org.ModeAllowed},
		}
	}
	readOnly := []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: bot}}
	askWrite := []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: bot}}
	inBot := filepath.Join(real, "Hearth", "Writer")

	cases := []struct {
		name     string
		grants   []org.Grant
		root     string
		tool     string
		args     string
		wantPath string // the resolved path the executor sees; "" when refused
		refusal  string // a fragment of the refusal; "" when it runs
	}{
		{"a relative path inside", readWrite(bot), bot, "files.write", `{"path":"brief.md"}`, filepath.Join(inBot, "brief.md"), ""},
		{"a new subfolder inside", readWrite(bot), bot, "files.write", `{"path":"notes/a.md"}`, filepath.Join(inBot, "notes", "a.md"), ""},
		{"an absolute path inside", readWrite(bot), bot, "files.read", `{"path":"` + filepath.Join(bot, "x.md") + `"}`, filepath.Join(inBot, "x.md"), ""},
		{"a granted folder through a symlinked parent", readWrite(aliased), aliased, "files.write", `{"path":"brief.md"}`, filepath.Join(inBot, "brief.md"), ""},
		{"a bot without files.write", readOnly, bot, "files.write", `{"path":"brief.md"}`, "", "not allowed"},
		{"a grant in ask mode", askWrite, bot, "files.write", `{"path":"brief.md"}`, "", "needs the owner's approval"},
		{"a path through a symlink out", readWrite(bot), bot, "files.read", `{"path":"out/secret.txt"}`, "", "outside the folders"},
		{"dot-dot out of the folder", readWrite(bot), bot, "files.write", `{"path":"../../elsewhere/x.md"}`, "", "outside the folders"},
		{"an absolute path outside", readWrite(bot), bot, "files.read", `{"path":"` + filepath.Join(outside, "x") + `"}`, "", "outside the folders"},
		{"a relative path with no root", readWrite(bot), "", "files.read", `{"path":"x.md"}`, "", "no folder"},
		{"a relative root", readWrite(bot), "Hearth/Writer", "files.write", `{"path":"x.md"}`, "", "not an absolute path"},
		{"a relative root with an absolute path", readWrite(bot), "Hearth/Writer", "files.read", `{"path":"` + filepath.Join(bot, "x.md") + `"}`, "", "not an absolute path"},
		{"a granted folder that does not exist", readWrite(filepath.Join(base, "gone")), bot, "files.read", `{"path":"x.md"}`, "", "outside the folders"},
		{"no path", readWrite(bot), bot, "files.read", `{}`, "", "path is required"},
		{"arguments that are not an object", readWrite(bot), bot, "files.read", `"x"`, "", "not a JSON object"},
		{"an unknown tool", readWrite(bot), bot, "files.delete", `{"path":"x"}`, "", "no such tool"},
		{"an act-tier tool even when granted", []org.Grant{{Capability: "calendar.write", Mode: org.ModeAllowed}}, bot, "calendar.write", `{}`, "", "needs the owner's approval"},
		{"a granted web tool", readWrite(bot), bot, "web.fetch", `{"url":"https://example.org"}`, "", ""},
		{"an ungranted web tool", readOnly, bot, "web.fetch", `{"url":"https://example.org"}`, "", "not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			d := Dispatcher{Defs: registry(t, rec), Grants: tc.grants, Root: tc.root}
			res, err := d.Execute(context.Background(), tc.tool, json.RawMessage(tc.args))
			if tc.refusal != "" {
				if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("err = %v, want a refusal containing %q", err, tc.refusal)
				}
				if strings.Contains(err.Error(), "\n") || !strings.HasPrefix(err.Error(), tc.tool+" ") {
					t.Fatalf("refusal %q is not one line naming the tool", err)
				}
				if rec.last != nil {
					t.Fatal("the executor ran after a refusal")
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Text != "ran "+tc.tool || rec.last == nil {
				t.Fatalf("result %q, executor ran %v", res.Text, rec.last != nil)
			}
			if rec.last.Path != tc.wantPath {
				t.Fatalf("path = %q, want %q", rec.last.Path, tc.wantPath)
			}
			if tc.wantPath != "" && rec.last.Folder != inBot {
				t.Fatalf("folder = %q, want %q", rec.last.Folder, inBot)
			}
		})
	}
}

func TestDispatcher_Tools(t *testing.T) {
	folder := t.TempDir()
	rec := &recorder{}
	cases := []struct {
		name   string
		grants []org.Grant
		want   []string
	}{
		{"no grants", nil, nil},
		{"the defaults", []org.Grant{
			{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesDelete, Mode: org.ModeAsk, Folder: folder},
			{Capability: org.WebFetch, Mode: org.ModeAllowed},
		}, []string{"files.read", "files.write", "web.fetch"}},
		{"ask mode is offered", []org.Grant{{Capability: org.WebFetch, Mode: org.ModeAsk}}, []string{"web.fetch"}},
		{"a files grant with no folder", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAllowed}}, nil},
		{"an act-tier grant", []org.Grant{{Capability: "calendar.write", Mode: org.ModeAllowed}}, []string{"calendar.write"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Dispatcher{Defs: registry(t, rec), Grants: tc.grants, Root: folder}
			var got []string
			for _, def := range d.Tools() {
				got = append(got, def.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Tools = %v, want %v", got, tc.want)
			}
		})
	}
}

// gatedRegistry is registry with a delete tool that has no standing.
func gatedRegistry(t *testing.T, r *recorder) *Registry {
	t.Helper()
	reg := registry(t, r)
	del := r.def("files.delete", org.FilesDelete, TierRead)
	del.NoStanding = true
	if err := reg.Add(del); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestDispatcher_Decide(t *testing.T) {
	base := t.TempDir()
	bot := filepath.Join(base, "Writer")
	shared := filepath.Join(base, "Shared")
	inner := filepath.Join(shared, "Allowed")
	mkdir(t, bot)
	mkdir(t, inner)
	grants := []org.Grant{
		{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: bot},
		{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: bot},
		{Capability: org.FilesDelete, Mode: org.ModeAllowed, Folder: bot},
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: shared},
		{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: inner},
		{Capability: org.WebFetch, Mode: org.ModeAsk},
		{Capability: "calendar.write", Mode: org.ModeAllowed},
	}
	allow := func(c org.Capability, folder string) []inbox.Allowance {
		return []inbox.Allowance{{ConversationID: "c", Capability: c, Folder: folder}}
	}
	inShared := `{"path":"` + filepath.Join(shared, "a.md") + `"}`
	cases := []struct {
		name       string
		allowances []inbox.Allowance
		tool, args string
		want       Verdict
		granted    string // the grant's folder Check reports
		refusal    string
	}{
		{"an allowed folder runs", nil, "files.write", `{"path":"a.md"}`, Auto, bot, ""},
		{"an ask-mode folder waits", nil, "files.write", inShared, Gated, shared, ""},
		{"an allowance for the folder runs", allow(org.FilesWrite, shared), "files.write", inShared, Auto, shared, ""},
		{"an allowance for another capability waits", allow(org.FilesRead, shared), "files.write", inShared, Gated, shared, ""},
		{"an allowance for another folder waits", allow(org.FilesWrite, bot), "files.write", inShared, Gated, shared, ""},
		{"an allowed folder inside an ask-mode one runs", nil, "files.write", `{"path":"` + filepath.Join(inner, "b.md") + `"}`, Auto, inner, ""},
		{"no standing waits under an allowed grant", nil, "files.delete", `{"path":"a.md"}`, Gated, bot, ""},
		{"no standing runs under an allowance", allow(org.FilesDelete, bot), "files.delete", `{"path":"a.md"}`, Auto, bot, ""},
		{"an ask-mode tool with no folder waits", nil, "web.fetch", `{"url":"https://example.org"}`, Gated, "", ""},
		{"an allowance with no folder runs it", allow(org.WebFetch, ""), "web.fetch", `{"url":"https://example.org"}`, Auto, "", ""},
		{"an act-tier tool waits even when allowed", nil, "calendar.write", `{}`, Gated, "", ""},
		{"a path outside every folder is refused", nil, "files.write", `{"path":"` + filepath.Join(base, "x.md") + `"}`, 0, "", "outside the folders"},
		{"an ungranted capability is refused", nil, "web.search", `{}`, 0, "", "no such tool"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			d := Dispatcher{Defs: gatedRegistry(t, rec), Grants: grants, Allowances: tc.allowances, Root: bot}
			c, err := d.Check(tc.tool, json.RawMessage(tc.args))
			if tc.refusal != "" {
				if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("err = %v, want a refusal containing %q", err, tc.refusal)
				}
				return
			}
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if c.Verdict != tc.want || c.Granted != tc.granted {
				t.Fatalf("verdict %d in %q, want %d in %q", c.Verdict, c.Granted, tc.want, tc.granted)
			}
			if v, err := d.Decide(tc.tool, json.RawMessage(tc.args)); v != tc.want || err != nil {
				t.Fatalf("Decide = %d, %v, want %d", v, err, tc.want)
			}
			if got := d.Gated(tc.tool, json.RawMessage(tc.args)); got != (tc.want == Gated) {
				t.Fatalf("Gated = %v, want %v", got, tc.want == Gated)
			}
			// Execute runs exactly what Decide calls Auto.
			_, err = d.Execute(context.Background(), tc.tool, json.RawMessage(tc.args))
			if ran := rec.last != nil; ran != (tc.want == Auto) {
				t.Fatalf("Execute ran = %v (err %v), want %v", ran, err, tc.want == Auto)
			}
		})
	}
}

func TestDispatcher_ExecuteApproved(t *testing.T) {
	base := t.TempDir()
	bot := filepath.Join(base, "Writer")
	shared := filepath.Join(base, "Shared")
	mkdir(t, bot)
	mkdir(t, shared)
	grants := []org.Grant{
		{Capability: org.FilesDelete, Mode: org.ModeAsk, Folder: bot},
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: shared},
	}
	cases := []struct {
		name, tool, args string
		refusal          string
	}{
		{"a gated call in its folder runs", "files.write", `{"path":"` + filepath.Join(shared, "a.md") + `"}`, ""},
		{"a call with no standing runs", "files.delete", `{"path":"a.md"}`, ""},
		{"a path outside the folders is still refused", "files.write", `{"path":"` + filepath.Join(bot, "a.md") + `"}`, "outside the folders"},
		{"dot-dot out is still refused", "files.delete", `{"path":"../Shared/a.md"}`, "outside the folders"},
		{"an ungranted tool is still refused", "files.read", `{"path":"a.md"}`, "not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			d := Dispatcher{Defs: gatedRegistry(t, rec), Grants: grants, Root: bot}
			_, err := d.ExecuteApproved(context.Background(), tc.tool, json.RawMessage(tc.args))
			if tc.refusal == "" {
				if err != nil || rec.last == nil {
					t.Fatalf("ExecuteApproved = %v, ran %v", err, rec.last != nil)
				}
				return
			}
			if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), tc.refusal) || rec.last != nil {
				t.Fatalf("err = %v, ran %v, want a refusal containing %q", err, rec.last != nil, tc.refusal)
			}
		})
	}
}

func TestChecked_DescribeNamesTheFolderTheCallLandsIn(t *testing.T) {
	base := t.TempDir()
	bot := filepath.Join(base, "Dana")
	other := filepath.Join(base, "Other")
	mkdir(t, bot)
	mkdir(t, other)
	symlink(t, other, filepath.Join(bot, "link"))
	grants := []org.Grant{
		{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: bot},
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: other},
	}
	reg, err := NewRegistry(Definition{
		Name: "files.write", Capability: org.FilesWrite, Tier: TierRead,
		Execute: func(context.Context, Call) (Result, error) { return Result{}, nil },
		Describe: func(args json.RawMessage) string {
			var a struct{ Path string }
			_ = json.Unmarshal(args, &a)
			return "Write " + a.Path
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	d := Dispatcher{Defs: reg, Grants: grants, Root: bot}
	cases := []struct{ name, path, want string }{
		{"dot-dot out of the bot's folder", "notes/../../Other/tax.md", "Write Other/tax.md"},
		{"a link out of the bot's folder", "link/tax.md", "Write Other/tax.md"},
		{"an absolute path", filepath.Join(other, "a", "b.md"), "Write Other/a/b.md"},
		{"a path in the bot's own folder", "brief.md", "Write Dana/brief.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := d.Check("files.write", json.RawMessage(`{"path":"`+tc.path+`","content":"x"}`))
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Describe(); got != tc.want {
				t.Fatalf("Describe = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDispatcher_NestedGrantsPlaceTheCallInTheInnerFolder(t *testing.T) {
	base := t.TempDir()
	outer := filepath.Join(base, "A")
	inner := filepath.Join(outer, "B")
	mkdir(t, inner)
	for _, order := range [][]string{{outer, inner}, {inner, outer}} {
		var grants []org.Grant
		for _, f := range order {
			grants = append(grants, org.Grant{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: f})
		}
		d := Dispatcher{Defs: registry(t, &recorder{}), Grants: grants, Root: base}
		c, err := d.Check("files.write", json.RawMessage(`{"path":"`+filepath.Join(inner, "x.md")+`"}`))
		if err != nil || c.Granted != inner {
			t.Fatalf("grants in order %v: granted %q, %v, want %q", order, c.Granted, err, inner)
		}
	}
}

func TestDispatcher_ValidateRefusesBeforeTheGate(t *testing.T) {
	rec := &recorder{}
	ask := rec.def("ask_owner", org.AskOwner, TierAct)
	ask.NoStanding = true
	ask.Validate = func(args json.RawMessage) error {
		if !strings.Contains(string(args), "question") {
			return errors.New("question is required")
		}
		return nil
	}
	reg, err := NewRegistry(ask)
	if err != nil {
		t.Fatal(err)
	}
	d := Dispatcher{Defs: reg, Grants: []org.Grant{{Capability: org.AskOwner, Mode: org.ModeAllowed}}}
	cases := []struct {
		name, args string
		want       Verdict
		refusal    string
	}{
		{"a valid request waits for the owner", `{"question":"which?"}`, Gated, ""},
		{"a malformed request is refused, not gated", `{}`, 0, "ask_owner refused: question is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := d.Decide("ask_owner", json.RawMessage(tc.args))
			if tc.refusal == "" {
				if v != tc.want || err != nil {
					t.Fatalf("Decide = %d, %v, want %d", v, err, tc.want)
				}
				return
			}
			if v != 0 || !errors.Is(err, ErrRefused) || err.Error() != tc.refusal || d.Gated("ask_owner", json.RawMessage(tc.args)) {
				t.Fatalf("Decide = %d, %v, want the refusal %q and no gate", v, err, tc.refusal)
			}
			if _, err := d.Execute(context.Background(), "ask_owner", json.RawMessage(tc.args)); err == nil || err.Error() != tc.refusal || rec.last != nil {
				t.Fatalf("Execute = %v (ran %v), want the refusal and no run", err, rec.last != nil)
			}
		})
	}
}

func TestDefinition_Standing(t *testing.T) {
	for _, tc := range []struct {
		def  Definition
		want bool
	}{
		{Definition{Tier: TierRead}, true},
		{Definition{Tier: TierRead, NoStanding: true}, false},
		{Definition{Tier: TierAct}, false},
	} {
		if got := tc.def.Standing(); got != tc.want {
			t.Errorf("%+v.Standing() = %v, want %v", tc.def, got, tc.want)
		}
	}
}

func TestRegistry(t *testing.T) {
	rec := &recorder{}
	read := rec.def("files.read", org.FilesRead, TierRead)
	if _, err := NewRegistry(read, read); err == nil {
		t.Fatal("NewRegistry accepted a name twice")
	}
	if _, err := NewRegistry(Definition{Name: "x"}); err == nil {
		t.Fatal("NewRegistry accepted a definition with no executor")
	}
	var r Registry
	if err := r.Add(read); err != nil {
		t.Fatal(err)
	}
	if got, ok := r.Lookup("files.read"); !ok || got.Name != "files.read" {
		t.Fatalf("Lookup = %v, %v", got.Name, ok)
	}
	if _, ok := r.Lookup("web.fetch"); ok {
		t.Fatal("Lookup found a tool never added")
	}
	if all := r.All(); len(all) != 1 {
		t.Fatalf("All = %d definitions, want 1", len(all))
	}
	var none *Registry
	if none.All() != nil {
		t.Fatal("a nil registry is not empty")
	}
}

func TestDispatcher_NoGrantReachesAReservedFolder(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, "Library", "hearth")
	mkdir(t, filepath.Join(data, "keys"))
	mkdir(t, filepath.Join(home, "Documents"))
	link := filepath.Join(home, "Documents", "sneak")
	symlink(t, filepath.Join(data, "keys"), link)

	d := Dispatcher{
		Defs:     registry(t, &recorder{}),
		Grants:   []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: home}},
		Root:     filepath.Join(home, "Documents"),
		Reserved: func() []string { return []string{data} },
	}
	for name, path := range map[string]string{
		"the data folder":      filepath.Join(data, "hearth.db"),
		"a file in its keys":   filepath.Join(data, "keys", "openai"),
		"a link into it":       filepath.Join(link, "openai"),
		"a new file in it":     filepath.Join(data, "new.md"),
		"the folder itself":    data,
		"a relative way to it": "../Library/hearth/hearth.db",
	} {
		if _, err := d.Check("files.write", json.RawMessage(`{"path":"`+path+`"}`)); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
	}
	if _, err := d.Check("files.write", json.RawMessage(`{"path":"`+filepath.Join(home, "Documents", "ok.md")+`"}`)); err != nil {
		t.Errorf("a file beside the reserved folder = %v, want it allowed", err)
	}
	d.Reserved = nil
	if _, err := d.Check("files.write", json.RawMessage(`{"path":"`+filepath.Join(data, "hearth.db")+`"}`)); err != nil {
		t.Errorf("with nothing reserved the same grant reaches it: %v", err)
	}
}

// A reserved folder that is not made yet, such as the owner's templates
// before the first is saved, is refused as one that is, whether the grant
// names the Hearth folder directly or through a link.
func TestDispatcher_AReservedFolderCountsBeforeItExists(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hearth := filepath.Join(base, "Hearth")
	mkdir(t, hearth)
	link := filepath.Join(base, "Link")
	symlink(t, hearth, link)
	check := func(folder, reserved, path string) error {
		d := Dispatcher{
			Defs:     registry(t, &recorder{}),
			Grants:   []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder}},
			Root:     folder,
			Reserved: func() []string { return []string{reserved} },
		}
		_, err := d.Check("files.write", json.RawMessage(`{"path":"`+path+`"}`))
		return err
	}
	templates := filepath.Join(hearth, "Templates")
	if err := check(hearth, templates, filepath.Join(templates, "x.deliverable.json")); !errors.Is(err, ErrRefused) {
		t.Errorf("Templates not made yet: err = %v, want a refusal", err)
	}
	linked := filepath.Join(link, "Templates")
	if err := check(link, linked, filepath.Join(linked, "y.deliverable.json")); !errors.Is(err, ErrRefused) {
		t.Errorf("Templates not made yet, through a link: err = %v, want a refusal", err)
	}
	if err := check(hearth, templates, filepath.Join(hearth, "Templates.md")); err != nil {
		t.Errorf("a file beside the folder not made yet: %v", err)
	}
	mkdir(t, templates)
	if err := check(hearth, templates, filepath.Join(templates, "x.deliverable.json")); !errors.Is(err, ErrRefused) {
		t.Errorf("Templates made: err = %v, want a refusal", err)
	}
	if _, err := os.Stat(filepath.Join(templates, "x.deliverable.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a check wrote a file: %v", err)
	}
}
