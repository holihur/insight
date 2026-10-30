// Package web embeds the compiled React frontend into the Go binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the embedded frontend rooted at the build output directory.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return embedded
	}
	return sub
}

// Available reports whether a real frontend build is embedded.
func Available() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}
