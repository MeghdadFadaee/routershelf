package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	smb "routershelf/internal/smbwire"
)

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
