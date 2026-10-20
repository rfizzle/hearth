package rpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/rpc/deliverables"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The measure and template calls end to end over the in-memory listener,
// against the rig's real store and a Hearth folder in a temp dir.

type spendLedger struct{}

func (spendLedger) Month(context.Context, string) (measure.Month, error) {
	return measure.Month{Org: measure.Spend{CostMicros: 2_000_000, Tokens: 10, Priced: true}}, nil
}

type fixedCount int

func (n fixedCount) Open(context.Context) (int, error)                { return int(n), nil }
func (n fixedCount) Count(context.Context, string) (int, error)       { return int(n), nil }
func (fixedCount) Used(context.Context, string) (int64, int64, error) { return 1, 2, nil }

// withDeliverables sets the options the way hearthd does, over the rig's
// store and orchestrator and the folder the test names.
func withDeliverables(r *rig) {
	n := fixedCount(2)
	r.opts.Deliverables = deliverables.Deps{
		Measures: measure.Deps{
			Roster: r.opts.Store, Projects: r.opts.Store, Status: r.opts.Orchestrator.Status,
			Ledger: spendLedger{}, Inbox: n, Notes: n, Handbook: n,
			Now: func() time.Time { return time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC) },
		},
		Root: func() string { return r.root },
	}
}

func TestMeasures_ListedAndReadByIDFromTheCoreOwnBooks(t *testing.T) {
	r := newCoordinatedRig(t, answer("On it."))
	withDeliverables(r)
	c := newClients(serve(t, r.opts), testToken)
	onboard(t, c, r)
	p := unary(t, c.proj.CreateProject, &hearthv1.CreateProjectRequest{Name: "Launch", Brief: launchBrief}).GetProject()

	list := unary(t, c.meas.ListMeasures, &hearthv1.ListMeasuresRequest{}).GetMeasures()
	var found bool
	for _, m := range list {
		found = found || m.GetId() == "project.tasks/"+p.GetId() && m.GetKind() == hearthv1.MeasureKind_MEASURE_KIND_PROGRESS && m.GetAsOf() == nil
	}
	if !found {
		t.Errorf("ListMeasures = %v, want the project's tasks", list)
	}

	got := unary(t, c.meas.ReadMeasures, &hearthv1.ReadMeasuresRequest{Ids: []string{
		"project.tasks/" + p.GetId(), "project.light/" + p.GetId(), "usage.cost/org/2026-10", "inbox.open", "project.tasks/gone",
	}}).GetValues()
	if len(got) != 5 {
		t.Fatalf("ReadMeasures = %v", got)
	}
	if got[0].GetTotal() != 0 || got[0].GetWord() != "No tasks yet" || got[0].GetAsOf() == nil {
		t.Errorf("tasks = %v", got[0])
	}
	if got[1].GetTone() != hearthv1.Tone_TONE_NEUTRAL || got[1].GetKind() != hearthv1.MeasureKind_MEASURE_KIND_LIGHT {
		t.Errorf("light = %v", got[1])
	}
	if got[2].GetNumber() != 2_000_000 || got[2].GetUnit() != "usd_micros" || got[3].GetNumber() != 2 {
		t.Errorf("cost, inbox = %v, %v", got[2], got[3])
	}
	if got[4].GetMissing() != "No such project" {
		t.Errorf("gone = %v, want missing, not an error", got[4])
	}
}

func TestMeasures_RefuseWhatTheyCannotRead(t *testing.T) {
	r := newRig(t)
	withDeliverables(r)
	c := newClients(serve(t, r.opts), testToken)
	many := make([]string, 101)
	for i := range many {
		many[i] = "team.size"
	}
	for name, ids := range map[string][]string{
		"an unknown family": {"cats.count"},
		"an alias":          {"notes.count/me"},
		"too many":          many,
	} {
		err := errOf(t, c.meas.ReadMeasures, &hearthv1.ReadMeasuresRequest{Ids: ids})
		if code(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want invalid_argument", name, err)
		}
	}
}

const mine = `{"version":1,"kind":"report","title":"Mine","template":{"id":"mine","name":"Mine","when":"When I say so."},
"sections":[{"heading":"One","columns":1,"allow":["text"],"hint":"Say it.","widgets":[{"kind":"text","markdown":"x"}]}]}`

