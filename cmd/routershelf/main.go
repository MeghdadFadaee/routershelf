package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"routershelf/internal/web"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	root, err := os.OpenRoot(cfg.root)
	if err != nil {
		log.Fatal("public directory unavailable")
	}
	root.Close()
	if cfg.drop {
		if err = syscall.Setgroups([]int{}); err != nil {
			log.Fatal(err)
		}
		if err = syscall.Setgid(65534); err != nil {
			log.Fatal(err)
		}
		if err = syscall.Setuid(65534); err != nil {
			log.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp4", cfg.listen)
	if err != nil {
		log.Fatal(err)
	}
	contentHandler := web.Handler(cfg.root)
	var manager *autocert.Manager
	if cfg.domain != "" {
		if cfg.certCache == "" {
			log.Fatal("-cert-cache is required for HTTPS")
		}
		manager = &autocert.Manager{Prompt: autocert.AcceptTOS, HostPolicy: autocert.HostWhitelist(cfg.domain), Cache: autocert.DirCache(cfg.certCache)}
		redirect := web.RedirectHandler(cfg.domain)
		httpListener, err := net.Listen("tcp4", cfg.httpListen)
		if err != nil {
			log.Fatal(err)
		}
		challengeServer := &http.Server{
			Handler:           manager.HTTPHandler(redirect),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       15 * time.Second,
			MaxHeaderBytes:    16 * 1024,
		}
		go func() { log.Fatal(challengeServer.Serve(&boundedListener{httpListener, make(chan struct{}, 16)})) }()
	}
	server := &http.Server{
		Handler:           contentHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       15 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	log.Printf("Read-only public directory listening on %s (domain=%s)", cfg.listen, cfg.domain)
	if manager != nil {
		server.TLSConfig = manager.TLSConfig()
		server.TLSConfig.MinVersion = tls.VersionTLS12
		// HTTP/1.1 plus ACME ALPN; bound connections remain effective for all clients.
		server.TLSConfig.NextProtos = []string{"http/1.1", "acme-tls/1"}
		log.Fatal(server.ServeTLS(&boundedListener{listener, make(chan struct{}, 32)}, "", ""))
	}
	log.Fatal(server.Serve(&boundedListener{listener, make(chan struct{}, 32)}))
}
