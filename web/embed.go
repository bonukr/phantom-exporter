// Package web embeds the GUI assets served by the HTTP server.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html app.js style.css
var assets embed.FS

// Assets returns the embedded GUI filesystem.
func Assets() fs.FS {
	return assets
}
