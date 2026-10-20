package wire

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/widget"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/gateway"
	"github.com/rfizzle/hearth/internal/store"
	storebooks "github.com/rfizzle/hearth/internal/store/books"
)

// Fail turns an error a domain package returned into the Connect error the
// app reads. It is the one place a domain error picks a code, so a handler
// hands its error to Reply and never inspects it.
// See docs/architecture.md#1-layering
func Fail(err error) error {
	var coded *connect.Error
	if errors.As(err, &coded) {
		return err
	}
	codes := []struct {
		code connect.Code
		errs []error
	}{
		{connect.CodeNotFound, []error{store.ErrNotFound, provider.ErrNoProfile, keystore.ErrNoKey}},
		{connect.CodeAlreadyExists, []error{store.ErrFolderTaken}},
		{connect.CodeFailedPrecondition, []error{orchestrate.ErrBusy, orchestrate.ErrWaiting, orchestrate.ErrPaused, orchestrate.ErrClosed, orchestrate.ErrNoCoordinator, project.ErrState}},
		{connect.CodeInvalidArgument, []error{orchestrate.ErrMismatch, orchestrate.ErrAttachment, org.ErrReservedName, widget.ErrInvalid, project.ErrInvalid}},
		{connect.CodeInvalidArgument, []error{inbox.ErrNoStanding, inbox.ErrBadFolder, inbox.ErrReservedFolder}},
		{connect.CodeInvalidArgument, []error{integration.ErrEmptyKey, integration.ErrUnknown}},
		{connect.CodeInvalidArgument, []error{gateway.ErrAddress}},
		{connect.CodeUnavailable, []error{gateway.ErrList}},
		{connect.CodeInvalidArgument, []error{handbook.ErrNoTitle, handbook.ErrFull, storebooks.ErrBadScope}},
		{connect.CodeUnavailable, []error{ErrUnavailable}},
	}
	for _, c := range codes {
		if slices.ContainsFunc(c.errs, func(e error) bool { return errors.Is(err, e) }) {
			return connect.NewError(c.code, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}

// Reply is a handler's answer: the message, or the code its error maps to.
func Reply[T any](msg *T, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, Fail(err)
	}
	return connect.NewResponse(msg), nil
}

// Invalid is the error for a request the domain was never asked about:
// a value the contract cannot carry into it.
func Invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

// PlainName is nil when name holds no invisible character (memory.Plain would
// drop one). A name is refused rather than renamed, since a person who typed
// it would be surprised by a different one.
func PlainName(name string) error {
	if memory.Plain(name) != name {
		return Invalid("the name has an invisible character, such as a zero-width space or a text direction mark; retype it as plain text")
	}
	return nil
}

// Need is invalid_argument naming the first field that is empty, from
// name and value pairs.
func Need(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			return Invalid("%s is required", pairs[i])
		}
	}
	return nil
}

// Each converts a list.
func Each[S, D any](in []S, f func(S) D) []D {
	out := make([]D, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

// ErrUnavailable marks a call that needed a model and could reach none: the
// owner has not signed in, or the provider failed.
var ErrUnavailable = errors.New("no model could be reached")

// Unavailable marks err as ErrUnavailable and keeps its words, so the owner
// reads what to do.
func Unavailable(err error) error { return unavailable{err} }

type unavailable struct{ error }

func (u unavailable) Unwrap() error        { return u.error }
func (u unavailable) Is(target error) bool { return target == ErrUnavailable }
