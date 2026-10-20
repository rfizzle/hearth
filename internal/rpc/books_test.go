package rpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/models"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The calls books serves, end to end over the in-memory listener: the
// owner's edits to notes, pages, budgets, tiers and a bot reach the domain
// and come back, with their events.

// errOf makes one call and keeps only its error.
func errOf[Req, Res any](t *testing.T, f func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), msg *Req) error {
	t.Helper()
	_, err := f(t.Context(), connect.NewRequest(msg))
	return err
}

// lastOf reads events until one satisfies has, and returns it.
func lastOf(f *follower, has func(*hearthv1.SubscribeResponse) bool) *hearthv1.SubscribeResponse {
	ev := f.until(has)
	return ev[len(ev)-1]
}

func noteChanged(m *hearthv1.SubscribeResponse) bool { return m.GetNoteChanged() != nil }

func TestNotes_ListedEditedPinnedDeletedWithEvents(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	for _, text := range []string{"Prefers short answers.", "Lives in Leeds."} {
		if _, err := r.opts.Store.AddNote(t.Context(), memory.Note{BotID: bot.GetId(), Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	f := follow(t, c, r.bus)

	list := unary(t, c.bot.ListNotes, &hearthv1.ListNotesRequest{BotId: bot.GetId()})
	if len(list.GetNotes()) != 2 || list.GetNotes()[0].GetText() != "Prefers short answers." || list.GetCap() != memory.Cap {
		t.Fatalf("ListNotes = %v", list)
	}
	first, second := list.GetNotes()[0], list.GetNotes()[1]

	edited := unary(t, c.bot.EditNote, &hearthv1.EditNoteRequest{Id: first.GetId(), Text: "Prefers one line."}).GetNote()
	if edited.GetText() != "Prefers one line." || edited.GetBotId() != bot.GetId() {
		t.Errorf("EditNote = %v", edited)
	}
	if got := lastOf(f, noteChanged).GetNoteChanged(); got.GetNote().GetText() != "Prefers one line." || got.GetBotId() != bot.GetId() || got.GetDeleted() {
		t.Errorf("note_changed after the edit = %v", got)
	}

	if pinned := unary(t, c.bot.PinNote, &hearthv1.PinNoteRequest{Id: second.GetId(), Pinned: true}).GetNote(); !pinned.GetPinned() {
		t.Errorf("PinNote = %v, want pinned", pinned)
	}
	if got := lastOf(f, noteChanged).GetNoteChanged(); !got.GetNote().GetPinned() || got.GetNote().GetId() != second.GetId() {
		t.Errorf("note_changed after the pin = %v", got)
	}

	unary(t, c.bot.DeleteNote, &hearthv1.DeleteNoteRequest{Id: first.GetId()})
	if got := lastOf(f, noteChanged).GetNoteChanged(); !got.GetDeleted() || got.GetNote().GetId() != first.GetId() || got.GetBotId() != bot.GetId() {
		t.Errorf("note_changed after the delete = %v", got)
	}
	if left := unary(t, c.bot.ListNotes, &hearthv1.ListNotesRequest{BotId: bot.GetId()}).GetNotes(); len(left) != 1 || left[0].GetId() != second.GetId() {
		t.Errorf("ListNotes after the delete = %v", left)
	}

	refusals := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"edit with no text", errOf(t, c.bot.EditNote, &hearthv1.EditNoteRequest{Id: second.GetId()}), connect.CodeInvalidArgument},
		{"edit to a control character", errOf(t, c.bot.EditNote, &hearthv1.EditNoteRequest{Id: second.GetId(), Text: "a\x00b"}), connect.CodeInvalidArgument},
		{"pin no note", errOf(t, c.bot.PinNote, &hearthv1.PinNoteRequest{}), connect.CodeInvalidArgument},
		{"pin a note that is not kept", errOf(t, c.bot.PinNote, &hearthv1.PinNoteRequest{Id: "nope"}), connect.CodeNotFound},
		{"delete no note", errOf(t, c.bot.DeleteNote, &hearthv1.DeleteNoteRequest{}), connect.CodeInvalidArgument},
		{"delete a note that is not kept", errOf(t, c.bot.DeleteNote, &hearthv1.DeleteNoteRequest{Id: "nope"}), connect.CodeNotFound},
		{"list for no bot", errOf(t, c.bot.ListNotes, &hearthv1.ListNotesRequest{}), connect.CodeInvalidArgument},
	}
	for _, tt := range refusals {
		if code(tt.err) != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.err, tt.want)
		}
	}
}

