package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/rpc"
	"github.com/rfizzle/hearth/internal/store"
)

func TestDrafter_ANoModelIsUnavailableAndAReplyIsAProposal(t *testing.T) {
	none := func(org.Tier) (agent.Stream, error) { return nil, errors.New("no account is signed in") }
	_, err := drafter(nil, none, nil)(t.Context(), rpc.DraftRequest{Description: "help", Tier: org.TierFast})
	if !errors.Is(err, rpc.ErrUnavailable) || !strings.Contains(err.Error(), "no account is signed in") {
		t.Errorf("err = %v, want unavailable in the owner's words", err)
	}

	say := func(org.Tier) (agent.Stream, error) {
		return func(context.Context, []llm.Message, []llm.Tool, string) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 2)
			ch <- llm.StreamEvent{Token: `{"name":"Wren","role":"Analyst","job_description":"Read.","tier":"best"}`}
			ch <- llm.StreamEvent{Done: true}
			close(ch)
			return ch, nil
		}, nil
	}
	got, err := drafter(nil, say, nil)(t.Context(), rpc.DraftRequest{Description: "help", Tier: org.TierFast})
	if err != nil || got.Name != "Wren" || got.Tier != org.TierBest {
		t.Errorf("draft = %+v, %v", got, err)
	}
}

func TestOpenCore_GivesOlderBotsTheRightToAsk(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Reads.", org.TierFast, filepath.Join(dir, "Hearth"))
	var older []org.Grant
	for _, g := range grants {
		if g.Capability != org.AskOwner {
			older = append(older, g)
		}
	}
	bot, err = db.CreateBot(t.Context(), bot, older, dm)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, dir)
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	got, err := opts.Store.GrantsFor(ctx, bot.ID)
	if err != nil || org.Allowed(got, org.AskOwner) != org.ModeAllowed {
		t.Errorf("grants after open = %v, %v, want the right to ask", got, err)
	}
}

func TestOpenCore_TheDataFolderIsReservedForGrantsAndTools(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, dir)
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if len(opts.ReservedFolders) == 0 || opts.ReservedFolders[0] != dir {
		t.Errorf("ReservedFolders = %v, want the data folder", opts.ReservedFolders)
	}
	if got := reservedFolders(filepath.Join(dir, "x", "..")); got[0] != dir {
		t.Errorf("reservedFolders = %v, want the cleaned path", got)
	}
}
