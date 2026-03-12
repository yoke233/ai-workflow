package httpx

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 0
	allowMap := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowMap[trimmed] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowMap[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func LoggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Printf("method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(start))
		})
	}
}

// SecurityHeadersMiddleware adds common security headers to all responses.
// These headers protect against clickjacking, MIME-type sniffing, XSS, and more.
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			// CSP: allow self, inline styles (Tailwind), and ws/wss for WebSocket.
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data:; font-src 'self' data:")
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBodySizeMiddleware limits the size of request bodies to prevent abuse.
// Default limit is 10MB if maxBytes <= 0.
func MaxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 10 << 20 // 10 MB
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HSTSMiddleware adds Strict-Transport-Security header when the request is over HTTPS
// or when behind a reverse proxy that sets X-Forwarded-Proto.
func HSTSMiddleware(maxAge int) func(http.Handler) http.Handler {
	if maxAge <= 0 {
		maxAge = 63072000 // 2 years
	}
	hstsValue := fmt.Sprintf("max-age=%d; includeSubDomains", maxAge)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RecoveryMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Printf("panic recovered path=%s err=%v", r.URL.Path, rec)
					WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