func TestHandbook_PagesWithTheMeterAndTheRefusals(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)

	everyone := &hearthv1.Scope{Everyone: true}
	put := unary(t, c.book.PutPage, &hearthv1.PutPageRequest{Title: "Tone", Markdown: "Plain words.", Scope: everyone})
	page := put.GetPage()
	if page.GetId() == "" || page.GetBytes() != int64(len("Plain words.")) || !page.GetScope().GetEveryone() {
		t.Fatalf("PutPage = %v", page)
	}
	if put.GetMeter().GetUsedBytes() != page.GetBytes() || put.GetMeter().GetCapBytes() != memory.HandbookCap {
		t.Errorf("PutPage meter = %v", put.GetMeter())
	}
	if got := lastOf(f, func(m *hearthv1.SubscribeResponse) bool { return m.GetHandbookChanged() != nil }).GetHandbookChanged(); got.GetPage().GetId() != page.GetId() || got.GetDeleted() {
		t.Errorf("handbook_changed = %v", got)
	}
	mine := unary(t, c.book.PutPage, &hearthv1.PutPageRequest{Title: "Dana's", Markdown: "Only Dana.", Scope: &hearthv1.Scope{BotId: bot.GetId()}})
	if mine.GetMeter().GetUsedBytes() != page.GetBytes()+mine.GetPage().GetBytes() {
		t.Errorf("a bot page's meter = %v, want everyone's and the bot's", mine.GetMeter())
	}

	list := unary(t, c.book.ListPages, &hearthv1.ListPagesRequest{BotId: bot.GetId()})
	if len(list.GetPages()) != 2 || list.GetPages()[0].GetId() != page.GetId() {
		t.Errorf("ListPages for the bot = %v", list.GetPages())
	}
	if all := unary(t, c.book.ListPages, &hearthv1.ListPagesRequest{}); len(all.GetPages()) != 1 {
		t.Errorf("ListPages for everyone = %v", all.GetPages())
	}

	refusals := []struct {
		name string
		req  *hearthv1.PutPageRequest
	}{
		{"over the cap", &hearthv1.PutPageRequest{Title: "Long", Markdown: strings.Repeat("x", memory.HandbookCap), Scope: everyone}},
		{"no title", &hearthv1.PutPageRequest{Markdown: "x", Scope: everyone}},
		{"a control character in the title", &hearthv1.PutPageRequest{Title: "a\x07b", Markdown: "x", Scope: everyone}},
		{"a line break in the title", &hearthv1.PutPageRequest{Title: "a\nb", Markdown: "x", Scope: everyone}},
		{"no scope", &hearthv1.PutPageRequest{Title: "Lost", Markdown: "x"}},
	}
	for _, tt := range refusals {
		if err := errOf(t, c.book.PutPage, tt.req); code(err) != connect.CodeInvalidArgument {
			t.Errorf("PutPage %s = %v, want invalid_argument", tt.name, err)
		}
	}

	del := unary(t, c.book.DeletePage, &hearthv1.DeletePageRequest{Id: page.GetId()})
	if del.GetMeter().GetUsedBytes() != 0 {
		t.Errorf("DeletePage meter = %v, want everyone's empty", del.GetMeter())
	}
	if got := lastOf(f, func(m *hearthv1.SubscribeResponse) bool { return m.GetHandbookChanged().GetDeleted() }).GetHandbookChanged(); got.GetPage().GetId() != page.GetId() {
		t.Errorf("handbook_changed after the delete = %v", got)
	}
	if err := errOf(t, c.book.DeletePage, &hearthv1.DeletePageRequest{Id: page.GetId()}); code(err) != connect.CodeNotFound {
		t.Errorf("DeletePage twice = %v, want not_found", err)
	}
}

