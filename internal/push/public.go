package push

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// A push endpoint comes from the browser, so the server must only ever reach the public internet
// with it (threat model T44): Check refuses an IP literal and a host with no public address, and
// every send dials through publicOnlyDial, which checks the address the connection actually goes
// to, so a host that resolves elsewhere at send time (DNS rebinding) is refused too. Redirects are
// never followed: a public push service could otherwise send the POST on to a private host.

const (
	// lookupTimeout bounds the DNS lookup Subscribe makes.
	lookupTimeout = 5 * time.Second
	// dialTimeout and tlsTimeout bound the connection part of one send.
	dialTimeout = 10 * time.Second
	tlsTimeout  = 10 * time.Second
)

var (
	errNotPublic  = errors.New("push endpoint address is not public")
	errNoRedirect = errors.New("push endpoint redirected")
)

// nonPublicPrefixes are the ranges netip's own predicates do not cover.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),          // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),      // carrier-grade NAT (RFC 6598)
	netip.MustParsePrefix("192.0.0.0/24"),       // IETF protocol assignments (RFC 6890)
	netip.MustParsePrefix("198.18.0.0/15"),      // benchmarking (RFC 2544)
	netip.MustParsePrefix("240.0.0.0/4"),        // reserved
	netip.MustParsePrefix("255.255.255.255/32"), // limited broadcast
	// Local-use NAT64 (RFC 8215): where the IPv4 sits depends on the network's own prefix length
	// (RFC 6052), so it cannot be checked again; no push service lives there.
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

// embeddingPrefixes are the IPv6 ranges whose address carries an IPv4 address, and where it sits:
// the IPv4 is checked again, so a private one cannot hide inside them.
var embeddingPrefixes = []struct {
	prefix netip.Prefix
	from   int // the first of the four bytes that hold the IPv4 address
}{
	{netip.MustParsePrefix("64:ff9b::/96"), 12}, // NAT64 (RFC 6052)
	{netip.MustParsePrefix("2002::/16"), 2},     // 6to4 (RFC 3056)
	{netip.MustParsePrefix("::/96"), 12},        // IPv4-compatible (deprecated, RFC 4291)
}

// isPublic reports whether a is an address the server may connect to: not loopback, private
// (RFC 1918, IPv6 ULA), link-local, carrier-grade NAT, reserved, unspecified or multicast, also
// when it is written as an IPv4-mapped address or carried inside a well-known NAT64, 6to4 or
// IPv4-compatible IPv6 address (local-use NAT64 is refused whole).
func isPublic(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsUnspecified() ||
		a.IsMulticast() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() {
		return false
	}
	for _, p := range nonPublicPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	if a.Is6() {
		b := a.As16()
		for _, e := range embeddingPrefixes {
			if e.prefix.Contains(a) {
				return isPublic(netip.AddrFrom4([4]byte(b[e.from : e.from+4])))
			}
		}
	}
	return true
}

// publicHost reports whether u names a host by name (not an IP literal) that resolves to at
// least one public address. The send-time dial refuses any non-public address the name also has.
func publicHost(ctx context.Context, r Resolver, u *url.URL) bool {
	host := u.Hostname()
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	found, err := r.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return false
	}
	for _, a := range found {
		if isPublic(a) {
			return true
		}
	}
	return false
}

// publicOnlyDial is the dialer's Control hook: it runs after DNS, on the address being dialled.
func publicOnlyDial(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !isPublic(ap.Addr()) {
		return errNotPublic
	}
	return nil
}

// refuseRedirect is the client's CheckRedirect: a push service answers, it never redirects.
func refuseRedirect(*http.Request, []*http.Request) error { return errNoRedirect }

// publicOnlyClient is the default push client: it dials only public addresses and uses no proxy
// (a proxy would be dialled in place of the push service, and none is configured).
func publicOnlyClient() *http.Client {
	dialer := &net.Dialer{Timeout: dialTimeout, Control: publicOnlyDial}
	return &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: tlsTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}
