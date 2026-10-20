package rpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func code(err error) connect.Code { return connect.CodeOf(err) }

func unary[Req, Res any](t *testing.T, call func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), msg *Req) *Res {
	t.Helper()
	res, err := call(t.Context(), connect.NewRequest(msg))
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	return res.Msg
}

// checkOnboardingFlow is the owner's first minutes: name the org, hire a
// bot, send it a message and watch the answer arrive on the event stream.
// The in-memory and the loopback tests both run it.
func checkOnboardingFlow(t *testing.T, c clients, r *rig) {
	t.Helper()
	ctx := t.Context()

	if got, err := c.org.GetOrg(ctx, connect.NewRequest(&hearthv1.GetOrgRequest{})); err != nil || got.Msg.GetOrg() != nil {
		t.Fatalf("GetOrg before onboarding = %v, %v, want an empty answer", got, err)
	}
	o := unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root}).GetOrg()
	if o.GetName() != "Acme" || o.GetWorkspacePath() != r.root {
		t.Fatalf("UpdateOrg = %v", o)
	}
	if fi, err := os.Stat(filepath.Join(r.root, "Inbox")); err != nil || !fi.IsDir() {
		t.Fatalf("UpdateOrg did not create the Hearth folder: %v", err)
	}
	if got := unary(t, c.org.GetOrg, &hearthv1.GetOrgRequest{}).GetOrg(); got.GetName() != "Acme" {
		t.Fatalf("GetOrg = %v", got)
	}
	office := unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots()
	if len(office) != 1 || office[0].GetName() != "Chief of Staff" || office[0].GetScope() != hearthv1.BotScope_BOT_SCOPE_ORG ||
		office[0].GetDmConversationId() == "" || office[0].GetFolderPath() != filepath.Join(r.root, "Chief of Staff") {
		t.Fatalf("after onboarding ListBots = %v, want the Chief of Staff at the org level with its DM", office)
	}
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	if again := unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots(); len(again) != 1 || again[0].GetId() != office[0].GetId() {
		t.Fatalf("a second UpdateOrg left %v, want the one Chief of Staff", again)
	}

	tpls := unary(t, c.bot.ListTemplates, &hearthv1.ListTemplatesRequest{}).GetTemplates()
	if len(tpls) != 3 || tpls[0].GetJobDescription() == "" || len(tpls[0].GetIntroduction().GetHow()) != 3 || len(tpls[0].GetIntroduction().GetTryAsking()) != 3 {
		t.Fatalf("ListTemplates = %v, want three, each with its introduction", tpls)
	}
	if in := office[0].GetIntroduction(); !strings.HasPrefix(in.GetPurpose(), "I run your office.") || len(in.GetHow()) != 3 || len(in.GetTryAsking()) != 3 {
		t.Fatalf("the Chief of Staff's introduction = %v, want the template's", in)
	}

	f := follow(t, c, r.bus)
	bot := unary(t, c.bot.CreateBot, &hearthv1.CreateBotRequest{
		Name: "Dana", Role: tpls[0].GetRole(), JobDescription: tpls[0].GetJobDescription(), Tier: tpls[0].GetSuggestedTier(),
		TemplateId: tpls[0].GetId(),
	}).GetBot()
	if bot.GetIntroduction().GetPurpose() != tpls[0].GetIntroduction().GetPurpose() || bot.GetIntroduction().GetPurpose() == "" {
		t.Fatalf("CreateBot introduction = %v, want the template's %v", bot.GetIntroduction(), tpls[0].GetIntroduction())
	}
	if bot.GetId() == "" || bot.GetDmConversationId() == "" || bot.GetStatus() != hearthv1.BotStatus_BOT_STATUS_IDLE ||
		bot.GetFolderPath() != filepath.Join(r.root, "Dana") {
		t.Fatalf("CreateBot = %v", bot)
	}
	hired := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetBotChanged() != nil })
	if got := hired[len(hired)-1].GetBotChanged().GetBot(); got.GetId() != bot.GetId() {
		t.Fatalf("bot_changed = %v, want the hired bot", got)
	}
	if bots := unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots(); len(bots) != 2 || bots[1].GetId() != bot.GetId() ||
		bots[1].GetScope() != hearthv1.BotScope_BOT_SCOPE_TEAM {
		t.Fatalf("ListBots = %v, want the Chief of Staff, then Dana on the team", bots)
	}
	if got := unary(t, c.bot.GetBot, &hearthv1.GetBotRequest{Id: bot.GetId()}).GetBot(); got.GetName() != "Dana" {
		t.Fatalf("GetBot = %v", got)
	}
	convs := unary(t, c.conv.ListConversations, &hearthv1.ListConversationsRequest{BotId: bot.GetId()}).GetConversations()
	if len(convs) != 1 || convs[0].GetId() != bot.GetDmConversationId() || convs[0].GetKind() != hearthv1.ConversationKind_CONVERSATION_KIND_DM {
		t.Fatalf("ListConversations = %v", convs)
	}

	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "Hello Dana"})
	if sent.GetMessage().GetContent() != "Hello Dana" || sent.GetRun().GetId() == "" {
		t.Fatalf("Send = %v", sent)
	}
	evs := f.until(runEnded)

	var sawOwner, sawDelta, sawFinal bool
	var last uint64
	for _, e := range evs {
		if e.GetSequence() <= last {
			t.Errorf("sequence %d follows %d", e.GetSequence(), last)
		}
		last = e.GetSequence()
		if m := e.GetMessageUpserted().GetMessage(); m != nil {
			switch {
			case m.GetAuthorBotId() == "" && m.GetContent() == "Hello Dana":
				sawOwner = true
			case m.GetAuthorBotId() == bot.GetId() && !m.GetStreaming() && m.GetContent() == "Hi there.":
				sawFinal = true
			}
		}
		if e.GetMessageDelta().GetText() != "" {
			sawDelta = true
		}
	}
	if !sawOwner || !sawDelta || !sawFinal {
		t.Errorf("owner message %v, delta %v, final message %v; events %v", sawOwner, sawDelta, sawFinal, evs)
	}
	if end := evs[len(evs)-1].GetRunChanged().GetRun(); end.GetState() != hearthv1.RunState_RUN_STATE_DONE || end.GetId() != sent.GetRun().GetId() {
		t.Errorf("the stream ended on %v, want the run done", end)
	}

	page := unary(t, c.conv.ListMessages, &hearthv1.ListMessagesRequest{ConversationId: bot.GetDmConversationId()})
	if len(page.GetMessages()) != 2 || page.GetHasMore() {
		t.Errorf("ListMessages = %d messages, has_more %v, want the owner's and the bot's", len(page.GetMessages()), page.GetHasMore())
	}
}

