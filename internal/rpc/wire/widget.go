package wire

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/org/widget"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// Control is the control a question asks the owner to answer with, read from
// the spec a message or an item stores. The zero value is a question asked
// in words alone.
type Control struct{ spec widget.Spec }

// ControlOf reads the stored spec. A spec the widget package cannot read was never
// stored by the core, so it is taken as none rather than failing the thread.
func ControlOf(stored string) Control {
	if stored == "" {
		return Control{}
	}
	spec, err := widget.ParseSpec([]byte(stored))
	if err != nil || !widget.Known(spec.Kind) {
		return Control{}
	}
	return Control{spec}
}

// maxWords is the longest reply or reason the owner may type, in runes.
const maxWords = 4096

// Words refuses text the owner typed, named what in the refusal, that is
// longer than a reply needs or holds a control character other than a line
// break or a tab: the answer field is a multi-line one. The refusal wraps
// widget.ErrInvalid.
func Words(what, text string) error {
	if n := utf8.RuneCountInString(text); n > maxWords {
		return fmt.Errorf("%w: %s is %d characters; keep it to %d or fewer", widget.ErrInvalid, what, n, maxWords)
	}
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) {
		return fmt.Errorf("%w: %s holds a control character; write it as plain text", widget.ErrInvalid, what)
	}
	return nil
}

// Proto is the control in the contract's form, or nil for none.
func (c Control) Proto() *hearthv1.Widget {
	s := c.spec
	if s.Kind == "" {
		return nil
	}
	out := &hearthv1.Widget{Required: s.Required}
	switch {
	case s.Single != nil:
		out.Kind = &hearthv1.Widget_SingleChoice{SingleChoice: &hearthv1.SingleChoice{
			Options: choices(s.Single.Options), AllowOther: s.Single.AllowOther,
		}}
	case s.Multiple != nil:
		out.Kind = &hearthv1.Widget_MultipleChoice{MultipleChoice: &hearthv1.MultipleChoice{
			Options: choices(s.Multiple.Options), Min: int32(s.Multiple.MinChoices), Max: int32(s.Multiple.MaxChoices),
		}}
	case s.FreeText != nil:
		out.Kind = &hearthv1.Widget_FreeText{FreeText: &hearthv1.FreeText{
			Placeholder: s.FreeText.Placeholder, MaxLength: int32(s.FreeText.MaxLength), Multiline: s.FreeText.Multiline,
		}}
	case s.YesNo != nil:
		out.Kind = &hearthv1.Widget_YesNo{YesNo: &hearthv1.YesNo{YesLabel: s.YesNo.YesLabel, NoLabel: s.YesNo.NoLabel}}
	case s.Date != nil:
		out.Kind = &hearthv1.Widget_Date{Date: &hearthv1.DatePick{
			WithTime: s.Date.WithTime, Earliest: day(s.Date.Earliest), Latest: day(s.Date.Latest),
		}}
	case s.Path != nil:
		out.Kind = &hearthv1.Widget_Path{Path: &hearthv1.PathPick{
			Folders: s.Path.Folders, Files: s.Path.Files, StartPath: s.Path.StartPath,
		}}
	case s.Scale != nil:
		out.Kind = &hearthv1.Widget_Scale{Scale: &hearthv1.Scale{
			Min: int32(s.Scale.Min), Max: int32(s.Scale.Max), MinLabel: s.Scale.MinLabel, MaxLabel: s.Scale.MaxLabel,
		}}
	}
	return out
}

func choices(opts []widget.Option) []*hearthv1.Choice {
	out := make([]*hearthv1.Choice, 0, len(opts))
	for _, o := range opts {
		out = append(out, &hearthv1.Choice{Id: o.ID, Label: o.Label, Detail: o.Detail})
	}
	return out
}

// A date the owner picks travels as a timestamp and is kept as text in the
// Mac's own zone, the zone of the owner's picker and of the bounds a bot
// writes: a day as that day's local midnight, a time as the instant read on
// the local clock. The core and the app run on one Mac, so the owner and the
// bot read the same day and the same time.

// day is the local midnight of a stored day, or nil for none.
func day(stored string) *timestamppb.Timestamp {
	t, err := time.ParseInLocation(widget.DateLayout, stored, time.Local)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}

