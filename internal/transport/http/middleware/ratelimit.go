package middleware

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter implements an in-memory per-IP token-bucket rate limiter with automatic eviction.
type RateLimiter struct {
	rps        float64
	burst      float64
	trustProxy bool
	log        *logger.Logger

	mu      sync.Mutex
	clients map[string]*bucket
	done    chan struct{}
	once    sync.Once
}

// NewRateLimiter creates a RateLimiter and starts background cleanup of stale IP buckets.
func NewRateLimiter(rps float64, burst int, trustProxy bool, log *logger.Logger) *RateLimiter {
	if rps <= 0 {
		rps = 10
	}
	if burst < 1 {
		burst = 20
	}
	rl := &RateLimiter{
		rps:        rps,
		burst:      float64(burst),
		trustProxy: trustProxy,
		log:        log,
		clients:    make(map[string]*bucket),
		done:       make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Middleware returns an http.Handler middleware enforcing the limiter's rate policy.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r, rl.trustProxy)
		if !rl.allow(ip) {
			response.RateLimitExceeded(w, r, rl.log)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.clients[ip]
	if !exists {
		rl.clients[ip] = &bucket{
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

func extractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
				return ip.String()
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return ip.String()
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
