package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_NotBlockedInitially(t *testing.T) {
	rl := NewRateLimiter()
	if rl.IsBlocked("1.2.3.4") {
		t.Fatal("expected IP to not be blocked initially")
	}
}

func TestRateLimiter_BlockAfterMaxAttempts(t *testing.T) {
	rl := &RateLimiter{
		attempts:    make(map[string]*ipRecord),
		MaxAttempts: 3,
		Window:      1 * time.Minute,
		Cooldown:    1 * time.Minute,
	}
	ip := "10.0.0.1"
	for i := 0; i < 2; i++ {
		if rl.RecordFailure(ip) {
			t.Fatalf("expected not blocked after %d failures", i+1)
		}
	}
	if !rl.RecordFailure(ip) {
		t.Fatal("expected blocked after 3rd failure")
	}
	if !rl.IsBlocked(ip) {
		t.Fatal("expected IsBlocked to return true")
	}
}

func TestRateLimiter_ResetClearsBlock(t *testing.T) {
	rl := &RateLimiter{
		attempts:    make(map[string]*ipRecord),
		MaxAttempts: 2,
		Window:      1 * time.Minute,
		Cooldown:    1 * time.Minute,
	}
	ip := "10.0.0.2"
	rl.RecordFailure(ip)
	rl.RecordFailure(ip)
	if !rl.IsBlocked(ip) {
		t.Fatal("expected blocked")
	}
	rl.Reset(ip)
	if rl.IsBlocked(ip) {
		t.Fatal("expected unblocked after reset")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := &RateLimiter{
		attempts:    make(map[string]*ipRecord),
		MaxAttempts: 2,
		Window:      1 * time.Minute,
		Cooldown:    1 * time.Minute,
	}
	rl.RecordFailure("a")
	rl.RecordFailure("a")
	if !rl.IsBlocked("a") {
		t.Fatal("expected 'a' blocked")
	}
	if rl.IsBlocked("b") {
		t.Fatal("expected 'b' not blocked")
	}
}

func TestRateLimitMiddleware_BlocksRequest(t *testing.T) {
	rl := &RateLimiter{
		attempts:    make(map[string]*ipRecord),
		MaxAttempts: 1,
		Window:      1 * time.Minute,
		Cooldown:    1 * time.Minute,
	}
	rl.RecordFailure("127.0.0.1")

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		want       string
	}{
		{"remote addr", "1.2.3.4:5678", "", "", "1.2.3.4"},
		{"x-forwarded-for single", "9.9.9.9:1234", "10.0.0.1", "", "10.0.0.1"},
		{"x-forwarded-for multiple", "9.9.9.9:1234", "10.0.0.1, 10.0.0.2", "", "10.0.0.1"},
		{"x-real-ip", "9.9.9.9:1234", "", "192.168.1.1", "192.168.1.1"},
		{"xff takes priority over xri", "9.9.9.9:1234", "10.0.0.1", "192.168.1.1", "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			got := extractClientIP(req)
			if got != tt.want {
				t.Errorf("extractClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
