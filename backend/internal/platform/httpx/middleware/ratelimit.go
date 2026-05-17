package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu             sync.Mutex
	visitors       map[string]*visitor
	rps            rate.Limit
	burst          int
	ttl            time.Duration
	done           chan struct{}
	// trustedProxies restricts which upstream IPs are trusted as reverse proxies
	// when reading X-Forwarded-For. When empty, any private/loopback address is
	// trusted (the previous default behaviour — preserved for backward compat).
	trustedProxies []netip.Prefix
}

// NewRateLimiter creates a RateLimiter that trusts any loopback or RFC-1918
// address as a reverse proxy (backward-compatible default). Use
// WithTrustedProxies to restrict trust to an explicit CIDR list.
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      5 * time.Minute,
		done:     make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// WithTrustedProxies returns a copy of the RateLimiter whose proxy trust is
// restricted to the given CIDR prefixes. An empty slice keeps the
// loopback/private default. Call before the limiter is put into service.
func (rl *RateLimiter) WithTrustedProxies(prefixes []netip.Prefix) *RateLimiter {
	rl.trustedProxies = prefixes
	return rl
}

// Stop terminates the background cleanup goroutine. Call during graceful
// shutdown to prevent the goroutine from leaking for the process lifetime.
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

func (rl *RateLimiter) get(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v, ok := rl.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-t.C:
			rl.mu.Lock()
			for k, v := range rl.visitors {
				if time.Since(v.lastSeen) > rl.ttl {
					delete(rl.visitors, k)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.clientIP(r)
			if !rl.get(key).Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":"too_many_requests","message":"Too many requests"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	// Only trust X-Forwarded-For when the direct connection comes from a
	// trusted reverse proxy. Accepting XFF from any RemoteAddr lets a direct
	// client spoof arbitrary source IPs and bypass per-IP rate limiting.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" && rl.isTrustedProxy(r.RemoteAddr) {
		// XFF may be a comma-separated list; the leftmost entry is the client.
		if comma := strings.IndexByte(xff, ','); comma > 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTrustedProxy returns true when addr is a trusted reverse proxy.
// When trustedProxies is non-empty, only those CIDRs are trusted.
// When empty, any loopback or RFC-1918 private address is trusted
// (backward-compatible default).
func (rl *RateLimiter) isTrustedProxy(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip, parseErr := netip.ParseAddr(host)
	if parseErr != nil {
		return false
	}
	if len(rl.trustedProxies) > 0 {
		for _, prefix := range rl.trustedProxies {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	// Default: trust loopback and private ranges (same as before).
	return ip.IsLoopback() || ip.IsPrivate()
}