func TestTemplates_SavedListedAndDeletedInTheTemplatesFolder(t *testing.T) {
	r := newRig(t)
	withDeliverables(r)
	c := newClients(serve(t, r.opts), testToken)

	// Before the org has a folder, the shipped five list and a save has nowhere to go.
	if os.MkdirAll(r.root, 0o755) != nil {
		t.Fatal("folder")
	}
	list := unary(t, c.deliv.ListDeliverableTemplates, &hearthv1.ListDeliverableTemplatesRequest{}).GetTemplates()
	if len(list) != 5 || !list[0].GetShipped() || list[0].GetJson() != "" {
		t.Fatalf("list = %v, want the shipped five", list)
	}

	saved := unary(t, c.deliv.SaveDeliverableTemplate, &hearthv1.SaveDeliverableTemplateRequest{Json: mine}).GetTemplate()
	path := filepath.Join(r.root, "Templates", "mine.deliverable.json")
	if saved.GetId() != "mine" || saved.GetPath() != path || saved.GetJson() != mine {
		t.Fatalf("Save = %v", saved)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != mine {
		t.Fatalf("file = %q, %v", data, err)
	}
	list = unary(t, c.deliv.ListDeliverableTemplates, &hearthv1.ListDeliverableTemplatesRequest{}).GetTemplates()
	if len(list) != 6 || list[5].GetId() != "mine" || list[5].GetShipped() {
		t.Fatalf("list after save = %v", list)
	}

	// A file that cannot be used is listed with its problem.
	if err := os.WriteFile(filepath.Join(r.root, "Templates", "bad.deliverable.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	list = unary(t, c.deliv.ListDeliverableTemplates, &hearthv1.ListDeliverableTemplatesRequest{}).GetTemplates()
	if last := list[len(list)-1]; last.GetId() != "bad" || last.GetProblem() == "" {
		t.Errorf("a bad file = %v", last)
	}

	for name, req := range map[string]*hearthv1.SaveDeliverableTemplateRequest{
		"not json":         {Json: "nope"},
		"a shipped id":     {Json: strings.Replace(mine, `"id":"mine"`, `"id":"report"`, 1)},
		"an id with a dot": {Json: strings.Replace(mine, `"id":"mine"`, `"id":"../x"`, 1)},
	} {
		if err := errOf(t, c.deliv.SaveDeliverableTemplate, req); code(err) != connect.CodeInvalidArgument {
			t.Errorf("Save %s: %v, want invalid_argument", name, err)
		}
	}

	for name, tt := range map[string]struct {
		id   string
		want connect.Code
	}{
		"shipped":  {"report", connect.CodeInvalidArgument},
		"a path":   {"../hearth", connect.CodeInvalidArgument},
		"missing":  {"nothing", connect.CodeNotFound},
		"an empty": {"", connect.CodeInvalidArgument},
	} {
		if err := errOf(t, c.deliv.DeleteDeliverableTemplate, &hearthv1.DeleteDeliverableTemplateRequest{Id: tt.id}); code(err) != tt.want {
			t.Errorf("Delete %s: %v, want %v", name, err, tt.want)
		}
	}
	unary(t, c.deliv.DeleteDeliverableTemplate, &hearthv1.DeleteDeliverableTemplateRequest{Id: "mine"})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the file is still there: %v", err)
	}
}

func TestTemplates_ALinkIsNeverFollowedOrRemoved(t *testing.T) {
	r := newRig(t)
	withDeliverables(r)
	c := newClients(serve(t, r.opts), testToken)
	outside := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(r.root, "Templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "mine.deliverable.json")); err != nil {
		t.Fatal(err)
	}
	if err := errOf(t, c.deliv.SaveDeliverableTemplate, &hearthv1.SaveDeliverableTemplateRequest{Json: mine}); err == nil {
		t.Error("Save wrote through a link")
	}
	if err := errOf(t, c.deliv.DeleteDeliverableTemplate, &hearthv1.DeleteDeliverableTemplateRequest{Id: "mine"}); err == nil {
		t.Error("Delete removed a link")
	}
	if data, _ := os.ReadFile(outside); string(data) != "keep" {
		t.Errorf("the file outside was changed to %q", data)
	}
}

func TestTemplates_WithoutAHearthFolderASaveHasNowhereToGo(t *testing.T) {
	r := newRig(t)
	withDeliverables(r)
	r.opts.Deliverables.Root = func() string { return "" }
	c := newClients(serve(t, r.opts), testToken)
	if got := len(unary(t, c.deliv.ListDeliverableTemplates, &hearthv1.ListDeliverableTemplatesRequest{}).GetTemplates()); got != 5 {
		t.Errorf("list = %d, want the shipped five", got)
	}
	if err := errOf(t, c.deliv.SaveDeliverableTemplate, &hearthv1.SaveDeliverableTemplateRequest{Json: mine}); code(err) != connect.CodeFailedPrecondition {
		t.Errorf("Save = %v, want failed_precondition", err)
	}
}
