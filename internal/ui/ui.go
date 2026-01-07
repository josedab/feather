// Package ui provides an embedded web UI for the Feather feature catalog.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// GetFileSystem returns the embedded file system for the static files.
func GetFileSystem() (http.FileSystem, error) {
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}
