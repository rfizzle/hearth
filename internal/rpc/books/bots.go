package books

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/provider/models"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// UpdateBot lays the owner's edits over the stored bot and keeps it, then
// tells the bus. A new name is refused when its folder would be the
// Inbox's, as at hiring.
// See docs/model.md#people
func (s Bots) UpdateBot(ctx context.Context, req *connect.Request[hearthv1.UpdateBotRequest]) (*connect.Response[hearthv1.UpdateBotResponse], error) {
	m := req.Msg
	if err := wire.Need("id", m.GetId()); err != nil {
		return nil, err
	}
	bot, err := s.Store.Bot(ctx, m.GetId())
	if err == nil {
		bot, err = edit(bot, m)
	}
	if err == nil {
		bot, err = s.Store.UpdateBot(ctx, bot)
	}
	if err != nil {
		return nil, wire.Fail(err)
	}
	s.Publish(wire.BotChanged{Bot: bot})
	return connect.NewResponse(&hearthv1.UpdateBotResponse{Bot: wire.BotToProto(bot)}), nil
}

// edit is bot with the request laid over it: an empty field, or an
// unspecified tier, keeps what is stored, and clear_model_override drops the
// override. An override is the one string models writes for a pick.
func edit(bot org.Bot, m *hearthv1.UpdateBotRequest) (org.Bot, error) {
	tier, err := wire.TierFromProto(cmp.Or(m.GetTier(), hearthv1.Tier(bot.Tier)))
	if err != nil {
		return bot, err
	}
	if err := wire.PlainName(strings.TrimSpace(m.GetName())); err != nil {
		return bot, err
	}
	bot.Name = keep(m.GetName(), bot.Name)
	bot.Role = keep(memory.OneLine(memory.Plain(m.GetRole())), bot.Role)
	bot.Tier = tier
	bot.JobDescription = keep(memory.Plain(m.GetJobDescription()), bot.JobDescription)
	bot.Avatar = keep(m.GetAvatar(), bot.Avatar)
	if bot.ModelOverride = keep(m.GetModelOverride(), bot.ModelOverride); m.GetClearModelOverride() {
		bot.ModelOverride = ""
	}
	if _, ok := models.Parse(bot.ModelOverride); bot.ModelOverride != "" && !ok {
		return bot, wire.Invalid("model_override %q does not name a provider and a model", bot.ModelOverride)
	}
	return bot, org.CheckName(bot.Name)
}

// keep is sent unless it is blank, when it is stored.
func keep(sent, stored string) string {
	if strings.TrimSpace(sent) == "" {
		return stored
	}
	return sent
}

// ListNotes is the bot's notes, oldest first, and how many it may keep.
// See docs/model.md#knowledge
func (s Bots) ListNotes(ctx context.Context, req *connect.Request[hearthv1.ListNotesRequest]) (*connect.Response[hearthv1.ListNotesResponse], error) {
	id := req.Msg.GetBotId()
	if err := wire.Need("bot_id", id); err != nil {
		return nil, err
	}
	notes, err := s.Store.Notes(ctx, id)
	return wire.Reply(&hearthv1.ListNotesResponse{Notes: wire.Each(notes, noteToProto), Cap: memory.Cap}, err)
}

// EditNote replaces a note's words, which are the owner's, typed.
func (s Bots) EditNote(ctx context.Context, req *connect.Request[hearthv1.EditNoteRequest]) (*connect.Response[hearthv1.EditNoteResponse], error) {
	text := req.Msg.GetText()
	if err := cmp.Or(wire.Need("text", text), wire.Words("the note", text)); err != nil {
		return nil, wire.Fail(err)
	}
	if text = memory.Plain(text); strings.TrimSpace(text) == "" {
		return nil, wire.Invalid("the note holds only invisible characters; write it as plain text")
	}
	note, err := s.change(ctx, req.Msg.GetId(), func(id string) (memory.Note, error) { return s.Store.EditNote(ctx, id, text) })
	return wire.Reply(&hearthv1.EditNoteResponse{Note: note}, err)
}

// PinNote pins a note, which a tidy then keeps, or unpins it.
func (s Bots) PinNote(ctx context.Context, req *connect.Request[hearthv1.PinNoteRequest]) (*connect.Response[hearthv1.PinNoteResponse], error) {
	pin := func(id string) (memory.Note, error) { return s.Store.PinNote(ctx, id, req.Msg.GetPinned()) }
	note, err := s.change(ctx, req.Msg.GetId(), pin)
	return wire.Reply(&hearthv1.PinNoteResponse{Note: note}, err)
}

// change makes an edit to the note with id, tells the bus about the note
// as stored, and is it in the contract's form.
func (s Bots) change(ctx context.Context, id string, edit func(id string) (memory.Note, error)) (*hearthv1.Note, error) {
	if err := wire.Need("id", id); err != nil {
		return nil, err
	}
	n, err := edit(id)
	if err != nil {
		return nil, err
	}
	s.Publish(wire.NoteChanged{BotID: n.BotID, Note: n})
	return noteToProto(n), nil
}

// DeleteNote makes the bot forget a note. The event names the bot that kept
// it, which the store reads notes by.
func (s Bots) DeleteNote(ctx context.Context, req *connect.Request[hearthv1.DeleteNoteRequest]) (*connect.Response[hearthv1.DeleteNoteResponse], error) {
	owner, err := s.keeper(ctx, req.Msg.GetId())
	if err == nil {
		err = s.Store.DeleteNote(ctx, req.Msg.GetId())
	}
	if err != nil {
		return nil, wire.Fail(err)
	}
	s.Publish(wire.NoteChanged{BotID: owner, Note: memory.Note{ID: req.Msg.GetId()}, Deleted: true})
	return connect.NewResponse(&hearthv1.DeleteNoteResponse{}), nil
}

// keeper is the id of the bot that keeps the note with id, which must be
// named; empty when no bot keeps it. A read that fails refuses the delete,
// so the event never goes out without its bot.
func (s Bots) keeper(ctx context.Context, id string) (string, error) {
	if err := wire.Need("id", id); err != nil {
		return "", err
	}
	bots, err := s.Store.Bots(ctx)
	for _, b := range bots {
		notes, err := s.Store.Notes(ctx, b.ID)
		if err != nil {
			return "", err
		}
		if slices.ContainsFunc(notes, func(n memory.Note) bool { return n.ID == id }) {
			return b.ID, nil
		}
	}
	return "", err
}
