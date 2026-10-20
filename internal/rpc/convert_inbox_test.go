package rpc

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/widget"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The tables hold every domain value, and every contract value but
// unspecified, and the two directions agree. A value added on either side
// without a row fails here.

func TestEnumTables_EveryDomainValueHasAContractValueAndBack(t *testing.T) {
	// The right to ask the owner is the one capability the contract has no value for.
	for _, c := range []org.Capability{org.FilesRead, org.FilesWrite, org.FilesDelete, org.WebFetch, org.WebSearch} {
		e := toProto(capabilities, c)
		if e == hearthv1.Capability_CAPABILITY_UNSPECIFIED {
			t.Errorf("capability %s has no contract value", c)
		} else if back, err := fromProto(capabilities, e); err != nil || back != c {
			t.Errorf("capability %s -> %v -> %v, %v", c, e, back, err)
		}
	}
	for _, m := range []org.Mode{org.ModeNotGranted, org.ModeAsk, org.ModeAllowed} {
		e := toProto(grantModes, m)
		if e == hearthv1.GrantMode_GRANT_MODE_UNSPECIFIED {
			t.Errorf("mode %d has no contract value", m)
		} else if back, err := fromProto(grantModes, e); err != nil || back != m {
			t.Errorf("mode %d -> %v -> %v, %v", m, e, back, err)
		}
	}
	for _, d := range []inbox.Decision{inbox.AllowOnce, inbox.AllowForTask, inbox.AlwaysAllow, inbox.Deny} {
		e := toProto(decisions, d)
		if e == hearthv1.Decision_DECISION_UNSPECIFIED {
			t.Errorf("decision %d has no contract value", d)
		} else if back, err := fromProto(decisions, e); err != nil || back != d {
			t.Errorf("decision %d -> %v -> %v, %v", d, e, back, err)
		}
	}
	for _, k := range []inbox.Kind{inbox.Approval, inbox.Question, inbox.Failure, inbox.Budget} {
		e := toProto(inboxKinds, k)
		if e == hearthv1.InboxKind_INBOX_KIND_UNSPECIFIED {
			t.Errorf("kind %d has no contract value", k)
		} else if back, err := fromProto(inboxKinds, e); err != nil || back != k {
			t.Errorf("kind %d -> %v -> %v, %v", k, e, back, err)
		}
	}
	for _, s := range []inbox.State{inbox.Open, inbox.Closed} {
		e := toProto(inboxStates, s)
		if e == hearthv1.InboxState_INBOX_STATE_UNSPECIFIED {
			t.Errorf("state %d has no domain value", s)
		} else if back, err := fromProto(inboxStates, e); err != nil || back != s {
			t.Errorf("state %d -> %v -> %v, %v", s, e, back, err)
		}
	}
}

func TestEnumTables_EveryContractValueHasADomainValue(t *testing.T) {
	for v := range hearthv1.Capability_name {
		if e := hearthv1.Capability(v); e != hearthv1.Capability_CAPABILITY_UNSPECIFIED {
			if _, ok := capabilities[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.GrantMode_name {
		if e := hearthv1.GrantMode(v); e != hearthv1.GrantMode_GRANT_MODE_UNSPECIFIED {
			if _, ok := grantModes[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.Decision_name {
		if e := hearthv1.Decision(v); e != hearthv1.Decision_DECISION_UNSPECIFIED {
			if _, ok := decisions[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.InboxKind_name {
		if e := hearthv1.InboxKind(v); e != hearthv1.InboxKind_INBOX_KIND_UNSPECIFIED {
			if _, ok := inboxKinds[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.InboxState_name {
		if e := hearthv1.InboxState(v); e != hearthv1.InboxState_INBOX_STATE_UNSPECIFIED {
			if _, ok := inboxStates[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for _, row := range capabilityRows {
		if _, ok := capabilities[row.capability]; !ok || row.title == "" {
			t.Errorf("sheet row %v has no domain value or no title", row.capability)
		}
	}
	if len(capabilityRows) != len(capabilities) {
		t.Errorf("the sheet lists %d capabilities and the table has %d", len(capabilityRows), len(capabilities))
	}
}

func TestDates_TravelAsTimestampsAndAreKeptAsText(t *testing.T) {
	midnight := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	afternoon := midnight.Add(14*time.Hour + 30*time.Minute)
	tests := []struct {
		name     string
		at       time.Time
		withTime bool
		text     string
	}{
		{"a day is its UTC midnight", midnight, false, "2026-10-24"},
		{"a time is the instant", afternoon, true, "2026-10-24 14:30"},
		{"a time that falls on midnight is still a time", midnight, true, "2026-10-24 00:00"},
		{"an instant sent to a day control keeps its time", afternoon, false, "2026-10-24 14:30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dateFromProto(timestamppb.New(tt.at), tt.withTime); got != tt.text {
				t.Errorf("dateFromProto = %q, want %q", got, tt.text)
			}
			if got := dateToProto(tt.text).AsTime(); !got.Equal(tt.at) {
				t.Errorf("dateToProto(%q) = %v, want %v", tt.text, got, tt.at)
			}
		})
	}
	if dateToProto("soon") != nil {
		t.Error("a date that is not one converts to a timestamp")
	}
}

func TestAnswers_FromTheContractNameTheirKind(t *testing.T) {
	yes := true
	five := 5
	tests := []struct {
		name string
		in   *hearthv1.WidgetAnswer
		spec widget.Spec
		want widget.Answer
	}{
		{"a choice", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_ChoiceId{ChoiceId: "a"}}, widget.Spec{},
			widget.Answer{Kind: widget.SingleChoice, Choices: []string{"a"}}},
		{"several choices", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_ChoiceIds{ChoiceIds: &hearthv1.ChoiceIDs{Ids: []string{"a", "b"}}}}, widget.Spec{},
			widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "b"}}},
		{"words", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Text{Text: "hi"}}, widget.Spec{},
			widget.Answer{Kind: widget.FreeText, Text: "hi"}},
		{"yes", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Yes{Yes: true}}, widget.Spec{},
			widget.Answer{Kind: widget.YesNo, Yes: &yes}},
		{"a path", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Path{Path: "/w"}}, widget.Spec{},
			widget.Answer{Kind: widget.Path, Path: "/w"}},
		{"a scale", &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Scale{Scale: 5}}, widget.Spec{},
			widget.Answer{Kind: widget.Scale, Value: &five}},
		{"other with no value", &hearthv1.WidgetAnswer{Other: "x"}, widget.Spec{Kind: widget.SingleChoice},
			widget.Answer{Kind: widget.SingleChoice, Other: "x"}},
		{"nothing is the control left empty", &hearthv1.WidgetAnswer{}, widget.Spec{Kind: widget.Date},
			widget.Answer{Kind: widget.Date}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := answerFromProto(tt.in, tt.spec)
			if got.Kind != tt.want.Kind || got.Other != tt.want.Other || got.Text != tt.want.Text || got.Path != tt.want.Path ||
				len(got.Choices) != len(tt.want.Choices) || (got.Yes == nil) != (tt.want.Yes == nil) || (got.Value == nil) != (tt.want.Value == nil) {
				t.Errorf("answerFromProto = %+v, want %+v", got, tt.want)
			}
			// And back: the same value, with the words the bot read.
			if back := answerToProto(got, "words"); back.GetTextForm() != "words" || back.GetOther() != got.Other {
				t.Errorf("answerToProto = %v", back)
			}
		})
	}
}
