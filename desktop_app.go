//go:build desktop

package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	httpx "github.com/yoke233/zhanggui/internal/adapters/http/server"
	"github.com/yoke233/zhanggui/internal/platform/appcmd"
	"github.com/yoke233/zhanggui/internal/platform/bootstrap"
)

// DesktopBootstrap contains the information the frontend needs to connect.
type DesktopBootstrap struct {
	Token string `json:"token"`
}

// DesktopApp manages the Go backend lifecycle within the Wails desktop shell.
type DesktopApp struct {
	ctx        context.Context
	apiHandler http.Handler
	token      string
	cleanup    func()
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.ctx = ctx

	cfg, dataDir, secrets, err := appcmd.LoadConfig()
	if err != nil {
		log.Fatalf("desktop: load config: %v", err)
	}

	closeLog, err := appcmd.InitAppLogger(dataDir, "desktop")
	if err != nil {
		log.Fatalf("desktop: init logger: %v", err)
	}

	tokenRegistry := httpx.NewTokenRegistry(secrets.Tokens)
	signalCfg := &bootstrap.AgentSignalConfig{
		TokenRegistry: tokenRegistry,
	}

	store, _, runtimeManager, cleanupFn, registrar := bootstrap.Build(
		appcmd.ExpandStorePath(cfg.Store.Path, dataDir),
		nil, cfg,
		bootstrap.SCMTokens{
			GitHub: strings.TrimSpace(secrets.GitHub.PAT),
			Codeup: strings.TrimSpace(secrets.Codeup.PAT),
		},
		nil, signalCfg,
	)
	if store == nil || registrar == nil {
		log.Fatal("desktop: bootstrap failed")
	}

	a.cleanup = func() {
		if runtimeManager != nil {
			_ = runtimeManager.Close()
		}
		if cleanupFn != nil {
			cleanupFn()
		}
		closeLog()
	}

	skipAuth := !cfg.Server.IsAuthRequired()
	srv := httpx.NewServer(httpx.Config{
		Auth:           tokenRegistry,
		RouteRegistrar: registrar,
		SkipAuth:       skipAuth,
	})
	a.apiHandler = srv.Handler()
	a.token = secrets.AdminToken()
}

func (a *DesktopApp) Shutdown(_ context.Context) {
	if a.cleanup != nil {
		a.cleanup()
	}
}

// GetBootstrap returns the desktop bootstrap info (token) to the frontend.
func (a *DesktopApp) GetBootstrap() DesktopBootstrap {
	return DesktopBootstrap{
		Token: a.token,
	}
}

// ServeHTTP delegates API/WS requests from Wails AssetServer to the Go handler.
func (a *DesktopApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.apiHandler != nil {
		a.apiHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "server not ready", http.StatusServiceUnavailable)
	}
}
