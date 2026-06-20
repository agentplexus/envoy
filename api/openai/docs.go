package openai

import (
	"net/http"

	scalargo "github.com/bdpiprava/scalar-go"
)

// handleScalarDocs serves the Scalar API documentation UI.
func (s *Server) handleScalarDocs(w http.ResponseWriter, _ *http.Request) {
	html, err := scalargo.NewV2(
		scalargo.WithSpecURL(s.config.APIPrefix+"/openapi.json"),
		scalargo.WithMetaDataOpts(
			scalargo.WithTitle("OmniAgent API"),
		),
		scalargo.WithTheme(scalargo.ThemeDefault),
		scalargo.WithDarkMode(),
		scalargo.WithLayout(scalargo.LayoutModern),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}
