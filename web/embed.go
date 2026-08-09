// Package web embeds the OmniAgent browser UI: a single capability-driven
// SPA (TRD §6) with no build step and no external/CDN assets, served by the
// gateway whenever the web UI is enabled (personal or team, config.Config.WebUIEnabled).
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// Assets is the embedded UI, rooted at dist so index.html/app.js/style.css
// are served at "/", "/app.js", "/style.css".
var Assets = mustSub(distFS, "dist")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err) // programming error: dist is embedded above
	}
	return sub
}
