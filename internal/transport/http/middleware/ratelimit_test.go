package middleware

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestExtractClientIPHonorsOnlyTrustedProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}

	tests := []struct {
		name    string
		remote  string
		xff     string
		trusted []netip.Prefix
		want    string
	}{
		{"no proxies configured ignores header", "203.0.113.10:5000", "198.51.100.1", nil, "203.0.113.10"},
		{"untrusted peer ignores header", "203.0.113.10:5000", "198.51.100.1", trusted, "203.0.113.10"},
		{"trusted peer uses appended hop", "10.0.0.5:5000", "198.51.100.1", trusted, "198.51.100.1"},
		{"client-supplied leftmost hop is ignored", "10.0.0.5:5000", "1.2.3.4, 198.51.100.1", trusted, "198.51.100.1"},
		{"chained trusted proxies are skipped", "10.0.0.5:5000", "198.51.100.1, 10.0.0.9", trusted, "198.51.100.1"},
		{"garbage hop falls back to peer", "10.0.0.5:5000", "not-an-ip", trusted, "10.0.0.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.xff)
			if got := extractClientIP(r, tc.trusted); got != tc.want {
				t.Fatalf("extractClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewRateLimiterRejectsNonPositiveLimits(t *testing.T) {
	if _, err := NewRateLimiter(0, 5, nil, nil); err == nil {
		t.Fatal("expected error for rps=0")
	}
	if _, err := NewRateLimiter(2, 0, nil, nil); err == nil {
		t.Fatal("expected error for burst=0")
	}
}