func TestOnboardingThenAnAnswer(t *testing.T) {
	r := newRig(t, answer("Hi ", "there."))
	checkOnboardingFlow(t, newClients(serve(t, r.opts), testToken), r)
}

func TestSend_DuringARunIsRefusedAndCancelEndsIt(t *testing.T) {
	r := newRig(t, hang("working"))
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	bot := unary(t, c.bot.CreateBot, &hearthv1.CreateBotRequest{Name: "Dana", JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST}).GetBot()

	f := follow(t, c, r.bus)
	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "one"})
	f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetMessageDelta() != nil })

	_, err := c.conv.Send(t.Context(), connect.NewRequest(&hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "two"}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Send during a run = %v, want failed_precondition", err)
	}

	unary(t, c.conv.Cancel, &hearthv1.CancelRequest{RunId: sent.GetRun().GetId()})
	evs := f.until(runEnded)
	if got := evs[len(evs)-1].GetRunChanged().GetRun().GetState(); got != hearthv1.RunState_RUN_STATE_CANCELLED {
		t.Errorf("run ended %v, want cancelled", got)
	}
	if _, err := c.conv.Cancel(t.Context(), connect.NewRequest(&hearthv1.CancelRequest{RunId: sent.GetRun().GetId()})); code(err) != connect.CodeNotFound {
		t.Errorf("Cancel of an ended run = %v, want not_found", err)
	}
}

