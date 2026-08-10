package web

import (
	"io/fs"
	"strings"
	"testing"
)

// readAsset reads one embedded UI file or fails the test.
func readAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(Assets, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// TestAssets_CSPClean guards the RMI-115 invariant that the SPA references no
// external/CDN assets — the whole UI must load from the embedded origin so a
// strict CSP holds. A literal "http://"/"https://" URL (as opposed to the
// "https:" protocol string used to pick ws/wss) would break that.
func TestAssets_CSPClean(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "style.css"} {
		body := readAsset(t, name)
		for _, needle := range []string{"http://", "https://", "src=\"//", "href=\"//"} {
			if strings.Contains(body, needle) {
				t.Errorf("%s references an external asset (%q); the UI must be CSP-clean", name, needle)
			}
		}
	}
}

// TestApp_TeamChatWired asserts the team-mode group chat surface (RMI-118) is
// present in the shipped bundle: the multiUser branch renders the team chat,
// which talks to the /api/chats endpoint set.
func TestApp_TeamChatWired(t *testing.T) {
	app := readAsset(t, "app.js")
	for _, needle := range []string{
		"renderTeamChat", // team chat view exists
		"caps.multiUser", // gated on the multiUser capability
		"/api/chats",     // chat list / create
		"/api/chats/dm",  // DM get-or-create
		"/members",       // membership management
		"/leave",         // leave group
		"chat.message",   // live message fan-out
	} {
		if !strings.Contains(app, needle) {
			t.Errorf("app.js missing team chat wiring: %q", needle)
		}
	}
}