func TestUsage_SummaryBudgetsResumeAndTheTick(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	ctx := t.Context()
	sept := time.Date(2026, 9, 14, 12, 0, 0, 0, time.Local)
	for range 2 {
		rec := usage.Record{BotID: bot.GetId(), Provider: "anthropic", Model: "unpriced", Prompt: 100, Completion: 20, Cached: 5, At: sept}
		if err := r.opts.Store.AddUsage(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	sum := unary(t, c.usage.Summary, &hearthv1.SummaryRequest{Month: "2026-09"})
	if sum.GetMonth() != "2026-09" || sum.GetOrg().GetRequests() != 2 || sum.GetOrg().GetPromptTokens() != 200 || sum.GetOrg().GetPriced() {
		t.Fatalf("Summary = %v", sum)
	}
	if len(sum.GetBots()) != 1 || sum.GetBots()[0].GetBotId() != bot.GetId() || sum.GetBots()[0].GetTotals().GetCompletionTokens() != 40 {
		t.Errorf("Summary bots = %v", sum.GetBots())
	}
	if len(sum.GetMonths()) == 0 || sum.GetMonths()[0] != "2026-09" {
		t.Errorf("Summary months = %v", sum.GetMonths())
	}

	set := unary(t, c.usage.SetBudgets, &hearthv1.SetBudgetsRequest{OrgCents: 5000, Bots: []*hearthv1.BotBudget{{BotId: bot.GetId(), Cents: 2000}}}).GetSummary()
	if set.GetOrgBudgetCents() != 5000 || len(set.GetBots()) != 1 || set.GetBots()[0].GetBudgetCents() != 2000 || set.GetMonth() != usage.Month(time.Now()) {
		t.Errorf("SetBudgets = %v", set)
	}
	if err := errOf(t, c.usage.SetBudgets, &hearthv1.SetBudgetsRequest{Bots: []*hearthv1.BotBudget{{Cents: 1}}}); code(err) != connect.CodeInvalidArgument {
		t.Errorf("a budget for no bot = %v, want invalid_argument", err)
	}
	if res := unary(t, c.usage.Resume, &hearthv1.ResumeRequest{}).GetSummary(); res.GetOrgPaused() {
		t.Errorf("Resume = %v, want the org working", res)
	}
	unary(t, c.usage.Resume, &hearthv1.ResumeRequest{BotId: bot.GetId()})

	f := follow(t, c, r.bus)
	if err := r.meter.Record(ctx, usage.Record{BotID: bot.GetId(), Provider: "anthropic", Model: "unpriced", Prompt: 7}); err != nil {
		t.Fatal(err)
	}
	tick := lastOf(f, func(m *hearthv1.SubscribeResponse) bool { return m.GetUsageTick() != nil }).GetUsageTick()
	if tick.GetBot().GetBotId() != bot.GetId() || tick.GetBot().GetBudgetCents() != 2000 || tick.GetOrg().GetPromptTokens() != 7 || tick.GetMonth() != usage.Month(time.Now()) {
		t.Errorf("usage_tick = %v", tick)
	}
}

func TestTiers_SetThenReadWithTheModelsOnOffer(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Key: "sk-ant-test"})
	unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{
		Provider: hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE, DisplayName: "Ollama", BaseUrl: "http://localhost:11434/v1", DefaultModel: "llama3.2",
	})
	f := follow(t, c, r.bus)

	choices := []*hearthv1.TierChoice{
		{Tier: hearthv1.Tier_TIER_FAST, Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Model: "claude-haiku"},
		{Tier: hearthv1.Tier_TIER_BEST, Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Model: "claude-opus"},
	}
	set := unary(t, c.prov.SetTiers, &hearthv1.SetTiersRequest{Tiers: choices}).GetTiers()
	if len(set) != 2 || set[1].GetModel() != "claude-opus" || set[1].GetProvider() != hearthv1.Provider_PROVIDER_ANTHROPIC {
		t.Fatalf("SetTiers = %v", set)
	}
	if got := lastOf(f, func(m *hearthv1.SubscribeResponse) bool { return m.GetTiersChanged() != nil }).GetTiersChanged(); len(got.GetTiers()) != 2 {
		t.Errorf("tiers_changed = %v", got)
	}

	tiers := unary(t, c.prov.Tiers, &hearthv1.TiersRequest{})
	if len(tiers.GetTiers()) != 2 || tiers.GetTiers()[0].GetModel() != "claude-haiku" {
		t.Errorf("Tiers = %v", tiers.GetTiers())
	}
	var anthropic, local bool
	for _, o := range tiers.GetOptions() {
		anthropic = anthropic || (o.GetProvider() == hearthv1.Provider_PROVIDER_ANTHROPIC && o.GetLabel() != "")
		local = local || o.GetModel() == "acme-large"
	}
	if !anthropic || !local {
		t.Errorf("Tiers options = %v, want the signed-in provider's and the detected runtime's", tiers.GetOptions())
	}

	for name, bad := range map[string]*hearthv1.TierChoice{
		"no tier":     {Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Model: "m"},
		"no provider": {Tier: hearthv1.Tier_TIER_FAST, Model: "m"},
		"no model":    {Tier: hearthv1.Tier_TIER_FAST, Provider: hearthv1.Provider_PROVIDER_ANTHROPIC},
	} {
		if err := errOf(t, c.prov.SetTiers, &hearthv1.SetTiersRequest{Tiers: []*hearthv1.TierChoice{bad}}); code(err) != connect.CodeInvalidArgument {
			t.Errorf("SetTiers with %s = %v, want invalid_argument", name, err)
		}
	}
	twice := &hearthv1.SetTiersRequest{Tiers: []*hearthv1.TierChoice{choices[0], choices[0]}}
	if err := errOf(t, c.prov.SetTiers, twice); code(err) != connect.CodeInvalidArgument {
		t.Errorf("SetTiers naming a tier twice = %v, want invalid_argument", err)
	}
}

