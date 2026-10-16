package web

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
)

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
