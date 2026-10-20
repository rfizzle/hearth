package native

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/document"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

// The bounds of one call to the app.
const (
	// DialTimeout is how long a connection to the app may take before the
	// app is taken to be gone.
	DialTimeout = 2 * time.Second
	// RequestTimeout is the whole of one call: connecting, the app's work
	// and reading the reply.
	RequestTimeout = 30 * time.Second
	// MaxReply is the largest reply read; a longer one is refused unread.
	MaxReply = 4 << 20
	// MaxLabels is the most labels kept for one page.
	MaxLabels = 5
)

// ErrNoApp is a call that could not connect: the app is not open, or not
// listening. It is document's, so a read says Hearth isn't open.
var ErrNoApp = document.ErrNoApp

// Client calls the app's native service. Make one with New; its zero value
// is not usable.
type Client struct {
	rpc     hearthv1connect.NativeServiceClient
	timeout time.Duration // RequestTimeout, shorter in a test
	// down is set by a call that could not connect; later calls return
	// ErrNoApp without trying.
	down atomic.Bool
}

// New is a client of the native service at addr, a loopback IP and port,
// that sends token, the app's token for this direction, as its bearer. Any other address is refused, so the token
// is only ever sent to this machine.
func New(addr, token string) (*Client, error) {
	return newClient(addr, token, (&net.Dialer{Timeout: DialTimeout}).DialContext, RequestTimeout)
}

// dialFunc connects to the app; a test hands one that opens no socket.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func newClient(addr, token string, dial dialFunc, timeout time.Duration) (*Client, error) {
	if err := loopback(addr); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("native: a launch token is required")
	}
	transport := &http.Transport{
		Proxy:                  nil, // loopback only; never through a proxy
		DialContext:            dial,
		DisableKeepAlives:      true, // the app answers one request a connection
		DisableCompression:     true,
		ResponseHeaderTimeout:  timeout,
		MaxResponseHeaderBytes: 16 << 10,
	}
	hc := &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	bearer := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	})
	return &Client{timeout: timeout, rpc: hearthv1connect.NewNativeServiceClient(hc, "http://"+addr,
		connect.WithProtoJSON(),
		connect.WithReadMaxBytes(MaxReply),
		connect.WithInterceptors(bearer),
	)}, nil
}

// loopback refuses an address that is not a loopback IP literal and a port.
func loopback(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("native: %q is not a host and port", addr)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("native: %q is not a loopback address", addr)
	}
	if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("native: %q has no port", addr)
	}
	return nil
}

// ForRun is a client over the same connection settings with the down mark
// clear, so a run that finds the app gone says so once, and the next run
// asks again.
func (c *Client) ForRun() document.Recogniser { return &Client{rpc: c.rpc, timeout: c.timeout} }

// Ping is the version of the native surface the app serves.
func (c *Client) Ping(ctx context.Context) (int32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, err := c.rpc.Ping(ctx, connect.NewRequest(&hearthv1.NativeServicePingRequest{}))
	if err != nil {
		return 0, c.failed(err)
	}
	return res.Msg.GetVersion(), nil
}

// Recognise asks the app for the text of pages of the file at path, an
// absolute path, at most document.Batch of them. Only the pages asked for are
// returned, each once, with at most MaxLabels labels; a page the app left out
// is left out. A call that cannot connect is ErrNoApp, and so is every call
// after it on this client.
func (c *Client) Recognise(ctx context.Context, path string, pages []int) ([]document.Recognised, error) {
	if c.down.Load() {
		return nil, ErrNoApp
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("native: the path must be absolute")
	}
	if len(pages) > document.Batch {
		return nil, fmt.Errorf("native: at most %d pages a call", document.Batch)
	}
	req := &hearthv1.ExtractTextRequest{Path: path}
	asked := make(map[int32]bool, len(pages))
	for _, n := range pages {
		if n < 1 || n > 1<<20 {
			return nil, fmt.Errorf("native: page %d is out of range", n)
		}
		req.Pages = append(req.Pages, int32(n))
		asked[int32(n)] = true
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, err := c.rpc.ExtractText(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, c.failed(err)
	}
	var out []document.Recognised
	for _, p := range res.Msg.GetPages() {
		src := p.GetSource()
		if !asked[p.GetPage()] || (src != hearthv1.TextSource_TEXT_SOURCE_TEXT_LAYER && src != hearthv1.TextSource_TEXT_SOURCE_RECOGNISED) {
			continue
		}
		asked[p.GetPage()] = false
		labels := p.GetLabels()
		out = append(out, document.Recognised{
			Page:          int(p.GetPage()),
			Text:          p.GetText(),
			FromTextLayer: src == hearthv1.TextSource_TEXT_SOURCE_TEXT_LAYER,
			Labels:        labels[:min(len(labels), MaxLabels)],
		})
	}
	return out, nil
}

// failed is err as the caller sees it: ErrNoApp, marking the client down,
// when the call never connected or the app said it is busy with an earlier
// read it gave up waiting for; the error otherwise.
func (c *Client) failed(err error) error {
	var op *net.OpError
	if (errors.As(err, &op) && op.Op == "dial") || connect.CodeOf(err) == connect.CodeUnavailable {
		c.down.Store(true)
		return ErrNoApp
	}
	return fmt.Errorf("native: %w", err)
}
