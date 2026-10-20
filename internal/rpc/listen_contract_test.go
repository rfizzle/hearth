//go:build contract

package rpc

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

// TestPingOverRealListeners proves the handler answers over the two kernel
// transports hearthd serve can bind: loopback TCP and a Unix socket.
func TestPingOverRealListeners(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address func(t *testing.T) string
	}{
		{name: "loopback tcp", network: "tcp", address: func(*testing.T) string { return "127.0.0.1:0" }},
		{name: "unix socket", network: "unix", address: func(t *testing.T) string {
			d, err := os.MkdirTemp("/tmp", "hd")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(d) })
			return filepath.Join(d, "h.sock")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := net.Listen(tt.network, tt.address(t))
			if err != nil {
				t.Fatalf("listen %s: %v", tt.network, err)
			}
			srv := &http.Server{Handler: NewHandler(Options{Version: "v0", Token: testToken}), ReadHeaderTimeout: time.Second}
			go func() { _ = srv.Serve(l) }()
			t.Cleanup(func() { _ = srv.Close() })

			addr := l.Addr().String()
			dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, tt.network, addr)
			}
			hc := &http.Client{Transport: &http.Transport{DialContext: dial}}
			c := hearthv1connect.NewHearthServiceClient(hc, "http://hearthd", bearer(testToken))
			res, err := c.Ping(context.Background(), connect.NewRequest(&hearthv1.PingRequest{}))
			if err != nil {
				t.Fatalf("Ping over %s: %v", tt.network, err)
			}
			if got := res.Msg.GetVersion(); got != "v0" {
				t.Errorf("version = %q, want %q", got, "v0")
			}
		})
	}
}

// TestOnboardingOverLoopback runs the owner's first minutes, and the refusal
// of a caller without the token, over a real loopback listener.
func TestOnboardingOverLoopback(t *testing.T) {
	r := newRig(t, answer("Hi ", "there."))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: NewHandler(r.opts), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })

	addr := l.Addr().String()
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	hc := &http.Client{Transport: &http.Transport{DialContext: dial}}
	_, err = newClients(hc, "").org.GetOrg(t.Context(), connect.NewRequest(&hearthv1.GetOrgRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("GetOrg without the token = %v, want unauthenticated", err)
	}
	checkOnboardingFlow(t, newClients(hc, testToken), r)
}
