package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter tracks failed authentication attempts per IP to mitigate brute-force attacks.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*ipRecord

	// MaxAttempts is the number of failed auth attempts allowed within the window.
	MaxAttempts int
	// Window is the time window for counting failures.
	Window time.Duration
	// Cooldown is how long an IP is blocked after exceeding MaxAttempts.
	Cooldown time.Duration
}

type ipRecord struct {
	failures  []time.Time
	blockedAt time.Time
}

// NewRateLimiter creates a RateLimiter with sensible defaults for auth protection.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		attempts:    make(map[string]*ipRecord),
		MaxAttempts: 10,
		Window:      5 * time.Minute,
		Cooldown:    15 * time.Minute,
	}
	go rl.cleanup()
	return rl
}

// IsBlocked reports whether the given IP is currently rate-limited.
func (rl *RateLimiter) IsBlocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rec, ok := rl.attempts[ip]
	if !ok {
		return false
	}
	if !rec.blockedAt.IsZero() && time.Since(rec.blockedAt) < rl.Cooldown {
		return true
	}
	return false
}

// RecordFailure records a failed auth attempt for the IP.
// Returns true if the IP is now blocked.
func (rl *RateLimiter) RecordFailure(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rec, ok := rl.attempts[ip]
	if !ok {
		rec = &ipRecord{}
		rl.attempts[ip] = rec
	}
	// If already blocked and cooldown hasn't expired, stay blocked.
	if !rec.blockedAt.IsZero() && now.Sub(rec.blockedAt) < rl.Cooldown {
		return true
	}
	// Reset if cooldown has expired.
	if !rec.blockedAt.IsZero() {
		rec.blockedAt = time.Time{}
		rec.failures = nil
	}
	// Prune old failures outside the window.
	cutoff := now.Add(-rl.Window)
	pruned := rec.failures[:0]
	for _, t := range rec.failures {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	rec.failures = append(pruned, now)
	if len(rec.failures) >= rl.MaxAttempts {
		rec.blockedAt = now
		return true
	}
	return false
}

// Reset clears tracking for a specific IP (e.g. after successful auth).
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// cleanup periodically removes stale records.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, rec := range rl.attempts {
			if !rec.blockedAt.IsZero() && now.Sub(rec.blockedAt) >= rl.Cooldown {
				delete(rl.attempts, ip)
				continue
			}
			if len(rec.failures) == 0 {
				delete(rl.attempts, ip)
				continue
			}
			latest := rec.failures[len(rec.failures)-1]
			if now.Sub(latest) >= rl.Window {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware wraps TokenAuthMiddleware with IP-based rate limiting
// to protect against brute-force token guessing.
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			if rl.IsBlocked(ip) {
				w.Header().Set("Retry-After", "900")
				WriteJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "too many failed attempts, try again later",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractClientIP returns the client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (the original client).
		if i := 0; i < len(xff) {
			for j := 0; j < len(xff); j++ {
				if xff[j] == ',' {
					return xff[:j]
				}
			}
			return xff
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