func TestListKnownServices_FillsDetection(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	for _, s := range unary(t, c.prov.ListKnownServices, &hearthv1.ListKnownServicesRequest{}).GetServices() {
		local := s.GetKind() == hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_LOCAL
		if local != s.GetDetected() || (local && (len(s.GetModels()) != 1 || s.GetModels()[0] != "acme-large")) {
			t.Errorf("%s: detected %v with %v; a runtime answering on this Mac is detected with its models", s.GetId(), s.GetDetected(), s.GetModels())
		}
	}
}

func TestUpdateBot_EditsWithAnOverride(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)

	pin := models.Encode(provider.Pick{Provider: provider.Anthropic, Model: "claude-opus"})
	got := unary(t, c.bot.UpdateBot, &hearthv1.UpdateBotRequest{Id: bot.GetId(), Role: "Researcher", ModelOverride: pin}).GetBot()
	if got.GetRole() != "Researcher" || got.GetModelOverride() != pin || got.GetName() != "Dana" || got.GetTier() != bot.GetTier() || got.GetJobDescription() != bot.GetJobDescription() {
		t.Errorf("UpdateBot = %v, want the role and the override changed and the rest kept", got)
	}
	if ev := lastOf(f, func(m *hearthv1.SubscribeResponse) bool { return m.GetBotChanged() != nil }).GetBotChanged().GetBot(); ev.GetModelOverride() != pin {
		t.Errorf("bot_changed = %v", ev)
	}
	if stored := unary(t, c.bot.GetBot, &hearthv1.GetBotRequest{Id: bot.GetId()}).GetBot(); stored.GetModelOverride() != pin {
		t.Errorf("GetBot after the update = %v", stored)
	}

	cleared := unary(t, c.bot.UpdateBot, &hearthv1.UpdateBotRequest{Id: bot.GetId(), Name: "Dana B", Tier: hearthv1.Tier_TIER_BEST, ClearModelOverride: true}).GetBot()
	if cleared.GetModelOverride() != "" || cleared.GetName() != "Dana B" || cleared.GetTier() != hearthv1.Tier_TIER_BEST || cleared.GetRole() != "Researcher" {
		t.Errorf("UpdateBot clearing the override = %v", cleared)
	}

	refusals := []struct {
		name string
		req  *hearthv1.UpdateBotRequest
		want connect.Code
	}{
		{"no id", &hearthv1.UpdateBotRequest{Name: "X"}, connect.CodeInvalidArgument},
		{"a bot that is not hired", &hearthv1.UpdateBotRequest{Id: "nope"}, connect.CodeNotFound},
		{"the Inbox's name", &hearthv1.UpdateBotRequest{Id: bot.GetId(), Name: "inbox"}, connect.CodeInvalidArgument},
		{"an override that names no model", &hearthv1.UpdateBotRequest{Id: bot.GetId(), ModelOverride: "claude"}, connect.CodeInvalidArgument},
		{"a tier hearth does not have", &hearthv1.UpdateBotRequest{Id: bot.GetId(), Tier: hearthv1.Tier(9)}, connect.CodeInvalidArgument},
	}
	for _, tt := range refusals {
		if err := errOf(t, c.bot.UpdateBot, tt.req); code(err) != tt.want {
			t.Errorf("UpdateBot with %s = %v, want %v", tt.name, err, tt.want)
		}
	}
}

