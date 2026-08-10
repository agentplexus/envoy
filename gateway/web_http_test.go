package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/plexusone/omniagent/config"
)

func TestWebHTTP_CapabilitiesHandler(t *testing.T) {
	caps := config.Capabilities{MultiUser: true, AuthRequired: true, GroupChats: true, Admin: true, Catalog: true}
	h := NewWebHTTP(WebHTTPConfig{Capabilities: caps})

	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	rec := httptest.NewRecorder()
	h.CapabilitiesHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"multiUser":true,"authRequired":true,"groupChats":true,"admin":true,"catalog":true,"googleSso":false,"githubSso":false}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}
}

func TestWebHTTP_CapabilitiesHandler_MethodNotAllowed(t *testing.T) {
	h := NewWebHTTP(WebHTTPConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/capabilities", nil)
	rec := httptest.NewRecorder()
	h.CapabilitiesHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestWebHTTP_AssetsHandler(t *testing.T) {
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>shell</html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('hi')")},
	}
	h := NewWebHTTP(WebHTTPConfig{Assets: assets})

	for _, path := range []string{"/", "/app.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.AssetsHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, rec.Code)
		}
	}
}
