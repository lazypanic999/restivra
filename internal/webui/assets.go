package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var assets embed.FS

func Handler() http.Handler {
	files, _ := fs.Sub(assets, "dist")
	return http.FileServer(http.FS(files))
}
