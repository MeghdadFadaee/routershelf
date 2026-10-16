package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// RedirectHandler redirects allowed read requests to the configured HTTPS host.
func RedirectHandler(domain string) http.Handler {
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