func TestErrorsCarryTheCodeTheAppReads(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	ctx := t.Context()
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	unary(t, c.bot.CreateBot, &hearthv1.CreateBotRequest{Name: "Dana", JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST})

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"cancel of an unknown run", func() error {
			_, err := c.conv.Cancel(ctx, connect.NewRequest(&hearthv1.CancelRequest{RunId: "nope"}))
			return err
		}, connect.CodeNotFound},
		{"cancel with no run", func() error {
			_, err := c.conv.Cancel(ctx, connect.NewRequest(&hearthv1.CancelRequest{}))
			return err
		}, connect.CodeInvalidArgument},
		{"get an unknown bot", func() error {
			_, err := c.bot.GetBot(ctx, connect.NewRequest(&hearthv1.GetBotRequest{Id: "nope"}))
			return err
		}, connect.CodeNotFound},
		{"send to an unknown conversation", func() error {
			_, err := c.conv.Send(ctx, connect.NewRequest(&hearthv1.SendRequest{ConversationId: "nope", Text: "hi"}))
			return err
		}, connect.CodeNotFound},
		{"send nothing", func() error {
			_, err := c.conv.Send(ctx, connect.NewRequest(&hearthv1.SendRequest{ConversationId: "x", Text: "  "}))
			return err
		}, connect.CodeInvalidArgument},
		{"hire with no name", func() error {
			_, err := c.bot.CreateBot(ctx, connect.NewRequest(&hearthv1.CreateBotRequest{JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST}))
			return err
		}, connect.CodeInvalidArgument},
		{"hire with no tier", func() error {
			_, err := c.bot.CreateBot(ctx, connect.NewRequest(&hearthv1.CreateBotRequest{Name: "Eli", JobDescription: "Help."}))
			return err
		}, connect.CodeInvalidArgument},
		{"hire a bot whose folder would be the Inbox", func() error {
			_, err := c.bot.CreateBot(ctx, connect.NewRequest(&hearthv1.CreateBotRequest{Name: "inbox", JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST}))
			return err
		}, connect.CodeInvalidArgument},
		{"hire into a folder another bot has", func() error {
			_, err := c.bot.CreateBot(ctx, connect.NewRequest(&hearthv1.CreateBotRequest{Name: "dana", JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST}))
			return err
		}, connect.CodeAlreadyExists},
		{"name the org with a relative folder", func() error {
			_, err := c.org.UpdateOrg(ctx, connect.NewRequest(&hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: "Hearth"}))
			return err
		}, connect.CodeInvalidArgument},
		{"a key for ChatGPT", func() error {
			_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{Provider: hearthv1.Provider_PROVIDER_CHATGPT, Key: "k"}))
			return err
		}, connect.CodeInvalidArgument},
		{"a key for no provider", func() error {
			_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{Key: "k"}))
			return err
		}, connect.CodeInvalidArgument},
		{"an empty integration key", func() error {
			_, err := c.integ.SetIntegrationKey(ctx, connect.NewRequest(&hearthv1.SetIntegrationKeyRequest{Integration: hearthv1.Integration_INTEGRATION_BRAVE_SEARCH}))
			return err
		}, connect.CodeInvalidArgument},
		{"an unknown integration", func() error {
			_, err := c.integ.ClearIntegration(ctx, connect.NewRequest(&hearthv1.ClearIntegrationRequest{}))
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := code(tt.call()); got != tt.want {
				t.Errorf("code = %v, want %v", got, tt.want)
			}
		})
	}
	if bots := unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots(); len(bots) != 2 {
		t.Errorf("the refused hires left %d bots, want the Chief of Staff and Dana", len(bots))
	}
}

