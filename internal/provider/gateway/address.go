package gateway

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ErrAddress is what a refused address wraps: not a web address, one that
// carries the key, or plain http to another machine.
var ErrAddress = errors.New("that address is refused")

// ErrList is what a service that could not list its models wraps: it did
// not answer, refused the key, or did not answer as a model list.
var ErrList = errors.New("the service did not list its models")

// refusal is words for the person that match the sentinel for what went
// wrong. None holds the key or what the service sent.
type refusal struct {
	text string
	kind error
}

func (r refusal) Error() string        { return r.text }
func (r refusal) Is(target error) bool { return target == r.kind }

// CheckBaseURL returns the address with no trailing slash, and whether it is
// on this Mac. It refuses anything but http or https, and plain http to
// another machine, where the key could be read on the way. A refusal
// wraps ErrAddress.
func CheckBaseURL(raw string) (base string, local bool, err error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", false, refusal{"enter the service's address, the field was empty", ErrAddress}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false, refusal{"the address should look like https://example.com/v1", ErrAddress}
	}
	// The address is shown and returned; it is not a secret, so a key in it
	// would leave the core.
	// See docs/model.md#providers-and-models
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", false, refusal{"put the key in the key field, not in the address", ErrAddress}
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	local = host == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme == "http" && !local {
		return "", false, refusal{"use an https address; over http the key could be read on the way", ErrAddress}
	}
	return raw, local, nil
}

// HostMoved reports whether two addresses are on different hosts, so a
// stored key must not follow the service to the new one.
func HostMoved(from, to string) bool {
	a, errA := url.Parse(from)
	b, errB := url.Parse(to)
	return errA != nil || errB != nil || a.Host != b.Host
}
