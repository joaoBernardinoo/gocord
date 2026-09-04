package httpapp

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a per-key token bucket. Keys are client addresses, so the
// number of live buckets is bounded by sweeping idle ones.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens replenished per second
	burst   float64 // maximum tokens held
	now     func() time.Time
}

type bucket struct {
	tokens float64
	seen   time.Time
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		now:     time.Now,
	}
}

// allow consumes one token for key, reporting whether the request may proceed.
// A non-positive rate disables limiting entirely.
func (l *rateLimiter) allow(key string) bool {
	if l.rate <= 0 {
		return true
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		l.sweepLocked(now)
		b = &bucket{tokens: l.burst, seen: now}
		l.buckets[key] = b
	} else {
		b.tokens += now.Sub(b.seen).Seconds() * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.seen = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweepLocked discards buckets that have sat idle long enough to have fully
// refilled, since those are indistinguishable from a fresh bucket. Callers must
// hold mu.
func (l *rateLimiter) sweepLocked(now time.Time) {
	if l.rate <= 0 {
		return
	}
	idle := time.Duration(l.burst/l.rate*float64(time.Second)) + time.Minute
	for key, b := range l.buckets {
		if now.Sub(b.seen) > idle {
			delete(l.buckets, key)
		}
	}
}

// clientKey identifies the caller for rate-limiting purposes.
//
// When trustProxy is set, the last X-Forwarded-For entry is used: a trusted
// reverse proxy appends the address it actually saw, so the rightmost value is
// the only one a client cannot forge. Without trustProxy the header is ignored
// entirely, because any client could otherwise mint an unlimited number of
// distinct keys.
func clientKey(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if candidate := strings.TrimSpace(parts[len(parts)-1]); candidate != "" {
				return candidate
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