// The credential handshake's wiring: a key the core held in its files before
// the app synced comes back to be stored, and the next sync holding it ends
// the files' part.
func TestSyncCredentials_RoundTrip(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{Provider: hearthv1.Provider_PROVIDER_GEMINI, Key: "gm-test-key"})

	first := unary(t, c.prov.SyncCredentials, &hearthv1.SyncCredentialsRequest{}).GetRecordsToStore()
	if len(first) != 1 || first[0].GetName() != "gemini" || string(first[0].GetValue()) != "gm-test-key" {
		t.Fatalf("records_to_store = %v, want the file's gemini key", first)
	}
	if again := unary(t, c.prov.SyncCredentials, &hearthv1.SyncCredentialsRequest{Records: first}).GetRecordsToStore(); len(again) != 0 {
		t.Errorf("the second sync = %v, want nothing more to store", again)
	}
	exp := unary(t, c.prov.ExportCredential, &hearthv1.ExportCredentialRequest{Name: "gemini"}).GetRecord()
	if string(exp.GetValue()) != "gm-test-key" {
		t.Errorf("ExportCredential after the sync = %q", exp.GetValue())
	}
}

func TestEditNote_DropsInvisibleCharacters(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	n, err := r.opts.Store.AddNote(t.Context(), memory.Note{BotID: bot.GetId(), Text: "Lives in Leeds."})
	if err != nil {
		t.Fatal(err)
	}
	first := &hearthv1.Note{Id: n.ID}
	for _, tc := range []struct{ name, in, want string }{
		{"bidi override", "a\u202eb", "ab"},
		{"zero-width space", "a\u200bb", "ab"},
		{"tag character", "a\U000E0041b", "ab"},
		{"variation selector", "a\ufe00b", "ab"},
		{"emoji presentation kept", "ok \u2764\ufe0f", "ok \u2764\ufe0f"},
	} {
		if got := unary(t, c.bot.EditNote, &hearthv1.EditNoteRequest{Id: first.GetId(), Text: tc.in}).GetNote().GetText(); got != tc.want {
			t.Errorf("EditNote %s = %q, want %q", tc.name, got, tc.want)
		}
	}
	if err := errOf(t, c.bot.EditNote, &hearthv1.EditNoteRequest{Id: first.GetId(), Text: "\u200b\u202e"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("EditNote of invisible characters alone = %v, want invalid_argument", err)
	}
}
