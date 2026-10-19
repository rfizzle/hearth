package rpc

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

// memListener is an in-memory net.Listener: the test's dialer hands it one
// end of a net.Pipe, so the full HTTP and Connect path runs with no kernel
// listener.
type memListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newMemListener() *memListener {
	return &memListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *memListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *memListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *memListener) Addr() net.Addr { return memAddr{} }

func (l *memListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type memAddr struct{}

func (memAddr) Network() string { return "mem" }
func (memAddr) String() string  { return "hearthd" }

const testToken = "launch-token"

// serve starts NewHandler(o) on an in-memory listener and returns an HTTP
// client whose requests reach it.
func serve(t *testing.T, o Options) *http.Client {
	t.Helper()
	l := newMemListener()
	srv := &http.Server{Handler: NewHandler(o), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &http.Client{Transport: &http.Transport{DialContext: l.dial}}
}

// bearer sets the Authorization header on every request the client sends.
func bearer(token string) connect.ClientOption {
	return connect.WithInterceptors(headerInterceptor{token})
}

type headerInterceptor struct{ token string }

func (h headerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if h.token != "" {
			req.Header().Set("Authorization", "Bearer "+h.token)
		}
		return next(ctx, req)
	}
}

func (h headerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		if h.token != "" {
			conn.RequestHeader().Set("Authorization", "Bearer "+h.token)
		}
		return conn
	}
}

func (headerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func streamWords(ctx context.Context, c hearthv1connect.HearthServiceClient, count int32) ([]int32, []string, error) {
	stream, err := c.StreamHello(ctx, connect.NewRequest(&hearthv1.HelloRequest{Count: count}))
	if err != nil {
		return nil, nil, err
	}
	defer stream.Close()
	var indices []int32
	var words []string
	for stream.Receive() {
		indices = append(indices, stream.Msg().GetIndex())
		words = append(words, stream.Msg().GetWord())
	}
	return indices, words, stream.Err()
}

func TestPing(t *testing.T) {
	hc := serve(t, Options{Version: "v1.2.3", Token: testToken})
	tests := []struct {
		name     string
		token    string
		wantCode connect.Code
	}{
		{name: "right token", token: testToken},
		{name: "wrong token", token: "nope", wantCode: connect.CodeUnauthenticated},
		{name: "missing token", token: "", wantCode: connect.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := hearthv1connect.NewHearthServiceClient(hc, "http://hearthd", bearer(tt.token))
			res, err := c.Ping(context.Background(), connect.NewRequest(&hearthv1.PingRequest{}))
			if tt.wantCode != 0 {
				if got := connect.CodeOf(err); got != tt.wantCode {
					t.Fatalf("Ping error code = %v (err %v), want %v", got, err, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Ping: %v", err)
			}
			if got := res.Msg.GetVersion(); got != "v1.2.3" {
				t.Errorf("Ping version = %q, want %q", got, "v1.2.3")
			}
		})
	}
}

func TestStreamHello(t *testing.T) {
	words := []string{"one", "two", "three"}
	hc := serve(t, Options{Token: testToken, Interval: time.Microsecond, Words: words})
	tests := []struct {
		name      string
		token     string
		count     int32
		wantWords []string
		wantCode  connect.Code
	}{
		{name: "five words cycle the list", token: testToken, count: 5, wantWords: []string{"one", "two", "three", "one", "two"}},
		{name: "zero means the default ten", token: testToken, count: 0, wantWords: []string{"one", "two", "three", "one", "two", "three", "one", "two", "three", "one"}},
		{name: "wrong token", token: "nope", count: 3, wantCode: connect.CodeUnauthenticated},
		{name: "missing token", token: "", count: 3, wantCode: connect.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := hearthv1connect.NewHearthServiceClient(hc, "http://hearthd", bearer(tt.token))
			indices, got, err := streamWords(context.Background(), c, tt.count)
			if tt.wantCode != 0 {
				if code := connect.CodeOf(err); code != tt.wantCode {
					t.Fatalf("StreamHello error code = %v (err %v), want %v", code, err, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("StreamHello: %v", err)
			}
			if strings.Join(got, " ") != strings.Join(tt.wantWords, " ") {
				t.Errorf("words = %q, want %q", got, tt.wantWords)
			}
			for i, idx := range indices {
				if idx != int32(i) {
					t.Errorf("chunk %d has index %d", i, idx)
				}
			}
		})
	}
}

func TestStreamHelloStopsWhenCancelled(t *testing.T) {
	hc := serve(t, Options{Token: testToken, Interval: time.Hour})
	c := hearthv1connect.NewHearthServiceClient(hc, "http://hearthd", bearer(testToken))
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.StreamHello(ctx, connect.NewRequest(&hearthv1.HelloRequest{Count: 3}))
	if err != nil {
		t.Fatalf("StreamHello: %v", err)
	}
	defer stream.Close()
	if !stream.Receive() {
		t.Fatalf("first word not received: %v", stream.Err())
	}
	cancel()
	if stream.Receive() {
		t.Fatalf("received %q after cancel", stream.Msg().GetWord())
	}
	if code := connect.CodeOf(stream.Err()); code != connect.CodeCanceled {
		t.Errorf("error code after cancel = %v (err %v), want %v", code, stream.Err(), connect.CodeCanceled)
	}
}

func TestUnknownPath(t *testing.T) {
	hc := serve(t, Options{Token: testToken})
	tests := []struct {
		name  string
		token string
		want  int
	}{
		{name: "without the token", token: "", want: http.StatusUnauthorized},
		{name: "with the token", token: testToken, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "http://hearthd/nothing", nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			res, err := hc.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = res.Body.Close()
			if res.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.want)
			}
		})
	}
}

func TestOptionsDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   Options
		want time.Duration
	}{
		{name: "zero interval", in: Options{}, want: 50 * time.Millisecond},
		{name: "set interval", in: Options{Interval: time.Second}, want: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.withDefaults()
			if got.Interval != tt.want {
				t.Errorf("Interval = %v, want %v", got.Interval, tt.want)
			}
			if len(got.Words) == 0 {
				t.Error("Words is empty after defaults")
			}
		})
	}
}
