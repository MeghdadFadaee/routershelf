package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"routershelf/internal/theme"
)

type entry struct {
	Name, Size, Modified string
	Link                 template.URL
	Dir                  bool
	Inline               bool
}
type page struct {
	Path      string
	Parent    template.URL
	Entries   []entry
	Truncated bool
}

var listing = template.Must(template.New("listing").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Index of {{.Path}}</title><link rel="stylesheet" href="/_nais/autoindex.css"><link rel="icon" href="/_nais/favicon.ico"></head><body><h1><span>Index of {{.Path}}</span></h1><pre>{{if .Parent}}<a href="../">../</a>
{{end}}{{range .Entries}}<a href="{{.Link}}"{{if not .Dir}} data-file="true"{{if not .Inline}} download{{end}}{{end}}>{{.Name}}{{if .Dir}}/{{end}}</a>  {{.Modified}}  {{.Size}}
{{else}}This folder is empty.
{{end}}{{if .Truncated}}Only the first 5,000 entries are displayed.
{{end}}</pre><script src="/_nais/autoindex.js" defer></script></body></html>`))

func escapedPath(p string) template.URL {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return template.URL(strings.Join(parts, "/"))
}
func sizeLabel(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	for _, u := range []struct {
		n int64
		s string
	}{{1 << 30, "GiB"}, {1 << 20, "MiB"}, {1 << 10, "KiB"}} {
		if n >= u.n {
			return fmt.Sprintf("%.1f %s", float64(n)/float64(u.n), u.s)
		}
	}
	return ""
}
func handler(rootPath string) http.Handler {
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

type boundedListener struct {
	net.Listener
	slots chan struct{}
}
type boundedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *boundedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
func (l *boundedListener) Accept() (net.Conn, error) {
	l.slots <- struct{}{}
	c, e := l.Listener.Accept()
	if e != nil {
		<-l.slots
		return nil, e
	}
	return &boundedConn{Conn: c, release: func() { <-l.slots }}, nil
}
func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	root, addr, domain, httpAddr, cache, drop := &cfg.root, &cfg.listen, &cfg.domain, &cfg.httpListen, &cfg.certCache, &cfg.drop
	r, e := os.OpenRoot(*root)
	if e != nil {
		log.Fatal("public directory unavailable")
	}
	r.Close()
	if *drop {
		if e = syscall.Setgroups([]int{}); e != nil {
			log.Fatal(e)
		}
		if e = syscall.Setgid(65534); e != nil {
			log.Fatal(e)
		}
		if e = syscall.Setuid(65534); e != nil {
			log.Fatal(e)
		}
	}
	l, e := net.Listen("tcp4", *addr)
	if e != nil {
		log.Fatal(e)
	}
	contentHandler := handler(*root)
	var manager *autocert.Manager
	if *domain != "" {
		if *cache == "" {
			log.Fatal("-cert-cache is required for HTTPS")
		}
		manager = &autocert.Manager{Prompt: autocert.AcceptTOS, HostPolicy: autocert.HostWhitelist(*domain), Cache: autocert.DirCache(*cache)}
		redirect := redirectHandler(*domain)
		hl, err := net.Listen("tcp4", *httpAddr)
		if err != nil {
			log.Fatal(err)
		}
		hs := &http.Server{Handler: manager.HTTPHandler(redirect), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 16 * 1024}
		go func() { log.Fatal(hs.Serve(&boundedListener{hl, make(chan struct{}, 16)})) }()
	}
	s := &http.Server{Handler: contentHandler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Minute, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 16 * 1024}
	log.Printf("Read-only public directory listening on %s (domain=%s)", *addr, *domain)
	if manager != nil {
		s.TLSConfig = manager.TLSConfig()
		s.TLSConfig.MinVersion = tls.VersionTLS12
		// HTTP/1.1 plus ACME ALPN; bound connections remain effective for all clients.
		s.TLSConfig.NextProtos = []string{"http/1.1", "acme-tls/1"}
		log.Fatal(s.ServeTLS(&boundedListener{l, make(chan struct{}, 32)}, "", ""))
	}
	log.Fatal(s.Serve(&boundedListener{l, make(chan struct{}, 32)}))
}

func redirectHandler(domain string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !strings.EqualFold(host, domain) {
			http.Error(w, "Unknown host", http.StatusMisdirectedRequest)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Read-only service", 405)
			return
		}
		target := url.URL{Scheme: "https", Host: domain, Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery}
		http.Redirect(w, r, target.String(), http.StatusPermanentRedirect)
	})
}

// Static assets are identified explicitly; unrecognized files remain downloads.
func inlineType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json", ".map":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".wasm":
		return "application/wasm"
	default:
		return ""
	}
}
func serveFile(w http.ResponseWriter, r *http.Request, name string, info os.FileInfo, f io.ReadSeeker) {
	contentType := inlineType(name)
	if contentType == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	} else {
		w.Header().Set("Content-Type", contentType)
		w.Header().Del("Content-Disposition")
		// Published static pages may use inline scripts/styles and external assets.
		// Directory listings retain their stricter policy; no server-side code runs.
		w.Header().Set("Content-Security-Policy", "object-src 'none'; frame-ancestors 'none'; base-uri 'self'")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
