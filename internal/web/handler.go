package web

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"routershelf/internal/theme"
)

// Handler serves public read-only content from rootPath, reopening the root
// for each request so removable storage does not retain a stale descriptor.
func Handler(rootPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Read-only service", 405)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/_nais/") {
			name := strings.TrimPrefix(r.URL.Path, "/_nais/")
			types := map[string]string{"autoindex.css": "text/css; charset=utf-8", "autoindex.js": "text/javascript; charset=utf-8", "favicon.ico": "image/x-icon", "favicon.png": "image/png"}
			contentType, ok := types[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			data, err := theme.Files.ReadFile("nais/" + name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
			return
		}
		p := r.URL.Path
		if strings.ContainsAny(p, "\\\x00\r\n") {
			http.Error(w, "Invalid path", 400)
			return
		}
		for _, part := range strings.Split(p, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		name := strings.TrimPrefix(p, "/")
		if name == "" {
			name = "."
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			http.Error(w, "Storage unavailable", 503)
			return
		}
		defer root.Close()
		f, err := root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			if !strings.HasSuffix(p, "/") {
				http.Redirect(w, r, string(escapedPath(p+"/")), 301)
				return
			}
			index, indexErr := root.Open(path.Join(name, "index.html"))
			if indexErr == nil {
				defer index.Close()
				indexInfo, statErr := index.Stat()
				if statErr == nil && indexInfo.Mode().IsRegular() {
					serveFile(w, r, "index.html", indexInfo, index)
					return
				}
			}
			rows, err := f.ReadDir(5000)
			if err != nil && err != io.EOF {
				http.Error(w, "Cannot list folder", 500)
				return
			}
			pg := page{Path: p, Truncated: len(rows) == 5000}
			if p != "/" {
				parent := path.Dir(strings.TrimSuffix(p, "/"))
				if parent != "/" {
					parent += "/"
				}
				pg.Parent = escapedPath(parent)
			}
			for _, row := range rows {
				if strings.HasPrefix(row.Name(), ".") {
					continue
				}
				st, err := root.Stat(path.Join(name, row.Name()))
				if err != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
					continue
				}
				link := p + row.Name()
				label := sizeLabel(st.Size())
				if st.IsDir() {
					link += "/"
					label = "—"
				}
				pg.Entries = append(pg.Entries, entry{Name: row.Name(), Size: label, Modified: st.ModTime().UTC().Format("2006-01-02 15:04 UTC"), Link: escapedPath(link), Dir: st.IsDir(), Inline: inlineType(row.Name()) != ""})
			}
			sort.Slice(pg.Entries, func(i, j int) bool {
				if pg.Entries[i].Dir != pg.Entries[j].Dir {
					return pg.Entries[i].Dir
				}
				return pg.Entries[i].Name < pg.Entries[j].Name
			})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			if r.Method == http.MethodGet {
				if err := listing.Execute(w, pg); err != nil {
					log.Print("listing response interrupted")
				}
			}
			return
		}
		if !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		serveFile(w, r, name, info, f)
	})
}