func TestIntegrationKey_IsConfiguredWithALabelAndNoKey(t *testing.T) {
	const key = "BSA-0123456789-wxyz"
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	f := follow(t, c, r.bus)

	set := unary(t, c.integ.SetIntegrationKey, &hearthv1.SetIntegrationKeyRequest{Integration: hearthv1.Integration_INTEGRATION_BRAVE_SEARCH, Key: key})
	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetIntegrationChanged() != nil })
	changed := evs[len(evs)-1].GetIntegrationChanged().GetIntegration()
	if !changed.GetConfigured() || changed.GetIntegration() != hearthv1.Integration_INTEGRATION_BRAVE_SEARCH {
		t.Errorf("integration_changed = %v", changed)
	}

	list := unary(t, c.integ.ListIntegrations, &hearthv1.ListIntegrationsRequest{})
	if len(list.GetIntegrations()) != 1 {
		t.Fatalf("ListIntegrations = %v", list)
	}
	got := list.GetIntegrations()[0]
	if !got.GetConfigured() || !strings.HasSuffix(got.GetLabel(), "wxyz") || got.GetHelpUrl() == "" {
		t.Errorf("ListIntegrations = %v, want configured with the last four characters", got)
	}
	for _, m := range []proto.Message{set, list, evs[len(evs)-1]} {
		if strings.Contains(prototext.Format(m), key) {
			t.Errorf("a response carries the key: %s", m)
		}
	}

	unary(t, c.integ.ClearIntegration, &hearthv1.ClearIntegrationRequest{Integration: hearthv1.Integration_INTEGRATION_BRAVE_SEARCH})
	f.until(func(m *hearthv1.SubscribeResponse) bool {
		return m.GetIntegrationChanged() != nil && !m.GetIntegrationChanged().GetIntegration().GetConfigured()
	})
	if got := unary(t, c.integ.ListIntegrations, &hearthv1.ListIntegrationsRequest{}).GetIntegrations()[0]; got.GetConfigured() {
		t.Errorf("after ClearIntegration: %v", got)
	}
}

func TestProviderAccounts_KeyReachesTheBusWithoutTheKey(t *testing.T) {
	const key = "sk-ant-0123456789-abcd"
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	f := follow(t, c, r.bus)

	set := unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Key: key})
	if set.GetAccount().GetState() != hearthv1.AccountState_ACCOUNT_STATE_SIGNED_IN {
		t.Errorf("SetAPIKey account = %v, want signed in", set.GetAccount())
	}
	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetAccountChanged() != nil })
	if got := evs[len(evs)-1].GetAccountChanged().GetAccount(); got.GetProvider() != hearthv1.Provider_PROVIDER_ANTHROPIC {
		t.Errorf("account_changed = %v", got)
	}
	accounts := unary(t, c.prov.ListAccounts, &hearthv1.ListAccountsRequest{})
	if len(accounts.GetAccounts()) != 4 {
		t.Errorf("ListAccounts = %v", accounts)
	}
	for _, text := range []string{prototext.Format(set), prototext.Format(accounts), prototext.Format(evs[len(evs)-1])} {
		if strings.Contains(text, key) {
			t.Errorf("a response carries the key: %s", text)
		}
	}

	unary(t, c.prov.ClearCredential, &hearthv1.ClearCredentialRequest{Provider: hearthv1.Provider_PROVIDER_ANTHROPIC})
	f.until(func(m *hearthv1.SubscribeResponse) bool {
		return m.GetAccountChanged().GetAccount().GetState() == hearthv1.AccountState_ACCOUNT_STATE_SIGNED_OUT
	})
}

