package push

import (
	"net/netip"
	"testing"
)

// isPublic refuses every address the server must not reach with a browser-given endpoint (threat
// model T44), also when an IPv6 form carries a private IPv4 inside it.
func TestIsPublic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		want bool
	}{
		{"203.0.113.10", true},
		{"8.8.8.8", true},
		{"2606:4700::1111", true},
		{"64:ff9b::808:808", true}, // NAT64 of 8.8.8.8
		{"2002:808:808::1", true},  // 6to4 of 8.8.8.8
		{"127.0.0.1", false},
		{"10.0.0.5", false},
		{"192.0.0.8", false},         // IETF protocol assignments
		{"198.18.0.1", false},        // benchmarking
		{"198.19.255.255", false},    // benchmarking
		{"240.0.0.1", false},         // reserved
		{"255.255.255.255", false},   // limited broadcast
		{"64:ff9b::a00:5", false},    // NAT64 of 10.0.0.5
		{"64:ff9b::7f00:1", false},   // NAT64 of 127.0.0.1
		{"64:ff9b:1::a00:5", false},  // local-use NAT64 of 10.0.0.5
		{"64:ff9b:1::808:808", true}, // local-use NAT64 of 8.8.8.8
		{"2002:a00:5::1", false},     // 6to4 of 10.0.0.5
		{"2002:a9fe:a9fe::1", false}, // 6to4 of 169.254.169.254
		{"::a00:5", false},           // IPv4-compatible 10.0.0.5
		{"::7f00:1", false},          // IPv4-compatible 127.0.0.1
		{"::808:808", true},          // IPv4-compatible 8.8.8.8
		{"::1", false},               // loopback, inside ::/96
		{"::", false},                // unspecified, inside ::/96
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			if got := isPublic(netip.MustParseAddr(tc.addr)); got != tc.want {
				t.Errorf("isPublic(%s) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}
