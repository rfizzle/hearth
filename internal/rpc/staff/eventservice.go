package staff

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func (s *eventService) Subscribe(ctx context.Context, req *connect.Request[hearthv1.SubscribeRequest], stream *connect.ServerStream[hearthv1.SubscribeResponse]) error {
	events, cancel := s.opts.Bus.Subscribe(req.Msg.GetAfterSequence())
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, open := <-events:
			if !open {
				return connect.NewError(connect.CodeAborted, errors.New("fell behind the events; reconnect from the last sequence read"))
			}
			msg, known := s.opts.eventToProto(ctx, e)
			if !known {
				continue
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}
