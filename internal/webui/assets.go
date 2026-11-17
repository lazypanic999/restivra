package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed all:dist
var assets embed.FS

func Handler() http.Handler {
	files, _ := fs.Sub(assets, "dist")
	return http.FileServer(http.FS(files))
}

// DirectoryHandler is opt-in for local frontend development. os.Root confines
// file access to compiled assets, including when a symlink tries to escape.
func DirectoryHandler(directory string) (http.Handler, func() error, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, nil, err
	}
	files := root.FS()
	server := http.FileServer(http.FS(files))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, segment := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(segment, ".") {
				http.NotFound(w, r)
				return
			}
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "."
		}
		info, err := fs.Stat(files, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			index, err := fs.Stat(files, path.Join(name, "index.html"))
			if err != nil || !index.Mode().IsRegular() {
				http.NotFound(w, r)
				return
			}
		}
		server.ServeHTTP(w, r)
	})
	return handler, root.Close, nil
}
