package after

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
)

func input(t *testing.T, streams func(org.Bot) (agent.Stream, error), logged *bytes.Buffer) Input {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return Input{
		Store: db, Bus: bus.New(), Log: log.New(logged, "", 0), Streams: streams,
		Run:     org.Run{ID: "r1", ConversationID: "c1"},
		Reserve: func(string) error { return nil }, Release: func(string) {},
	}
}

func TestRun_ASummaryThatFailsIsLoggedNotRaised(t *testing.T) {
	var logged bytes.Buffer
	in := input(t, func(org.Bot) (agent.Stream, error) { return nil, errors.New("no model") }, &logged)
	var w Worker
	w.Run(context.Background(), in, org.Bot{ID: "b1", Name: "Dana"}, assembly.Output{Behind: true, Until: 5})
	if got := logged.String(); !strings.Contains(got, "conversation c1 was not summarised: no model") {
		t.Errorf("log = %q", got)
	}
}

func TestRun_NothingBehindAsksNoModel(t *testing.T) {
	var logged bytes.Buffer
	var asked atomic.Int32
	in := input(t, func(org.Bot) (agent.Stream, error) { asked.Add(1); return nil, errors.New("no model") }, &logged)
	var w Worker
	w.Run(context.Background(), in, org.Bot{ID: "b1", Name: "Dana"}, assembly.Output{})
	if asked.Load() != 0 || logged.Len() != 0 {
		t.Errorf("asked %d times, logged %q; want neither", asked.Load(), logged.String())
	}
}

func TestRun_OneSummaryAtATimePerConversation(t *testing.T) {
	var logged bytes.Buffer
	var w Worker
	var in Input
	inner := make(chan struct{})
	in = input(t, func(org.Bot) (agent.Stream, error) {
		// A second run ends while the first is writing: it must not start one.
		w.Run(context.Background(), in, org.Bot{ID: "b1"}, assembly.Output{Behind: true})
		close(inner)
		return nil, errors.New("no model")
	}, &logged)
	w.Run(context.Background(), in, org.Bot{ID: "b1"}, assembly.Output{Behind: true})
	<-inner
	if n := strings.Count(logged.String(), "was not summarised"); n != 1 {
		t.Errorf("summaries tried = %d, want 1:\n%s", n, logged.String())
	}
}
