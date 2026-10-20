package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// fakeResolver answers from a table; a host it does not know is not found.
type fakeResolver map[string][]string

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	answers, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	addrs := make([]netip.Addr, 0, len(answers))
	for _, a := range answers {
		addrs = append(addrs, netip.MustParseAddr(a))
	}
	return addrs, nil
}

// reply is a canned response.
type reply struct {
	status   int
	location string
	ctype    string
	body     []byte
}

// sent is what the fake transport saw of one request.
type sent struct {
	url, method, agent, cookie string
	pin                        netip.Addr
	deadline                   time.Duration
}

// fakeTransport answers each URL from a table and records every request.
// It stands in for the pinned transport, so nothing is dialled.
type fakeTransport struct {
	replies map[string]reply
	sent    []sent
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	pin, _ := req.Context().Value(pinKey{}).(netip.Addr)
	deadline, _ := req.Context().Deadline()
	f.sent = append(f.sent, sent{
		url: req.URL.String(), method: req.Method, agent: req.Header.Get("User-Agent"),
		cookie: req.Header.Get("Cookie"), pin: pin, deadline: time.Until(deadline),
	})
	r, ok := f.replies[req.URL.String()]
	if !ok {
		return nil, errors.New("connection refused")
	}
	h := http.Header{}
	if r.location != "" {
		h.Set("Location", r.location)
	}
	if r.ctype != "" {
		h.Set("Content-Type", r.ctype)
	}
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return &http.Response{
		StatusCode: r.status, Status: http.StatusText(r.status), Header: h,
		Body: io.NopCloser(bytes.NewReader(r.body)), Request: req,
	}, nil
}

var resolver = fakeResolver{
	"example.org":       {"93.184.215.14"},
	"other.example":     {"93.184.215.15"},
	"intranet.example":  {"10.0.0.5"},
	"lan6.example":      {"2a02:8071:1:2::99"},
	"split.example":     {"93.184.215.16", "192.168.0.10"},
	"rebind.example":    {"127.0.0.1"},
	"metadata.example":  {"169.254.169.254"},
	"v6private.example": {"fd00::1"},
}

// testFetcher is a Fetcher over the fake resolver and transport whose
// machine has the interfaces given in CIDR form and no others.
func testFetcher(transport http.RoundTripper, interfaces ...string) *Fetcher {
	f := newFetcher(resolver, transport)
	f.budget = newClock().budget()
	f.interfaces = func() ([]net.Addr, error) {
		var addrs []net.Addr
		for _, cidr := range interfaces {
			ip, n, err := net.ParseCIDR(cidr)
			if err != nil {
				return nil, err
			}
			addrs = append(addrs, &net.IPNet{IP: ip, Mask: n.Mask})
		}
		return addrs, nil
	}
	return f
}

func html200(body string) reply {
	return reply{ctype: "text/html; charset=utf-8", body: []byte(body)}
}

