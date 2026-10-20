package native

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/document"
)

const testToken = "launch-token"

// memListener is a loopback listener held in memory: the client dials it
// with a pipe, so the test opens no socket.
type memListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
	dials atomic.Int32
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

func (l *memListener) Close() error { l.once.Do(func() { close(l.done) }); return nil }

func (l *memListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4242} }

func (l *memListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	l.dials.Add(1)
	server, client := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// app is an httptest server playing the app: it holds the request to the
// shape the app's parser accepts (POST, JSON, a Content-Length, the bearer)
// and answers with the JSON reply serve writes.
func app(t *testing.T, serve func(w http.ResponseWriter, req map[string]any)) (*Client, *memListener) {
	t.Helper()
	l := newMemListener()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != http.MethodPost || r.URL.Path != "/hearth.v1.NativeService/ExtractText":
			t.Errorf("request %s %s, want POST to ExtractText", r.Method, r.URL.Path)
		case r.Header.Get("Content-Type") != "application/json":
			t.Errorf("content type %q, want application/json", r.Header.Get("Content-Type"))
		case r.ContentLength < 0 || len(r.TransferEncoding) > 0:
			t.Errorf("body length %d, encoding %v: the app reads a Content-Length body only", r.ContentLength, r.TransferEncoding)
		}
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"code":"unauthenticated","message":"the launch token does not match"}`)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		serve(w, req)
	}))
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)
	c, err := newClient("127.0.0.1:4242", testToken, l.dial, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	return c, l
}

func TestRecogniseReadsTheReply(t *testing.T) {
	var got map[string]any
	c, _ := app(t, func(w http.ResponseWriter, req map[string]any) {
		got = req
		_, _ = io.WriteString(w, `{"pages":[
			{"page":3,"text":"From the layer","source":"TEXT_SOURCE_TEXT_LAYER"},
			{"page":5,"text":"Seen","source":"TEXT_SOURCE_RECOGNISED","labels":["a","b","c","d","e","f"]},
			{"page":9,"text":"not asked for","source":"TEXT_SOURCE_RECOGNISED"},
			{"page":5,"text":"twice","source":"TEXT_SOURCE_RECOGNISED"},
			{"page":7,"text":"no source"}
		]}`)
	})
	out, err := c.Recognise(context.Background(), "/Users/me/Hearth/scan.pdf", []int{3, 5, 7})
	if err != nil {
		t.Fatal(err)
	}
	want := []document.Recognised{
		{Page: 3, Text: "From the layer", FromTextLayer: true},
		{Page: 5, Text: "Seen", Labels: []string{"a", "b", "c", "d", "e"}},
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("pages = %+v, want %+v", out, want)
	}
	if got["path"] != "/Users/me/Hearth/scan.pdf" || !reflect.DeepEqual(got["pages"], []any{3.0, 5.0, 7.0}) {
		t.Errorf("request = %v", got)
	}
}

func TestRecogniseRefusals(t *testing.T) {
	cases := []struct {
		name  string
		serve func(http.ResponseWriter, map[string]any)
		token string
		want  string
	}{
		{"wrong token", nil, "not-the-token", "unauthenticated"},
		{"too slow", func(w http.ResponseWriter, _ map[string]any) {
			time.Sleep(time.Second)
			_, _ = io.WriteString(w, `{}`)
		}, testToken, "deadline"},
		{"a 5 MB reply", func(w http.ResponseWriter, _ map[string]any) {
			_, _ = io.WriteString(w, `{"pages":[{"page":1,"source":"TEXT_SOURCE_RECOGNISED","text":"`+strings.Repeat("x", 5<<20)+`"}]}`)
		}, testToken, "resource_exhausted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, l := app(t, func(w http.ResponseWriter, req map[string]any) {
				if tc.serve == nil {
					t.Error("served a request with the wrong token")
					return
				}
				tc.serve(w, req)
			})
			if tc.token != testToken {
				other, err := newClient("127.0.0.1:4242", tc.token, l.dial, time.Second)
				if err != nil {
					t.Fatal(err)
				}
				c = other
			}
			out, err := c.Recognise(context.Background(), "/a/scan.png", []int{1})
			if err == nil || errors.Is(err, ErrNoApp) || !strings.Contains(err.Error(), tc.want) || out != nil {
				t.Errorf("got %v, %v; want an error with %q that is not ErrNoApp", out, err, tc.want)
			}
			if c.down.Load() {
				t.Error("a call that connected marked the client down")
			}
		})
	}
}

func TestRecogniseWhileTheAppIsBusyIsNoApp(t *testing.T) {
	c, _ := app(t, func(w http.ResponseWriter, _ map[string]any) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"code":"unavailable","message":"busy with an earlier read"}`)
	})
	if _, err := c.Recognise(context.Background(), "/a/scan.png", []int{1}); !errors.Is(err, ErrNoApp) || !c.down.Load() {
		t.Errorf("err = %v, down = %v; want ErrNoApp and the run marked down", err, c.down.Load())
	}
}

func TestRecogniseWithNoAppIsDownForTheRun(t *testing.T) {
	var dials atomic.Int32
	refused := func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	}
	c, err := newClient("127.0.0.1:4242", testToken, refused, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	run := c.ForRun()
	for range 3 {
		if _, err := run.Recognise(context.Background(), "/a/scan.pdf", []int{1}); !errors.Is(err, ErrNoApp) {
			t.Fatalf("err = %v, want ErrNoApp", err)
		}
	}
	if dials.Load() != 1 {
		t.Errorf("dials = %d, want 1: the run stops asking once the app is gone", dials.Load())
	}
	if _, err := c.ForRun().Recognise(context.Background(), "/a/scan.pdf", []int{1}); !errors.Is(err, ErrNoApp) || dials.Load() != 2 {
		t.Errorf("the next run: err = %v, dials = %d; want it to ask again", err, dials.Load())
	}
}

func TestRecogniseRefusesBeforeCalling(t *testing.T) {
	c, l := app(t, func(http.ResponseWriter, map[string]any) { t.Error("called the app") })
	for _, tc := range []struct {
		path  string
		pages []int
	}{
		{"relative/scan.pdf", []int{1}},
		{"/a/scan.pdf", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		{"/a/scan.pdf", []int{0}},
	} {
		if _, err := c.Recognise(context.Background(), tc.path, tc.pages); err == nil {
			t.Errorf("%q %v: no error", tc.path, tc.pages)
		}
	}
	if l.dials.Load() != 0 {
		t.Errorf("dials = %d, want none", l.dials.Load())
	}
}

func TestNewTakesLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:5000", "[::1]:5000"} {
		if _, err := New(addr, testToken); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
	for _, addr := range []string{"localhost:5000", "10.0.0.2:5000", "0.0.0.0:5000", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:x", "example.com:80"} {
		if _, err := New(addr, testToken); err == nil {
			t.Errorf("%s: accepted", addr)
		}
	}
	if _, err := New("127.0.0.1:5000", ""); err == nil {
		t.Error("an empty token was accepted")
	}
}
