package rpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

const (
	// defaultInterval is the pause between streamed words.
	defaultInterval = 50 * time.Millisecond
	// defaultCount is how many words StreamHello sends when asked for none.
	defaultCount = 10
	// maxCount caps how many words one StreamHello may ask for. The count is
	// request input and every word holds the stream open for an interval, so
	// without a cap one caller could pin a handler for as long as it liked.
	// 1,000 words is far more than the demo stream needs and ends within a
	// minute at the default pace.
	maxCount     = 1000
	defaultWords = "Hello from hearthd, the Go core of hearth, streaming one word at a time to the app."
)

// Options configures the handler NewHandler returns.
type Options struct {
	// Version is what Ping reports.
	Version string
	// Token is the launch token every request must carry as a bearer token.
	Token string
	// Interval is the pause between streamed words; zero means 50 ms.
	Interval time.Duration
	// Words is the list StreamHello cycles through; empty means a fixed sentence.
	Words []string
}

func (o Options) withDefaults() Options {
	if o.Interval <= 0 {
		o.Interval = defaultInterval
	}
	if len(o.Words) == 0 {
		o.Words = strings.Fields(defaultWords)
	}
	return o
}

// NewHandler returns the HTTP handler for the hearth.v1 contract, behind the
// launch-token check.
func NewHandler(o Options) http.Handler {
	o = o.withDefaults()
	mux := http.NewServeMux()
	path, h := hearthv1connect.NewHearthServiceHandler(&service{opts: o})
	mux.Handle(path, h)
	return requireToken(o.Token, mux)
}

type service struct {
	opts Options
}

func (s *service) Ping(context.Context, *connect.Request[hearthv1.PingRequest]) (*connect.Response[hearthv1.PingResponse], error) {
	return connect.NewResponse(&hearthv1.PingResponse{Version: s.opts.Version}), nil
}

func (s *service) StreamHello(ctx context.Context, req *connect.Request[hearthv1.HelloRequest], stream *connect.ServerStream[hearthv1.HelloChunk]) error {
	count := int(req.Msg.GetCount())
	if count <= 0 {
		count = defaultCount
	}
	count = min(count, maxCount)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for i := range count {
		if i > 0 {
			timer.Reset(s.opts.Interval)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
		chunk := &hearthv1.HelloChunk{Index: int32(i), Word: s.opts.Words[i%len(s.opts.Words)]}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}