func TestFetch(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 2<<20)
	chain := map[string]reply{}
	for i := range 5 {
		chain["https://example.org/hop"+string(rune('0'+i))] = reply{status: http.StatusFound, location: "/hop" + string(rune('1'+i))}
	}
	chain["https://example.org/hop5"] = html200("five hops")

	cases := []struct {
		name    string
		url     string
		replies map[string]reply
		want    string   // the body read; "" when refused
		urls    []string // the URLs requested, in order
		pins    []string // the address each request was pinned to
		trunc   bool
		err     string
	}{
		{
			name: "a public page", url: "https://example.org/page",
			replies: map[string]reply{"https://example.org/page": html200("hello")},
			want:    "hello", urls: []string{"https://example.org/page"}, pins: []string{"93.184.215.14"},
		},
		{
			name: "a public IP literal", url: "http://93.184.215.14/",
			replies: map[string]reply{"http://93.184.215.14/": html200("hi")},
			want:    "hi", urls: []string{"http://93.184.215.14/"}, pins: []string{"93.184.215.14"},
		},
		{name: "a host on the local network", url: "https://intranet.example/", err: "private address"},
		{name: "a host with one private address among public ones", url: "https://split.example/", err: "private address"},
		{name: "a host that resolves to loopback", url: "https://rebind.example/", err: "loopback address"},
		{name: "the metadata service by name", url: "http://metadata.example/latest/", err: "cloud metadata"},
		{name: "the metadata service by address", url: "http://169.254.169.254/latest/", err: "cloud metadata"},
		{name: "an IPv6 private address", url: "http://v6private.example/", err: "private address"},
		{name: "loopback spelled as IPv4-mapped IPv6", url: "http://[::ffff:127.0.0.1]/", err: "loopback"},
		{name: "a host that cannot be found", url: "https://nowhere.example/", err: "could not be found"},
		{name: "a port other than 80 or 443", url: "http://example.org:8080/", err: "ports 80 and 443"},
		{name: "a scheme other than http", url: "file:///etc/passwd", err: "http and https"},
		{
			name: "a redirect to a private address is refused at the hop", url: "https://example.org/go",
			replies: map[string]reply{"https://example.org/go": {status: http.StatusFound, location: "http://intranet.example/admin"}},
			urls:    []string{"https://example.org/go"}, err: "private address",
		},
		{
			name: "a redirect to a loopback literal is refused at the hop", url: "https://example.org/go",
			replies: map[string]reply{"https://example.org/go": {status: http.StatusMovedPermanently, location: "http://127.0.0.1/"}},
			urls:    []string{"https://example.org/go"}, err: "loopback",
		},
		{
			name: "a redirect to another public host is checked and followed", url: "https://example.org/go",
			replies: map[string]reply{
				"https://example.org/go":      {status: http.StatusSeeOther, location: "https://other.example/there"},
				"https://other.example/there": html200("arrived"),
			},
			want: "arrived", urls: []string{"https://example.org/go", "https://other.example/there"},
			pins: []string{"93.184.215.14", "93.184.215.15"},
		},
		{
			name: "five redirects are followed", url: "https://example.org/hop0", replies: chain, want: "five hops",
			urls: []string{"https://example.org/hop0", "https://example.org/hop1", "https://example.org/hop2", "https://example.org/hop3", "https://example.org/hop4", "https://example.org/hop5"},
		},
		{
			name: "a sixth redirect is refused", url: "https://example.org/loop",
			replies: map[string]reply{"https://example.org/loop": {status: http.StatusTemporaryRedirect, location: "/loop"}},
			err:     "more than 5 times",
		},
		{
			name: "a redirect that says nowhere", url: "https://example.org/go",
			replies: map[string]reply{"https://example.org/go": {status: http.StatusFound}}, err: "without saying where",
		},
		{
			name: "a 2 MiB body is cut at 1 MiB", url: "https://example.org/big",
			replies: map[string]reply{"https://example.org/big": {ctype: "text/plain", body: big}},
			want:    string(big[:maxBody]), trunc: true,
		},
		{
			name: "an error status", url: "https://example.org/missing",
			replies: map[string]reply{"https://example.org/missing": {status: http.StatusNotFound}}, err: "answered Not Found",
		},
		{name: "a host that does not answer", url: "https://example.org/down", err: "could not be reached"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &fakeTransport{replies: tc.replies}
			f := testFetcher(transport)
			page, err := f.Fetch(context.Background(), tc.url)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if string(page.Body) != tc.want || page.Truncated != tc.trunc {
				t.Fatalf("body %d bytes, truncated %v; want %d bytes, %v", len(page.Body), page.Truncated, len(tc.want), tc.trunc)
			}
			if tc.urls != nil {
				var got []string
				for _, s := range transport.sent {
					got = append(got, s.url)
				}
				if strings.Join(got, " ") != strings.Join(tc.urls, " ") {
					t.Fatalf("requested %v, want %v", got, tc.urls)
				}
			}
			for i, s := range transport.sent {
				if i < len(tc.pins) && s.pin.String() != tc.pins[i] {
					t.Errorf("request %d pinned to %s, want %s", i, s.pin, tc.pins[i])
				}
				if !s.pin.IsValid() || checkAddr(s.pin) != nil {
					t.Errorf("request %d went out pinned to %v", i, s.pin)
				}
				if s.method != http.MethodGet || s.agent != userAgent || s.cookie != "" {
					t.Errorf("request %d: %s, agent %q, cookie %q", i, s.method, s.agent, s.cookie)
				}
				if s.deadline <= 0 || s.deadline > fetchTimeout {
					t.Errorf("request %d has %s left, want at most %s", i, s.deadline, fetchTimeout)
				}
			}
		})
	}
}