func TestEveryRPCWithoutTheTokenIsUnauthenticated(t *testing.T) {
	r := newRig(t)
	hc := serve(t, r.opts)
	for _, token := range []string{"", "wrong"} {
		c := newClients(hc, token)
		ctx := t.Context()
		calls := map[string]func() error{
			"GetOrg": func() error { _, err := c.org.GetOrg(ctx, connect.NewRequest(&hearthv1.GetOrgRequest{})); return err },
			"UpdateOrg": func() error {
				_, err := c.org.UpdateOrg(ctx, connect.NewRequest(&hearthv1.UpdateOrgRequest{}))
				return err
			},
			"ListTemplates": func() error {
				_, err := c.bot.ListTemplates(ctx, connect.NewRequest(&hearthv1.ListTemplatesRequest{}))
				return err
			},
			"CreateBot": func() error {
				_, err := c.bot.CreateBot(ctx, connect.NewRequest(&hearthv1.CreateBotRequest{}))
				return err
			},
			"ListBots": func() error {
				_, err := c.bot.ListBots(ctx, connect.NewRequest(&hearthv1.ListBotsRequest{}))
				return err
			},
			"DraftBot": func() error {
				_, err := c.bot.DraftBot(ctx, connect.NewRequest(&hearthv1.DraftBotRequest{}))
				return err
			},
			"GetBot": func() error { _, err := c.bot.GetBot(ctx, connect.NewRequest(&hearthv1.GetBotRequest{})); return err },
			"ListConversations": func() error {
				_, err := c.conv.ListConversations(ctx, connect.NewRequest(&hearthv1.ListConversationsRequest{}))
				return err
			},
			"ListMessages": func() error {
				_, err := c.conv.ListMessages(ctx, connect.NewRequest(&hearthv1.ListMessagesRequest{}))
				return err
			},
			"Send":   func() error { _, err := c.conv.Send(ctx, connect.NewRequest(&hearthv1.SendRequest{})); return err },
			"Cancel": func() error { _, err := c.conv.Cancel(ctx, connect.NewRequest(&hearthv1.CancelRequest{})); return err },
			"ListAccounts": func() error {
				_, err := c.prov.ListAccounts(ctx, connect.NewRequest(&hearthv1.ListAccountsRequest{}))
				return err
			},
			"SetAPIKey": func() error {
				_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{}))
				return err
			},
			"ClearCredential": func() error {
				_, err := c.prov.ClearCredential(ctx, connect.NewRequest(&hearthv1.ClearCredentialRequest{}))
				return err
			},
			"StartChatGPTLogin": func() error {
				_, err := c.prov.StartChatGPTLogin(ctx, connect.NewRequest(&hearthv1.StartChatGPTLoginRequest{}))
				return err
			},
			"ListIntegrations": func() error {
				_, err := c.integ.ListIntegrations(ctx, connect.NewRequest(&hearthv1.ListIntegrationsRequest{}))
				return err
			},
			"SetIntegrationKey": func() error {
				_, err := c.integ.SetIntegrationKey(ctx, connect.NewRequest(&hearthv1.SetIntegrationKeyRequest{}))
				return err
			},
			"ClearIntegration": func() error {
				_, err := c.integ.ClearIntegration(ctx, connect.NewRequest(&hearthv1.ClearIntegrationRequest{}))
				return err
			},
			"ListGrants": func() error {
				_, err := c.bot.ListGrants(ctx, connect.NewRequest(&hearthv1.ListGrantsRequest{}))
				return err
			},
			"SetGrants": func() error {
				_, err := c.bot.SetGrants(ctx, connect.NewRequest(&hearthv1.SetGrantsRequest{}))
				return err
			},
			"GetConversation": func() error {
				_, err := c.conv.GetConversation(ctx, connect.NewRequest(&hearthv1.GetConversationRequest{}))
				return err
			},
			"Retry": func() error { _, err := c.conv.Retry(ctx, connect.NewRequest(&hearthv1.RetryRequest{})); return err },
			"ListInbox": func() error {
				_, err := c.inbox.ListInbox(ctx, connect.NewRequest(&hearthv1.ListInboxRequest{}))
				return err
			},
			"Decide": func() error {
				_, err := c.inbox.Decide(ctx, connect.NewRequest(&hearthv1.DecideRequest{}))
				return err
			},
			"Answer": func() error {
				_, err := c.inbox.Answer(ctx, connect.NewRequest(&hearthv1.AnswerRequest{}))
				return err
			},
			"Acknowledge": func() error {
				_, err := c.inbox.Acknowledge(ctx, connect.NewRequest(&hearthv1.AcknowledgeRequest{}))
				return err
			},
			"Subscribe": func() error {
				s, err := c.event.Subscribe(ctx, connect.NewRequest(&hearthv1.SubscribeRequest{}))
				if err != nil {
					return err
				}
				defer s.Close()
				s.Receive()
				return s.Err()
			},
		}
		for name, call := range calls {
			if got := code(call()); got != connect.CodeUnauthenticated {
				t.Errorf("%s with token %q = %v, want unauthenticated", name, token, got)
			}
		}
	}
}

