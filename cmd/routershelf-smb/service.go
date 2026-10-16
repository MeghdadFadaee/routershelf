package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	smb "routershelf/internal/smbwire"
)

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
	err = smb.Serve(ctx, ln, smb.Config{
		ProtectedPaths: c.protectedRelative,
		ExpectedRoot:   initial,
		Ready: func() {
			log.Print("SMB ready; authenticated signing required; read_only=", c.ReadOnly)
			if c.ready != nil {
				c.ready()
			}
		},
		Logf:            diagnostic,
		Root:            c.Root,
		Share:           c.Share,
		User:            c.User,
		NTHash:          c.ntHash,
		Allow:           c.prefixes,
		ReadOnly:        c.ReadOnly,
		DiskMetadata:    true,
		CaseInsensitive: c.CaseInsensitive,
	})
	select {
	case <-lost:
		return errors.New("USB storage disappeared or changed; sessions closed; reconnect after storage recovers")
	default:
		return err
	}
}
