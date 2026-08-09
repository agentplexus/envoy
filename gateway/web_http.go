package gateway

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/plexusone/omniagent/config"
)

// WebHTTPConfig configures the embedded web UI handlers.
type WebHTTPConfig struct {
	// Capabilities is served at GET /api/capabilities and drives the SPA's
	// capability-aware rendering (TRD §1a/§6).
	Capabilities config.Capabilities
	// Assets is the embedded web/dist directory (index.html, app.js,
	// style.css, ...). No external/CDN assets are ever referenced.
	Assets fs.FS
	Logger *slog.Logger
}

// WebHTTP serves the embedded SPA and its capabilities endpoint. The two
// handlers are mounted separately (CapabilitiesHandler at the exact path
// "/api/capabilities", AssetsHandler at the subtree "/") so capabilities
// keeps working even when team mode also mounts "/api/" — Go's ServeMux
// prefers the longer, more specific pattern.
type WebHTTP struct {
	caps   config.Capabilities
	assets fs.FS
	logger *slog.Logger
}

// NewWebHTTP builds the web UI handler set.
func NewWebHTTP(cfg WebHTTPConfig) *WebHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &WebHTTP{caps: cfg.Capabilities, assets: cfg.Assets, logger: cfg.Logger}
}

// CapabilitiesHandler serves GET /api/capabilities.
func (h *WebHTTP) CapabilitiesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(h.caps); err != nil {
			h.logger.Error("encode capabilities response", "error", err)
		}
	})
}

// AssetsHandler serves the embedded SPA at "/".
func (h *WebHTTP) AssetsHandler() http.Handler {
	return http.FileServer(http.FS(h.assets))
}
