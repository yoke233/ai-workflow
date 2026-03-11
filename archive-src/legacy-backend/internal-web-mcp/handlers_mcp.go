package web

import (
	"github.com/go-chi/chi/v5"
)

// registerMCPRoutes is intentionally a no-op.
// MCP has been removed from the default /api/v1 runtime path and the source is
// retained only as archived legacy implementation for later cleanup/reference.
func registerMCPRoutes(_ chi.Router, _ Config) {
}
