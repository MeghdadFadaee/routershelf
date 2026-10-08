package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	root, listen, domain, httpListen, certCache string
	drop                                        bool
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Flags override the environment. The launcher loads the private .env file;
// the binary itself does not execute shell configuration files.
func parseConfig(args []string) (config, error) {
	c := config{}
	mount := os.Getenv("RS_USB_MOUNT")
	root := os.Getenv("RS_ROOT")
	cache := os.Getenv("RS_CERT_CACHE")
	if mount != "" {
		if root == "" {
			root = filepath.Join(mount, envDefault("RS_PUBLIC_SUBDIR", "public-storage"))
		}
		if cache == "" {
			cache = filepath.Join(mount, envDefault("RS_SERVICE_SUBDIR", ".public-share-service"), "certificates")
		}
	}
	drop := false
	if raw := os.Getenv("RS_DROP_PRIVILEGES"); raw != "" {
		var err error
		drop, err = strconv.ParseBool(raw)
		if err != nil {
			return c, fmt.Errorf("RS_DROP_PRIVILEGES must be a boolean")
		}
	}
	ip := envDefault("RS_LAN_IP", "127.0.0.1")
	httpsPort := envDefault("RS_HTTPS_PORT", "8888")
	httpPort := envDefault("RS_HTTP_PORT", "10080")
	fs := flag.NewFlagSet("routershelf", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&c.root, "root", root, "Shared directory (RS_ROOT or RS_USB_MOUNT)")
	fs.StringVar(&c.listen, "listen", net.JoinHostPort(ip, httpsPort), "Listener (RS_LAN_IP, RS_HTTPS_PORT)")
	fs.StringVar(&c.domain, "domain", os.Getenv("RS_DOMAIN"), "HTTPS domain (RS_DOMAIN)")
	fs.StringVar(&c.httpListen, "http-listen", net.JoinHostPort(ip, httpPort), "ACME HTTP listener (RS_HTTP_PORT)")
	fs.StringVar(&c.certCache, "cert-cache", cache, "Persistent private cache (RS_CERT_CACHE)")
	fs.BoolVar(&c.drop, "drop-privileges", drop, "Drop to uid/gid 65534 (RS_DROP_PRIVILEGES)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if len(fs.Args()) != 0 {
		return c, fmt.Errorf("unexpected positional arguments")
	}
	if c.root == "" {
		return c, fmt.Errorf("set RS_ROOT / RS_USB_MOUNT or -root")
	}
	for _, addr := range []string{c.listen, c.httpListen} {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || net.ParseIP(host) == nil {
			return c, fmt.Errorf("listeners must use numeric IP:port")
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return c, fmt.Errorf("invalid listener port")
		}
		if c.drop && n < 1024 {
			return c, fmt.Errorf("privilege drop requires internal ports >=1024")
		}
	}
	if c.domain != "" {
		if c.certCache == "" {
			return c, fmt.Errorf("HTTPS requires RS_CERT_CACHE / RS_USB_MOUNT or -cert-cache")
		}
		if c.listen == c.httpListen {
			return c, fmt.Errorf("HTTP and HTTPS listeners must differ")
		}
		if strings.ContainsAny(c.domain, "/: \t\r\n") {
			return c, fmt.Errorf("domain must be a hostname without scheme or port")
		}
		r, err := filepath.Abs(c.root)
		if err != nil {
			return c, err
		}
		p, err := filepath.Abs(c.certCache)
		if err != nil {
			return c, err
		}
		rel, err := filepath.Rel(r, p)
		if err != nil {
			return c, err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return c, fmt.Errorf("certificate cache must be outside shared root")
		}
	}
	return c, nil
}
