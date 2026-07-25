// Package web embeds the built SPA so the serve binary is self-contained.
package web

import "embed"

// Dist holds the Vite build output (web/dist). A placeholder index.html is
// committed so the Go build works before the frontend exists.
//
//go:embed all:dist
var Dist embed.FS
