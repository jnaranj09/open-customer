package httpapi

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	cacheImmutable = "public, max-age=31536000, immutable"
	cacheNoCache   = "no-cache"
	immutablePath  = "_app/immutable/"
	indexFile      = "index.html"
)

// noCacheFiles are the built files the browser must revalidate on every load.
var noCacheFiles = map[string]bool{
	indexFile:           true,
	"service-worker.js": true,
	"_app/version.json": true,
}

// staticHandler serves the built SPA from build. The index (and the fallback for every unknown
// path or directory) goes through http.ServeContent: http.FileServer answers /index.html with a
// 301 to /, and the service worker would precache that redirect. Other files use
// http.FileServerFS. Routing /api/ paths away from here is the caller's job.
func staticHandler(build fs.FS) http.Handler {
	files := http.FileServerFS(build)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || name == indexFile || !isFile(build, name) {
			serveIndex(w, r, build)
			return
		}
		switch {
		case strings.HasPrefix(name, immutablePath):
			w.Header().Set("Cache-Control", cacheImmutable)
		case noCacheFiles[name]:
			w.Header().Set("Cache-Control", cacheNoCache)
		}
		files.ServeHTTP(w, r)
	})
}

func isFile(build fs.FS, name string) bool {
	info, err := fs.Stat(build, name)
	return err == nil && !info.IsDir()
}

func serveIndex(w http.ResponseWriter, r *http.Request, build fs.FS) {
	index, err := fs.ReadFile(build, indexFile)
	if err != nil {
		problem.Write(w, r, fmt.Errorf("read embedded index: %w", err))
		return
	}
	w.Header().Set("Cache-Control", cacheNoCache)
	http.ServeContent(w, r, indexFile, time.Time{}, bytes.NewReader(index))
}
