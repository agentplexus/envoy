// Package web provides embedded web assets for the OpenAI-compatible API server.
package web

import "embed"

//go:embed index.html
var Assets embed.FS
