package rpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/org/widget"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The conversions between the widget package's values and the contract's.
// A message and an Inbox item hold the control and the owner's answer as
// JSON; these turn it into messages, and an answer back. The core validates
// and checks in the widget package; nothing here judges a value.
// See docs/model.md#owner-interaction.

// widgetOf is the control the stored spec describes, and the spec read, or
// nil when there is none. A spec the widget package cannot read was never
// stored by the core, so it is left off rather than failing the thread.
func widgetOf(stored string) (*hearthv1.Widget, widget.Spec) {
	if stored == "" {
		return nil, widget.Spec{}
	}
	spec, err := widget.ParseSpec([]byte(stored))
	if err != nil {
		return nil, widget.Spec{}
	}
	return widgetToProto(spec), spec
}

// answerOf is the owner's stored answer to the control spec describes, in
// the words the bot read, or nil when there is none.
func answerOf(stored string, spec widget.Spec) *hearthv1.WidgetAnswer {
	if stored == "" {
		return nil
	}
	a, err := widget.ParseAnswer([]byte(stored))
	if err != nil {
		return nil
	}
	return answerToProto(a, widget.Render(spec, a))
}

func widgetToProto(s widget.Spec) *hearthv1.Widget {
	out := &hearthv1.Widget{Required: s.Required}
	switch {
	case s.Single != nil:
		out.Kind = &hearthv1.Widget_SingleChoice{SingleChoice: &hearthv1.SingleChoice{
			Options: each(s.Single.Options, choiceToProto), AllowOther: s.Single.AllowOther,
		}}
	case s.Multiple != nil:
		out.Kind = &hearthv1.Widget_MultipleChoice{MultipleChoice: &hearthv1.MultipleChoice{
			Options: each(s.Multiple.Options, choiceToProto), Min: int32(s.Multiple.MinChoices), Max: int32(s.Multiple.MaxChoices),
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

func choiceToProto(o widget.Option) *hearthv1.Choice {
	return &hearthv1.Choice{Id: o.ID, Label: o.Label, Detail: o.Detail}
}

// A date the owner picks travels as a timestamp and is kept as text, in
// UTC: a day as that day's midnight, a time as the instant. The widget
// package reads and checks the text in UTC, so the two sides agree on which
// day a date is wherever the Mac is.

// day is the UTC midnight of a stored day, or nil for none.
func day(stored string) *timestamppb.Timestamp {
	t, err := time.Parse(widget.DateLayout, stored)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}

// dateToProto is the timestamp a stored date or date and time names.
func dateToProto(stored string) *timestamppb.Timestamp {
	if t, err := time.Parse(widget.DateTimeLayout, stored); err == nil {
		return timestamppb.New(t)
	}
	return day(stored)
}

// dateFromProto is the text for a timestamp the owner picked: a day for the
// UTC midnight a calendar day travels as, unless the control asked for a
// time, and the instant otherwise.
func dateFromProto(t *timestamppb.Timestamp, withTime bool) string {
	at := t.AsTime()
	if !withTime && at.Equal(at.Truncate(24*time.Hour)) {
		return at.Format(widget.DateLayout)
	}
	return at.Format(widget.DateTimeLayout)
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

// answerFromProto is the answer the owner sent to the control spec, for the
// widget package to check. The value names the kind; an answer with no value
// is "Other" when it has text for it, and otherwise a control left empty, of
// the control's own kind.
func answerFromProto(v *hearthv1.WidgetAnswer, spec widget.Spec) widget.Answer {
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
		a.Kind, a.Date = widget.Date, dateFromProto(x.Date, spec.Date != nil && spec.Date.WithTime)
	case *hearthv1.WidgetAnswer_Path:
		a.Kind, a.Path = widget.Path, x.Path
	case *hearthv1.WidgetAnswer_Scale:
		n := int(x.Scale)
		a.Kind, a.Value = widget.Scale, &n
	case nil:
		a.Kind = spec.Kind
		if a.Other != "" {
			a.Kind = widget.SingleChoice
		}
	}
	return a
}
