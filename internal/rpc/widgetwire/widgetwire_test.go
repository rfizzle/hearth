package widgetwire

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/org/widget"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// inZone makes zone the Mac's local zone for the test.
func inZone(t *testing.T, zone string) {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Skipf("no zone data for %s: %v", zone, err)
	}
	was := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = was })
}

func TestDates_AMacEastOrWestOfUTCKeepsTheOwnersDayAndTime(t *testing.T) {
	for _, zone := range []string{"Asia/Tokyo", "America/Los_Angeles", "UTC"} {
		t.Run(zone, func(t *testing.T) {
			inZone(t, zone)
			// The owner picks a plain day: the picker's start of the day.
			day := timestamppb.New(time.Date(2026, 10, 24, 0, 0, 0, 0, time.Local))
			if got := dateFromProto(day, false); got != "2026-10-24" {
				t.Errorf("a plain day = %q, want 2026-10-24", got)
			}
			// And 14:30 on a control that asks for a time.
			at := timestamppb.New(time.Date(2026, 10, 24, 14, 30, 0, 0, time.Local))
			if got := dateFromProto(at, true); got != "2026-10-24 14:30" {
				t.Errorf("14:30 = %q, want 2026-10-24 14:30", got)
			}
			// 00:30 the next day stays the next day, past a bound the bot set.
			late := timestamppb.New(time.Date(2026, 11, 1, 0, 30, 0, 0, time.Local))
			if got := dateFromProto(late, true); got != "2026-11-01 00:30" {
				t.Errorf("00:30 = %q, want 2026-11-01 00:30", got)
			}
			// Back again: the stored text names the instant the owner picked.
			if got := dateToProto("2026-10-24 14:30").AsTime(); !got.Equal(at.AsTime()) {
				t.Errorf("round trip of 14:30 = %v, want %v", got, at.AsTime())
			}
			if got := dateToProto("2026-10-24").AsTime(); !got.Equal(day.AsTime()) {
				t.Errorf("round trip of the day = %v, want %v", got, day.AsTime())
			}
		})
	}
}

func TestWords_AreShortPlainText(t *testing.T) {
	for text, want := range map[string]bool{
		"": true, "Keep it": true, "two\nlines": true, "tab\there": true, "bell\x07": false, "nul\x00": false,
		strings.Repeat("a", 4096): true, strings.Repeat("a", 4097): false,
	} {
		err := Words("the answer", text)
		if (err == nil) != want || (err != nil && !Invalid(err)) {
			t.Errorf("Words(%.12q) = %v, want ok %v and an Invalid refusal", text, err, want)
		}
	}
}

func TestDates_TravelAsTimestampsAndAreKeptAsText(t *testing.T) {
	inZone(t, "Asia/Tokyo")
	midnight := time.Date(2026, 10, 24, 0, 0, 0, 0, time.Local)
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
			got := Control{tt.spec}.ReadAnswer(tt.in)
			if got.Kind != tt.want.Kind || got.Other != tt.want.Other || got.Text != tt.want.Text || got.Path != tt.want.Path ||
				len(got.Choices) != len(tt.want.Choices) || (got.Yes == nil) != (tt.want.Yes == nil) || (got.Value == nil) != (tt.want.Value == nil) {
				t.Errorf("answerFromProto = %+v, want %+v", got, tt.want)
			}
			// And back: the same value, with the words the bot read.
			if back := answerToProto(*got, "words"); back.GetTextForm() != "words" || back.GetOther() != got.Other {
				t.Errorf("answerToProto = %v", back)
			}
		})
	}
}

func TestControl_EveryKindReadsIntoTheContract(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		has    func(*hearthv1.Widget) bool
	}{
		{"single", `{"kind":"single_choice","required":true,"allow_other":true,"options":[{"id":"a","label":"A","detail":"d"},{"id":"b","label":"B"}]}`,
			func(w *hearthv1.Widget) bool {
				s := w.GetSingleChoice()
				return w.GetRequired() && s.GetAllowOther() && len(s.GetOptions()) == 2 && s.GetOptions()[0].GetDetail() == "d"
			}},
		{"multiple", `{"kind":"multiple_choice","min_choices":1,"max_choices":2,"options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}`,
			func(w *hearthv1.Widget) bool {
				return w.GetMultipleChoice().GetMin() == 1 && w.GetMultipleChoice().GetMax() == 2
			}},
		{"free text", `{"kind":"free_text","max_length":50,"placeholder":"p","multiline":true}`,
			func(w *hearthv1.Widget) bool {
				f := w.GetFreeText()
				return f.GetMaxLength() == 50 && f.GetPlaceholder() == "p" && f.GetMultiline()
			}},
		{"yes no", `{"kind":"yes_no","yes_label":"Keep","no_label":"Drop"}`,
			func(w *hearthv1.Widget) bool {
				return w.GetYesNo().GetYesLabel() == "Keep" && w.GetYesNo().GetNoLabel() == "Drop"
			}},
		{"date", `{"kind":"date","with_time":true,"earliest":"2026-10-01","latest":"2026-10-31"}`,
			func(w *hearthv1.Widget) bool {
				d := w.GetDate()
				return d.GetWithTime() && d.GetEarliest().AsTime().In(time.Local).Day() == 1 && d.GetLatest().AsTime().In(time.Local).Day() == 31
			}},
		{"path", `{"kind":"path","folders":true,"start_path":"/w"}`,
			func(w *hearthv1.Widget) bool { return w.GetPath().GetFolders() && w.GetPath().GetStartPath() == "/w" }},
		{"scale", `{"kind":"scale","min":1,"max":5,"min_label":"low","max_label":"high"}`,
			func(w *hearthv1.Widget) bool {
				s := w.GetScale()
				return s.GetMin() == 1 && s.GetMax() == 5 && s.GetMaxLabel() == "high"
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if w := Of(tt.stored).Proto(); w == nil || !tt.has(w) {
				t.Errorf("Proto = %v", w)
			}
		})
	}
	for _, none := range []string{"", "not json", `{"kind":"nonsense"}`} {
		if w := Of(none).Proto(); w != nil {
			t.Errorf("Of(%q).Proto() = %v, want none", none, w)
		}
	}
	if ans := Of(`{"kind":"yes_no"}`).AnswerProto(`{"kind":"yes_no","yes":true}`); ans.GetTextForm() != "The owner said: Yes" || !ans.GetYes() {
		t.Errorf("AnswerProto = %v", ans)
	}
	if Of("").ReadAnswer(nil) != nil {
		t.Error("no value read as an answer")
	}
	if !Invalid(widget.Check(widget.Spec{Kind: widget.Date}, widget.Answer{Kind: widget.YesNo})) {
		t.Error("a refusal by the widget package is not Invalid")
	}
}