// dateToProto is the timestamp a stored date or date and time names.
func dateToProto(stored string) *timestamppb.Timestamp {
	if t, err := time.ParseInLocation(widget.DateTimeLayout, stored, time.Local); err == nil {
		return timestamppb.New(t)
	}
	return day(stored)
}

// dateFromProto is the text for a timestamp the owner picked: a day for the
// local midnight a calendar day travels as, unless the control asked for a
// time, and the instant on the local clock otherwise.
func dateFromProto(t *timestamppb.Timestamp, withTime bool) string {
	at := t.AsTime().In(time.Local)
	if h, m, s := at.Clock(); !withTime && h == 0 && m == 0 && s == 0 && at.Nanosecond() == 0 {
		return at.Format(widget.DateLayout)
	}
	return at.Format(widget.DateTimeLayout)
}

// AnswerProto is the owner's stored answer to the control in the contract's
// form, with the words the bot read for it, or nil when there is none.
func (c Control) AnswerProto(stored string) *hearthv1.WidgetAnswer {
	if stored == "" {
		return nil
	}
	a, err := widget.ParseAnswer([]byte(stored))
	if err != nil {
		return nil
	}
	return answerToProto(a, widget.Render(c.spec, a))
}

func answerToProto(a widget.Answer, textForm string) *hearthv1.WidgetAnswer {
	out := &hearthv1.WidgetAnswer{Other: a.Other, TextForm: textForm}
	switch a.Kind {
	case widget.SingleChoice:
		if len(a.Choices) > 0 {
			out.Value = &hearthv1.WidgetAnswer_ChoiceId{ChoiceId: a.Choices[0]}
		}
	case widget.MultipleChoice:
		out.Value = &hearthv1.WidgetAnswer_ChoiceIds{ChoiceIds: &hearthv1.ChoiceIDs{Ids: a.Choices}}
	case widget.FreeText:
		out.Value = &hearthv1.WidgetAnswer_Text{Text: a.Text}
	case widget.YesNo:
		if a.Yes != nil {
			out.Value = &hearthv1.WidgetAnswer_Yes{Yes: *a.Yes}
		}
	case widget.Date:
		if a.Date != "" {
			out.Value = &hearthv1.WidgetAnswer_Date{Date: dateToProto(a.Date)}
		}
	case widget.Path:
		out.Value = &hearthv1.WidgetAnswer_Path{Path: a.Path}
	case widget.Scale:
		if a.Value != nil {
			out.Value = &hearthv1.WidgetAnswer_Scale{Scale: int32(*a.Value)}
		}
	}
	return out
}

// ReadAnswer is the answer the owner sent to the control, for the widget
// package to check, and nil when they sent none. The value names the kind;
// an answer with no value is "Other" when it has text for it, and otherwise
// the control left empty, of the control's own kind.
func (c Control) ReadAnswer(v *hearthv1.WidgetAnswer) *widget.Answer {
	if v == nil {
		return nil
	}
	a := widget.Answer{Other: v.GetOther()}
	switch x := v.GetValue().(type) {
	case *hearthv1.WidgetAnswer_ChoiceId:
		a.Kind, a.Choices = widget.SingleChoice, []string{x.ChoiceId}
	case *hearthv1.WidgetAnswer_ChoiceIds:
		a.Kind, a.Choices = widget.MultipleChoice, x.ChoiceIds.GetIds()
	case *hearthv1.WidgetAnswer_Text:
		a.Kind, a.Text = widget.FreeText, x.Text
	case *hearthv1.WidgetAnswer_Yes:
		a.Kind, a.Yes = widget.YesNo, &x.Yes
	case *hearthv1.WidgetAnswer_Date:
		a.Kind, a.Date = widget.Date, dateFromProto(x.Date, c.spec.Date != nil && c.spec.Date.WithTime)
	case *hearthv1.WidgetAnswer_Path:
		a.Kind, a.Path = widget.Path, x.Path
	case *hearthv1.WidgetAnswer_Scale:
		n := int(x.Scale)
		a.Kind, a.Value = widget.Scale, &n
	case nil:
		a.Kind = c.spec.Kind
		if a.Other != "" {
			a.Kind = widget.SingleChoice
		}
	}
	return &a
}
