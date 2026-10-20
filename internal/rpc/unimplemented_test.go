package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

// TestNotYetServed pins the handlers that wait for their real version: each
// answers unimplemented, behind the token like every other call.
func TestNotYetServed(t *testing.T) {
	r := newRig(t)
	hc := serve(t, r.opts)
	c := newClients(hc, testToken)
	inbox := hearthv1connect.NewInboxServiceClient(hc, "http://hearthd", bearer(testToken))

	calls := map[string]func(context.Context) error{
		"ListGrants": func(ctx context.Context) error {
			_, err := c.bot.ListGrants(ctx, connect.NewRequest(&hearthv1.ListGrantsRequest{}))
			return err
		},
		"SetGrants": func(ctx context.Context) error {
			_, err := c.bot.SetGrants(ctx, connect.NewRequest(&hearthv1.SetGrantsRequest{}))
			return err
		},
		"GetConversation": func(ctx context.Context) error {
			_, err := c.conv.GetConversation(ctx, connect.NewRequest(&hearthv1.GetConversationRequest{}))
			return err
		},
		"Retry": func(ctx context.Context) error {
			_, err := c.conv.Retry(ctx, connect.NewRequest(&hearthv1.RetryRequest{}))
			return err
		},
		"ListInbox": func(ctx context.Context) error {
			_, err := inbox.ListInbox(ctx, connect.NewRequest(&hearthv1.ListInboxRequest{}))
			return err
		},
		"Decide": func(ctx context.Context) error {
			_, err := inbox.Decide(ctx, connect.NewRequest(&hearthv1.DecideRequest{}))
			return err
		},
		"Answer": func(ctx context.Context) error {
			_, err := inbox.Answer(ctx, connect.NewRequest(&hearthv1.AnswerRequest{}))
			return err
		},
		"Acknowledge": func(ctx context.Context) error {
			_, err := inbox.Acknowledge(ctx, connect.NewRequest(&hearthv1.AcknowledgeRequest{}))
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if got := connect.CodeOf(call(t.Context())); got != connect.CodeUnimplemented {
				t.Errorf("code = %v, want unimplemented", got)
			}
		})
	}
}
