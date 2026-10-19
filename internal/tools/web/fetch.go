package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

const (
	fetchTimeout = 20 * time.Second
	dialTimeout  = 10 * time.Second
	maxRedirects = 5
	maxBody      = 1 << 20
	userAgent    = "Hearth/1.0 (+https://github.com/rfizzle/hearth)"
)

// Resolver looks a host name up. *net.Resolver is one.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Fetcher GETs public pages. Each hop's host is looked up and every address
// it has checked before the request, the connection goes to the address that
// was checked and no other, redirects are followed by hand so each is
// checked the same way, and no cookie is kept. It is safe for concurrent use.
type Fetcher struct {
	resolver Resolver
	client   *http.Client
	// interfaces lists this machine's interface addresses with their
	// masks; net.InterfaceAddrs outside tests.
	interfaces func() ([]net.Addr, error)
}

// Page is a fetched response: the URL it was finally read from, its content
// type, and its body cut at 1 MiB, with Truncated set when it was.
type Page struct {
	URL         string
	ContentType string
	Body        []byte
	Truncated   bool
}

// pinKey is the context key the checked address travels to the dial under.
type pinKey struct{}

// NewFetcher is a Fetcher over the system resolver and the network.
func NewFetcher() *Fetcher {
	return newFetcher(net.DefaultResolver, pinnedTransport())
}

func newFetcher(resolver Resolver, transport http.RoundTripper) *Fetcher {
	return &Fetcher{
		resolver:   resolver,
		interfaces: net.InterfaceAddrs,
		client: &http.Client{
			Transport: transport,
			// Followed in Fetch, where each hop is checked; the client must
			// never follow one on its own.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// pinnedTransport dials only the address a hop was checked against. The
// proxy is off because a proxy would carry the connection past that check,
// and keep-alives are off so a connection is never reused for a request it
// was not dialled for.
func pinnedTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: dialTimeout}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialPinned(ctx, dialer.DialContext, network, addr)
		},
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   dialTimeout,
		ResponseHeaderTimeout: dialTimeout,
	}
}

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// dialPinned dials the address the request's hop was checked against, on
// the port the URL named, in place of whatever the host name would resolve
// to now. With no checked address on the context it refuses, so a request
// that skipped the check can never connect.
func dialPinned(ctx context.Context, dial dialFunc, network, addr string) (net.Conn, error) {
	pin, ok := ctx.Value(pinKey{}).(netip.Addr)
	if !ok {
		return nil, errors.New("web.fetch: no checked address to connect to")
	}
	if err := checkAddr(pin); err != nil {
		return nil, err
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	return dial(ctx, network, net.JoinHostPort(pin.Unmap().String(), port))
}

// Fetch GETs rawURL, following at most five redirects, within 20 seconds.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	u, err := url.Parse(rawURL)
	if err != nil {
		return Page{}, fmt.Errorf("%q is not a URL", rawURL)
	}
	for hop := 0; ; hop++ {
		addr, err := f.checkHop(ctx, u)
		if err != nil {
			return Page{}, err
		}
		resp, err := f.get(ctx, u, addr)
		if err != nil {
			return Page{}, err
		}
		next, err := redirectTarget(resp, u)
		if err == nil && next == nil {
			page, err := readPage(resp, u)
			resp.Body.Close()
			return page, err
		}
		resp.Body.Close()
		if err != nil {
			return Page{}, err
		}
		if hop == maxRedirects {
			return Page{}, fmt.Errorf("%s redirected more than %d times; web.fetch stopped", rawURL, maxRedirects)
		}
		u = next
	}
}

// checkHop is the address policy for one hop: the URL first, then every
// address its host has. One address that is not public, or that is on one
// of this machine's own networks, refuses the hop, because the dial could
// otherwise be steered to it. It returns the address the hop is pinned to.
func (f *Fetcher) checkHop(ctx context.Context, u *url.URL) (netip.Addr, error) {
	host, err := checkURL(u)
	if err != nil {
		return netip.Addr{}, err
	}
	addrs := []netip.Addr{}
	if literal, err := netip.ParseAddr(host); err == nil {
		addrs = append(addrs, literal)
	} else if addrs, err = f.resolver.LookupNetIP(ctx, "ip", host); err != nil || len(addrs) == 0 {
		return netip.Addr{}, fmt.Errorf("%s: the host %s could not be found", u.Redacted(), host)
	}
	local, err := f.localNetworks()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%s: this Mac's own networks could not be listed, so the address cannot be checked", u.Redacted())
	}
	for _, a := range addrs {
		err := checkAddr(a)
		if err == nil {
			err = checkLocal(a, local)
		}
		if err != nil {
			return netip.Addr{}, fmt.Errorf("%s: %w", u.Redacted(), err)
		}
	}
	return addrs[0], nil
}

// localNetworks is the prefix of every interface on this machine. An
// interface with no usable mask, or a /0, counts as its own address alone,
// so a stray default route cannot refuse the whole internet.
func (f *Fetcher) localNetworks() ([]netip.Prefix, error) {
	addrs, err := f.interfaces()
	if err != nil {
		return nil, err
	}
	var nets []netip.Prefix
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		ones, bits := ipnet.Mask.Size()
		if bits == 128 && ip.Is4() {
			ones -= 96
		}
		if ones <= 0 || ones > ip.BitLen() {
			ones = ip.BitLen()
		}
		nets = append(nets, netip.PrefixFrom(ip, ones).Masked())
	}
	return nets, nil
}

// checkLocal refuses an address on one of local's networks. The ranges in
// classify cannot see these: on an IPv6 network the router, the printer and
// this Mac have global addresses under the site's prefix, and a LAN may use
// public IPv4 space, yet a page served there is not on the public internet.
func checkLocal(a netip.Addr, local []netip.Prefix) error {
	for _, p := range local {
		if p.Contains(a.Unmap()) {
			return fmt.Errorf("%s is on this Mac's own network, and web.fetch reads public pages only", a)
		}
	}
	return nil
}

func (f *Fetcher) get(ctx context.Context, u *url.URL, pin netip.Addr) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.WithValue(ctx, pinKey{}, pin), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u.Redacted(), err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.5")
	resp, err := f.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s did not answer within %s", u.Redacted(), fetchTimeout)
		}
		return nil, fmt.Errorf("%s could not be reached", u.Redacted())
	}
	return resp, nil
}

// redirectTarget is where resp redirects to, resolved against from, or nil
// when it is not a redirect. A redirect with no usable Location is an error.
func redirectTarget(resp *http.Response, from *url.URL) (*url.URL, error) {
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return nil, nil
	}
	next, err := from.Parse(resp.Header.Get("Location"))
	if err != nil || resp.Header.Get("Location") == "" {
		return nil, fmt.Errorf("%s redirected without saying where to", from.Redacted())
	}
	return next, nil
}

func readPage(resp *http.Response, u *url.URL) (Page, error) {
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Page{}, fmt.Errorf("%s answered %s", u.Redacted(), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return Page{}, fmt.Errorf("%s: the page stopped part way: %w", u.Redacted(), err)
	}
	page := Page{URL: u.String(), ContentType: resp.Header.Get("Content-Type"), Body: body}
	if len(body) > maxBody {
		page.Body, page.Truncated = body[:maxBody], true
	}
	return page, nil
}
