package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorageReplacementClosesIndependentService(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("do not drop the test process's root privileges")
	}
	root := filepath.Join(t.TempDir(), "share")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()
	c := config{Mount: "/", Root: root, Listen: addr, UID: os.Geteuid(), GID: os.Getegid(), Share: "Storage", User: "storage", ReadOnly: true}
	ready := make(chan struct{})
	c.ready = func() { close(ready) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, c) }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("startup failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("startup never ready")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		nc, err := net.DialTimeout("tcp4", addr, 100*time.Millisecond)
		if err == nil {
			nc.Close()
			break
		}
		select {
		case err := <-done:
			t.Fatalf("service exited: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("service never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.Rename(root, root+"-removed"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "disappeared or changed") {
			t.Fatalf("expected storage loss: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("storage replacement did not close service")
	}
	nc, err := net.DialTimeout("tcp4", addr, 100*time.Millisecond)
	if err == nil {
		nc.Close()
		t.Fatal("listener survived storage replacement")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("replacement storage was initialized")
	}
}
