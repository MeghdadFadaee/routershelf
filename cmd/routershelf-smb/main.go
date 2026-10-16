// routershelf-smb is an independent private file service. It never changes
// firewall, routing, firmware settings, or the web service.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	smb "routershelf/internal/smbwire"
)

func mounted(path string) bool {
	if runtime.GOOS != "linux" {
		return true
	}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		v := strings.Fields(sc.Text())
		if len(v) > 1 {
			p := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(v[1])
			if p == path {
				return true
			}
		}
	}
	return false
}
func drop(c config) error {
	if os.Geteuid() != 0 {
		if os.Geteuid() != c.UID || os.Getegid() != c.GID {
			return errors.New("run as configured uid/gid or start as root to bind and drop privileges")
		}
		return nil
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return err
	}
	if err := syscall.Setgid(c.GID); err != nil {
		return err
	}
	return syscall.Setuid(c.UID)
}
func serve(ctx context.Context, c config) error {
	if !mounted(c.Mount) {
		return errors.New("expected USB mount is unavailable; refusing underlying mount directory")
	}
	initial, err := os.Stat(c.Root)
	if err != nil {
		return errors.New("share unavailable")
	}
	ln, err := net.Listen("tcp4", c.Listen)
	if err != nil {
		return errors.New("cannot bind SMB listener; check address, privileges and existing SMB services")
	}
	defer ln.Close()
	if err := drop(c); err != nil {
		return fmt.Errorf("cannot drop SMB privileges: %w", err)
	}
	// Confirm the dropped account can read this directory before announcing ready.
	d, err := os.Open(c.Root)
	if err != nil {
		return errors.New("share is unreadable by configured account")
	}
	_, err = d.Readdirnames(1)
	d.Close()
	if err != nil && err != io.EOF {
		return errors.New("share cannot be listed")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lost := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := os.Stat(c.Root)
				if err != nil || !os.SameFile(initial, current) || !mounted(c.Mount) {
					close(lost)
					cancel()
					return
				}
			}
		}
	}()
	var diagnostic func(string, ...any)
	if c.Diagnostics {
		diagnostic = log.Printf
	}
	err = smb.Serve(ctx, ln, smb.Config{ProtectedPaths: c.protectedRelative, ExpectedRoot: initial, Ready: func() {
		log.Print("SMB ready; authenticated signing required; read_only=", c.ReadOnly)
		if c.ready != nil {
			c.ready()
		}
	}, Logf: diagnostic, Root: c.Root, Share: c.Share, User: c.User, NTHash: c.ntHash, Allow: c.prefixes, ReadOnly: c.ReadOnly, DiskMetadata: true, CaseInsensitive: c.CaseInsensitive})
	select {
	case <-lost:
		return errors.New("USB storage disappeared or changed; sessions closed; reconnect after storage recovers")
	default:
		return err
	}
}
func run() error {
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "-help") {
		fmt.Println("RouterShelf SMB (experimental)\nUsage: routershelf-smb -config /absolute/private/smb.json [-check-config]\n       routershelf-smb -hash-password < password-from-hidden-prompt\nRS_SMB_CONFIG supplies the default config path. No web configuration is required.\nSee docs/smb.md for Finder setup, permissions, updates and recovery.")
		return nil
	}
	if len(os.Args) == 2 && os.Args[1] == "-hash-password" {
		// Pipe from a hidden terminal prompt; never put a password in argv/env.
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 1025))
		if err != nil {
			return err
		}
		b = []byte(strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"))
		if len(b) < 12 || len(b) > 1024 {
			return errors.New("use a unique password of 12–1024 bytes")
		}
		h := smb.NTHash(string(b))
		clear(b)
		fmt.Println(hex.EncodeToString(h[:]))
		return nil
	}
	c, check, err := loadConfig(os.Args[1:])
	if err != nil {
		return err
	}
	if check {
		fmt.Println("SMB configuration valid; no service started")
		return nil
	}
	runtime.GOMAXPROCS(1)
	debug.SetMemoryLimit(20 << 20)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return serve(ctx, c)
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
