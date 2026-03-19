package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	webassets "github.com/yoke233/zhanggui/web"
)

type Config struct {
	Addr           string
	Auth           *TokenRegistry
	AllowedOrigins []string
	Frontend       fs.FS
	RouteRegistrar func(chi.Router)
	Logger         *log.Logger
	// SkipAuth disables token authentication entirely.
	// Used when the server listens on localhost only (desktop / local dev).
	SkipAuth bool
}

type Server struct {
	httpServer *http.Server
	router     chi.Router
}

func NewServer(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(os.Stdout, "[web] ", log.LstdFlags)
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	frontendFS := cfg.Frontend
	if frontendFS == nil {
		if embeddedFS, err := webassets.DistFS(); err == nil {
			frontendFS = embeddedFS
		}
	}

	r := chi.NewRouter()
	r.Use(RecoveryMiddleware(logger))
	r.Use(LoggingMiddleware(logger))
	r.Use(SecurityHeadersMiddleware())
	r.Use(HSTSMiddleware(0))
	r.Use(CORSMiddleware(cfg.AllowedOrigins))
	r.Use(MaxBodySizeMiddleware(0))
	authRequired := !cfg.SkipAuth && cfg.Auth != nil && !cfg.Auth.IsEmpty()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// Unauthenticated endpoint: lets frontend detect if token login is needed.
	r.Get("/api/auth/status", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]bool{"auth_required": authRequired})
	})
	if cfg.RouteRegistrar != nil {
		r.Route("/api", func(r chi.Router) {
			if authRequired {
				rl := NewRateLimiter()
				r.Use(RateLimitMiddleware(rl))
				r.Use(TokenAuthMiddleware(cfg.Auth, WithRateLimiter(rl), WithAuthLogger(logger)))
			}
			cfg.RouteRegistrar(r)
		})
	}
	if frontendFS != nil {
		spa := newSPAFallbackHandler(frontendFS)
		r.NotFound(spa.ServeHTTP)
	}
	return &Server{
		router: r,
		httpServer: &http.Server{
			Addr:    cfg.Addr,
			Handler: r,
		},
	}
}

// Handler returns the underlying HTTP handler (chi router).
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) Start() error {
	err := s.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type spaFallbackHandler struct {
	files fs.FS
}

func newSPAFallbackHandler(frontendFS fs.FS) spaFallbackHandler {
	return spaFallbackHandler{files: frontendFS}
}

func (h spaFallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	cleanPath := path.Clean("/" + r.URL.Path)
	if isAPIRoute(cleanPath) {
		http.NotFound(w, r)
		return
	}
	staticPath := strings.TrimPrefix(cleanPath, "/")
	if staticPath != "" && fileExists(h.files, staticPath) {
		http.ServeFileFS(w, r, h.files, staticPath)
		return
	}
	http.ServeFileFS(w, r, h.files, "index.html")
}

func isAPIRoute(requestPath string) bool {
	const apiPrefix = "/api"
	return strings.HasPrefix(strings.ToLower(requestPath), apiPrefix)
}

func fileExists(frontendFS fs.FS, name string) bool {
	f, err := frontendFS.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	return err == nil && !info.IsDir()
}
