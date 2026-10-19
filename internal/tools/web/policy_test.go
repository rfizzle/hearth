package web

import (
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		addr string
		want addrClass
	}{
		{"93.184.215.14", classPublic},
		{"2606:2800:21f:cb07:6820:80da:af6b:8b2c", classPublic},
		{"127.0.0.1", classLoopback},
		{"127.8.9.10", classLoopback},
		{"::1", classLoopback},
		{"::ffff:127.0.0.1", classLoopback},
		{"10.1.2.3", classPrivate},
		{"172.16.0.1", classPrivate},
		{"172.31.255.255", classPrivate},
		{"192.168.1.1", classPrivate},
		{"fd12:3456::1", classPrivate},
		{"169.254.10.1", classLinkLocal},
		{"fe80::1", classLinkLocal},
		{"100.64.0.1", classCGNAT},
		{"100.127.255.254", classCGNAT},
		{"169.254.169.254", classMetadata},
		{"169.254.170.2", classMetadata},
		{"100.100.100.200", classMetadata},
		{"fd00:ec2::254", classMetadata},
		{"::ffff:169.254.169.254", classMetadata},
		{"64:ff9b::a9fe:a9fe", classMetadata}, // NAT64 of 169.254.169.254
		{"64:ff9b::7f00:1", classLoopback},    // NAT64 of 127.0.0.1
		{"64:ff9b:1::a00:1", classPrivate},    // local-use NAT64 of 10.0.0.1
		{"2002:7f00:1::", classLoopback},      // 6to4 of 127.0.0.1
		{"2002:5db8:d70e::1", classPublic},    // 6to4 of a public address
		{"::7f00:1", classLoopback},           // IPv4-compatible 127.0.0.1
		{"0.0.0.0", classUnspecified},
		{"::", classUnspecified},
		{"0.1.2.3", classReserved},
		{"224.0.0.1", classMulticast},
		{"ff02::1", classMulticast},
		{"255.255.255.255", classReserved},
		{"240.0.0.1", classReserved},
		{"192.0.2.1", classReserved},
		{"2001:db8::1", classReserved},
		{"2001:0:4136:e378::1", classReserved}, // Teredo
		{"100::1", classReserved},              // outside global unicast
		{"172.32.0.1", classPublic},
		{"100.128.0.1", classPublic},
	}
	for _, tc := range cases {
		a := netip.MustParseAddr(tc.addr)
		if got := classify(a); got != tc.want {
			t.Errorf("classify(%s) = %s, want %s", tc.addr, got, tc.want)
		}
		if err := checkAddr(a); (err == nil) != (tc.want == classPublic) {
			t.Errorf("checkAddr(%s) = %v", tc.addr, err)
		}
	}
}

func TestCheckURL(t *testing.T) {
	cases := []struct {
		raw, host, err string
	}{
		{"https://example.org/page", "example.org", ""},
		{"http://example.org", "example.org", ""},
		{"https://example.org:443/", "example.org", ""},
		{"http://example.org:80/", "example.org", ""},
		{"https://[2606:2800::1]/", "2606:2800::1", ""},
		{"ftp://example.org/", "", "http and https"},
		{"file:///etc/passwd", "", "http and https"},
		{"javascript:alert(1)", "", "http and https"},
		{"https://example.org:8080/", "", "ports 80 and 443"},
		{"https://example.org:22/", "", "ports 80 and 443"},
		{"https://user:pass@example.org/", "", "user name or password"},
		{"https:///path", "", "no host"},
		{"http://[fe80::1%25en0]/", "", "no host"},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		host, err := checkURL(u)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("checkURL(%s) = %v, want %q", tc.raw, err, tc.err)
			}
			if err != nil && strings.Contains(err.Error(), "pass@") {
				t.Errorf("checkURL(%s) repeats the password: %v", tc.raw, err)
			}
			continue
		}
		if err != nil || host != tc.host {
			t.Errorf("checkURL(%s) = %q, %v, want %q", tc.raw, host, err, tc.host)
		}
	}
}