func TestFetch_LocalNetworks(t *testing.T) {
	ok := map[string]reply{
		"http://[2a02:8071:1:2::1]/": html200("router"),
		"http://81.2.69.1/":          html200("lan"),
		"https://example.org/":       html200("public"),
		"https://lan6.example/":      html200("nas"),
	}
	site := "2a02:8071:1:2::abcd/64" // this Mac's global IPv6 address on its site's /64
	cases := []struct {
		name       string
		url        string
		interfaces []string
		err        string
	}{
		{"a global IPv6 address inside a local /64", "http://[2a02:8071:1:2::1]/", []string{"127.0.0.1/8", site}, "on this Mac's own network"},
		{"the same address with no interface on its network", "http://[2a02:8071:1:2::1]/", []string{"127.0.0.1/8", "2a02:8071:9:9::1/64"}, ""},
		{"a public IPv4 prefix a LAN uses", "http://81.2.69.1/", []string{"81.2.69.10/24"}, "on this Mac's own network"},
		{"this Mac's own address", "http://81.2.69.10/", []string{"81.2.69.10/24"}, "on this Mac's own network"},
		{"a name that resolves onto a local network", "https://lan6.example/", []string{site}, "on this Mac's own network"},
		{"a public page elsewhere", "https://example.org/", []string{site, "81.2.69.10/24"}, ""},
		{"an interface with a /0 is only its own address", "https://example.org/", []string{"93.184.215.99/0"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &fakeTransport{replies: ok}
			_, err := testFetcher(transport, tc.interfaces...).Fetch(context.Background(), tc.url)
			if tc.err == "" {
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("err = %v, want %q", err, tc.err)
			}
			if len(transport.sent) != 0 {
				t.Fatalf("a request went out: %v", transport.sent)
			}
		})
	}
	t.Run("interfaces that cannot be listed", func(t *testing.T) {
		f := testFetcher(&fakeTransport{replies: ok})
		f.interfaces = func() ([]net.Addr, error) { return nil, errors.New("no interfaces") }
		if _, err := f.Fetch(context.Background(), "https://example.org/"); err == nil || !strings.Contains(err.Error(), "could not be listed") {
			t.Fatalf("err = %v, want a refusal", err)
		}
	})
}

func TestFetcher_KeepsNoCookies(t *testing.T) {
	if f := NewFetcher(); f.client.Jar != nil {
		t.Fatal("the fetcher has a cookie jar")
	}
	tr, ok := NewFetcher().client.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || !tr.DisableKeepAlives {
		t.Fatal("the transport may use a proxy or reuse a connection")
	}
}

func TestDialPinned(t *testing.T) {
	var dialled string
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialled = addr
		return nil, errors.New("not dialled in a test")
	}
	pinned := func(a string) context.Context {
		return context.WithValue(context.Background(), pinKey{}, netip.MustParseAddr(a))
	}
	cases := []struct {
		name   string
		ctx    context.Context
		addr   string
		dialed string
		err    string
	}{
		{"the checked address, not the name", pinned("93.184.215.14"), "example.org:443", "93.184.215.14:443", "not dialled"},
		{"an IPv6 address", pinned("2606:2800::1"), "example.org:80", "[2606:2800::1]:80", "not dialled"},
		{"no checked address", context.Background(), "example.org:443", "", "no checked address"},
		{"a private pin", pinned("10.0.0.1"), "example.org:443", "", "private address"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dialled = ""
			_, err := dialPinned(tc.ctx, dial, "tcp", tc.addr)
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("err = %v, want %q", err, tc.err)
			}
			if dialled != tc.dialed {
				t.Fatalf("dialled %q, want %q", dialled, tc.dialed)
			}
		})
	}
}