func TestEnumsThatAreCastNumberAlike(t *testing.T) {
	checks := []struct {
		name      string
		got, want int
	}{
		{"fast", int(hearthv1.Tier_TIER_FAST), int(org.TierFast)},
		{"balanced", int(hearthv1.Tier_TIER_BALANCED), int(org.TierBalanced)},
		{"best", int(hearthv1.Tier_TIER_BEST), int(org.TierBest)},
		{"idle", int(hearthv1.BotStatus_BOT_STATUS_IDLE), int(org.BotIdle)},
		{"bot working", int(hearthv1.BotStatus_BOT_STATUS_WORKING), int(org.BotWorking)},
		{"dm", int(hearthv1.ConversationKind_CONVERSATION_KIND_DM), int(org.ConversationDM)},
		{"text", int(hearthv1.MessageKind_MESSAGE_KIND_TEXT), int(org.MessageText)},
		{"tool call", int(hearthv1.MessageKind_MESSAGE_KIND_TOOL_CALL), int(org.MessageToolCall)},
		{"tool result", int(hearthv1.MessageKind_MESSAGE_KIND_TOOL_RESULT), int(org.MessageToolResult)},
		{"system", int(hearthv1.MessageKind_MESSAGE_KIND_SYSTEM), int(org.MessageSystem)},
		{"question", int(hearthv1.MessageKind_MESSAGE_KIND_QUESTION), int(org.MessageQuestion)},
		{"approval request", int(hearthv1.MessageKind_MESSAGE_KIND_APPROVAL_REQUEST), int(org.MessageApprovalRequest)},
		{"approval decision", int(hearthv1.MessageKind_MESSAGE_KIND_APPROVAL_DECISION), int(org.MessageApprovalDecision)},
		{"waiting owner", int(hearthv1.RunState_RUN_STATE_WAITING_OWNER), int(org.RunWaitingOwner)},
		{"queued", int(hearthv1.RunState_RUN_STATE_QUEUED), int(org.RunQueued)},
		{"run working", int(hearthv1.RunState_RUN_STATE_WORKING), int(org.RunWorking)},
		{"done", int(hearthv1.RunState_RUN_STATE_DONE), int(org.RunDone)},
		{"failed", int(hearthv1.RunState_RUN_STATE_FAILED), int(org.RunFailed)},
		{"cancelled", int(hearthv1.RunState_RUN_STATE_CANCELLED), int(org.RunCancelled)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: contract %d, org %d; a cast needs them alike", c.name, c.got, c.want)
		}
	}
}

func TestUpdateOrg_RefusesWhileNoWayToMakeTheFolderIsSet(t *testing.T) {
	s := &orgService{opts: Options{}}
	_, err := s.UpdateOrg(t.Context(), connect.NewRequest(&hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: "/Hearth"}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("UpdateOrg = %v, want failed_precondition", err)
	}
}
