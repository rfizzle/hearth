package web

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// addrClass is what kind of network an address belongs to. Only public is
// fetched.
type addrClass string

const (
	classPublic      addrClass = "public"
	classLoopback    addrClass = "loopback"
	classPrivate     addrClass = "private"
	classLinkLocal   addrClass = "link-local"
	classCGNAT       addrClass = "carrier-grade NAT"
	classMetadata    addrClass = "cloud metadata"
	classUnspecified addrClass = "unspecified"
	classMulticast   addrClass = "multicast"
	classReserved    addrClass = "reserved"
)

// classify is the class of a. An IPv6 address that carries an IPv4 address
// (mapped, compatible, NAT64 or 6to4) is judged by the IPv4 address it
// reaches, so 127.0.0.1 cannot be fetched by spelling it in IPv6. Every
// IPv6 address outside global unicast is reserved: an address the policy
// does not understand is never public.
func classify(a netip.Addr) addrClass {
	a = a.Unmap()
	// The ranges that are not public, by class, as prefixes separated by
	// spaces. The first range holding an address decides its class, so the
	// cloud credential services come before the link-local and carrier-grade
	// NAT ranges they sit in, and are named when refused.
	ranges := [...]struct {
		class    addrClass
		prefixes string
	}{
		{classMetadata, "169.254.169.254/32 169.254.170.0/24 100.100.100.200/32 fd00:ec2::254/128"},
		{classUnspecified, "0.0.0.0/32 ::/128"},
		{classLoopback, "127.0.0.0/8 ::1/128"},
		{classPrivate, "10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 fc00::/7"},
		{classLinkLocal, "169.254.0.0/16 fe80::/10"},
		{classCGNAT, "100.64.0.0/10"},
		{classMulticast, "224.0.0.0/4 ff00::/8"},
		// Documentation, benchmarking, Teredo, broadcast and the other
		// special-purpose blocks: not addresses a public page lives at.
		{classReserved, "0.0.0.0/8 192.0.0.0/24 192.0.2.0/24 198.18.0.0/15 198.51.100.0/24 203.0.113.0/24 240.0.0.0/4 2001::/23 2001:db8::/32"},
	}
	for _, r := range ranges {
		for _, p := range strings.Fields(r.prefixes) {
			if netip.MustParsePrefix(p).Contains(a) {
				return r.class
			}
		}
	}
	if v4, ok := embedded4(a); ok {
		return classify(v4)
	}
	if a.Is6() && !netip.MustParsePrefix("2000::/3").Contains(a) {
		return classReserved
	}
	return classPublic
}

// embedded4 is the IPv4 address an IPv6 address reaches through a
// translation or tunnel: IPv4-compatible (::a.b.c.d), NAT64 (64:ff9b::/96,
// and 64:ff9b:1::/48 taken at its last four bytes) and 6to4 (2002::/16).
func embedded4(a netip.Addr) (netip.Addr, bool) {
	if !a.Is6() {
		return netip.Addr{}, false
	}
	b := a.As16()
	switch {
	case netip.MustParsePrefix("::/96").Contains(a),
		netip.MustParsePrefix("64:ff9b::/96").Contains(a),
		netip.MustParsePrefix("64:ff9b:1::/48").Contains(a):
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	case netip.MustParsePrefix("2002::/16").Contains(a):
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	}
	return netip.Addr{}, false
}

// checkAddr refuses every address that is not public.
func checkAddr(a netip.Addr) error {
	if class := classify(a); class != classPublic {
		return fmt.Errorf("%s is a %s address, and web.fetch reads public pages only", a, class)
	}
	return nil
}

// checkURL refuses a URL web.fetch does not fetch, before any lookup: a
// scheme other than http or https, a port other than 80 or 443, a user name
// or password, or a host with no name or with an IPv6 zone. It returns the
// host to resolve.
func checkURL(u *url.URL) (string, error) {
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%s: web.fetch reads http and https pages only", u.Redacted())
	}
	if u.User != nil {
		return "", fmt.Errorf("%s: a URL with a user name or password is not fetched", u.Redacted())
	}
	host := u.Hostname()
	if host == "" || strings.Contains(host, "%") {
		return "", fmt.Errorf("%s: the URL has no host web.fetch can use", u.Redacted())
	}
	if p := u.Port(); p != "" {
		if port, err := strconv.Atoi(p); err != nil || (port != 80 && port != 443) {
			return "", fmt.Errorf("%s: web.fetch uses ports 80 and 443 only", u.Redacted())
		}
	}
	return host, nil
}
