package rpc

import (
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/widget"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/store"
)

// fail turns an error a domain package returned into the Connect error the
// app reads. It is the one place a domain error picks a code, so a handler
// hands its error to reply and never inspects it.
// See docs/architecture.md#1-layering
func fail(err error) error {
	code := connect.CodeInternal
	var coded *connect.Error
	switch {
	case errors.As(err, &coded):
		return err
	case errors.Is(err, store.ErrNotFound), errors.Is(err, provider.ErrNoProfile):
		code = connect.CodeNotFound
	case errors.Is(err, store.ErrFolderTaken):
		code = connect.CodeAlreadyExists
	case errors.Is(err, orchestrate.ErrBusy), errors.Is(err, orchestrate.ErrWaiting), errors.Is(err, orchestrate.ErrClosed):
		code = connect.CodeFailedPrecondition
	case errors.Is(err, orchestrate.ErrMismatch), errors.Is(err, widget.ErrInvalid), errors.Is(err, inbox.ErrNoStanding), errors.Is(err, inbox.ErrBadFolder):
		code = connect.CodeInvalidArgument
	case errors.Is(err, draft.ErrUnavailable):
		code = connect.CodeUnavailable
	case errors.Is(err, integration.ErrEmptyKey), errors.Is(err, integration.ErrUnknown):
		code = connect.CodeInvalidArgument
	}
	return connect.NewError(code, err)
}

// reply is a handler's answer: the message, or the code its error maps to.
func reply[T any](msg *T, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(msg), nil
}

// invalid is the error for a request the domain was never asked about:
// a value the contract cannot carry into it.
func invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

// need is invalid_argument naming the first field that is empty, from
// name and value pairs.
func need(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			return invalid("%s is required", pairs[i])
		}
	}
	return nil
}

// each converts a list.
func each[S, D any](in []S, f func(S) D) []D {
	out := make([]D, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
