package middleware

import (
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
)

// maxBuckets bounds limiter memory. Once full, new clients share overflowKey's bucket until
// the periodic cleanup frees space.
const (
	maxBuckets  = 100_000
	overflowKey = "overflow"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter implements an in-memory per-IP token-bucket rate limiter with automatic eviction.
type RateLimiter struct {
	rps            float64
	burst          float64
	trustedProxies []netip.Prefix
	log            *logger.Logger

	mu      sync.Mutex
	clients map[string]*bucket
	done    chan struct{}
	once    sync.Once
}

// NewRateLimiter creates a RateLimiter and starts background cleanup of stale IP buckets.
// Forwarding headers are honored only when the direct peer is inside trustedProxies.
func NewRateLimiter(rps float64, burst int, trustedProxies []netip.Prefix, log *logger.Logger) (*RateLimiter, error) {
	if rps <= 0 || math.IsNaN(rps) || math.IsInf(rps, 0) {
		return nil, errors.New("rate limiter rps must be a positive finite number")
	}
	if burst < 1 {
		return nil, errors.New("rate limiter burst must be at least 1")
	}
	rl := &RateLimiter{
		rps:            rps,
		burst:          float64(burst),
		trustedProxies: trustedProxies,
		log:            log,
		clients:        make(map[string]*bucket),
		done:           make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl, nil
}

// Middleware returns an http.Handler middleware enforcing the limiter's rate policy.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r, rl.trustedProxies)
		if !rl.allow(ip) {
			response.RateLimitExceeded(w, r, rl.log)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(key string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.clients[key]
	if !exists && len(rl.clients) >= maxBuckets {
		key = overflowKey
		b, exists = rl.clients[key]
	}
	if !exists {
		rl.clients[key] = &bucket{
			tokens:   rl.burst - 1,
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.lastSeen = now
	b.tokens = math.Min(rl.burst, b.tokens+elapsed*rl.rps)
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-3 * time.Minute)
			rl.mu.Lock()
			for ip, b := range rl.clients {
				if b.lastSeen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Close terminates the background eviction goroutine.
func (rl *RateLimiter) Close() {
	rl.once.Do(func() {
		close(rl.done)
	})
}

// extractClientIP returns the direct peer address unless that peer is a trusted proxy. For a trusted
// peer it walks X-Forwarded-For from the right, skipping trusted proxies, and returns the first
// address a trusted proxy appended. Entries left of that are client-supplied and never used.
func extractClientIP(r *http.Request, trustedProxies []netip.Prefix) string {
	peer := remoteAddr(r)
	if !peer.IsValid() {
		return r.RemoteAddr
	}
	if !isTrusted(peer, trustedProxies) {
		return peer.String()
	}

	hops := r.Header.Values("X-Forwarded-For")
	var entries []string
	for _, h := range hops {
		entries = append(entries, strings.Split(h, ",")...)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(entries[i]))
		if err != nil {
			break
		}
		addr = addr.Unmap()
		if !isTrusted(addr, trustedProxies) {
			return addr.String()
		}
	}

	if len(entries) == 0 {
		if xri, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
			return xri.Unmap().String()
		}
	}
	return peer.String()
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
