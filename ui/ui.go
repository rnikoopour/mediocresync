package ui

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed all:dist
var files embed.FS

// FS returns a sub-filesystem rooted at dist/, suitable for http.FileServer.
func FS() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		log.Fatalf("failed to create static sub-FS: %v", err)
	}
	return sub
}
