package smbserver

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func takeLock(c *testClient, id []byte, flags uint32) ntStatus {
	body := make([]byte, 24)
	le.PutUint16(body, 48)
	le.PutUint16(body[2:], 1)
	copy(body[8:], id)
	body = le.AppendUint64(body, 0)
	body = le.AppendUint64(body, 100)
	body = le.AppendUint32(body, flags)
	body = le.AppendUint32(body, 0)
	st, _ := c.do(cmdLock, body)
	return st
}
func TestRouterShelfLocksProtectData(t *testing.T) {
	addr := startServer(t, Config{DiskMetadata: true})
	a, b := connect(t, addr), connect(t, addr)
	ia, st := a.create("file", accessRW, dispCreate, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	ib, st := b.create("file", accessRW, dispOpen, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	if st := a.write(ia, 0, []byte("original")); st != statusSuccess {
		t.Fatal(st)
	}
	if st := takeLock(a, ia, lockExclusive); st != statusSuccess {
		t.Fatal(st)
	}
	if st := b.write(ib, 0, []byte("corrupt!")); st != statusFileLockConflict {
		t.Fatalf("locked write: %x", st)
	}
	if _, st := b.read(ib, 0, 8); st != statusFileLockConflict {
		t.Fatalf("locked read: %x", st)
	}
	if st, _ := b.do(cmdSetInfo, setInfoBody(ib, fileEndOfFile, le.AppendUint64(nil, 0))); st != statusFileLockConflict {
		t.Fatalf("locked truncate: %x", st)
	}
	if _, st := b.create("file", accessRW, dispOverwrite, 0); st != statusFileLockConflict {
		t.Fatalf("locked overwrite: %x", st)
	}
	if st := takeLock(a, ia, lockUnlock); st != statusSuccess {
		t.Fatal(st)
	}
	if data, st := b.read(ib, 0, 8); st != statusSuccess || string(data) != "original" {
		t.Fatalf("data damaged: %q %x", data, st)
	}
}
func TestRouterShelfShareModes(t *testing.T) {
	addr := startServer(t, Config{})
	a, b := connect(t, addr), connect(t, addr)
	body := createBody("file", accessRW, dispCreate, 0)
	le.PutUint32(body[32:], 1) // only share read
	st, resp := a.do(cmdCreate, body)
	if st != statusSuccess {
		t.Fatal(st)
	}
	id := bytes.Clone(resp[headerSize+64 : headerSize+80])
	if _, st := b.create("file", accessRW, dispOverwrite, 0); st != statusSharingViolation {
		t.Fatalf("conflicting writer: %x", st)
	}
	if _, st := b.create("file", accessRead, dispOpen, 0); st != statusSuccess {
		t.Fatalf("compatible reader: %x", st)
	}
	if st := a.close(id); st != statusSuccess {
		t.Fatal(st)
	}
}
func TestRouterShelfDiskMetadata(t *testing.T) {
	root := t.TempDir()
	addr := startServer(t, Config{Root: root, DiskMetadata: true})
	c := connect(t, addr)
	id, st := c.create("data", accessRW, dispCreate, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	basic := make([]byte, 40)
	le.PutUint32(basic[32:], attrReadOnly)
	if st, _ := c.do(cmdSetInfo, setInfoBody(id, fileBasic, basic)); st != statusNotSupported {
		t.Fatalf("per-file protection falsely acknowledged: %x", st)
	}
	c.close(id)
	if _, st := c.create("data:AFP_Resource", accessRW, dispOpenIf, 0); st != statusNotSupported {
		t.Fatalf("volatile metadata accepted: %x", st)
	}
	for _, name := range []string{"._data", ".DS_Store"} {
		id, st := c.create(name, accessRW, dispCreate, 0)
		if st != statusSuccess {
			t.Fatal(st)
		}
		if st := c.write(id, 0, []byte("persistent metadata")); st != statusSuccess {
			t.Fatal(st)
		}
		c.close(id)
	}
	// A new independent server sees the disk files, with no in-memory metadata.
	d := connect(t, startServer(t, Config{Root: root, DiskMetadata: true}))
	for _, name := range []string{"._data", ".DS_Store"} {
		id, st := d.create(name, accessRead, dispOpen, 0)
		if st != statusSuccess {
			t.Fatal(st)
		}
		data, st := d.read(id, 0, 100)
		if st != statusSuccess || string(data) != "persistent metadata" {
			t.Fatalf("lost metadata: %q %x", data, st)
		}
		d.close(id)
	}
}
func TestRouterShelfTransferAndReconnect(t *testing.T) {
	root := t.TempDir()
	addr := startServer(t, Config{Root: root, DiskMetadata: true})
	c := connect(t, addr)
	id, st := c.create("transfer.partial", accessRW, dispCreate, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	block := bytes.Repeat([]byte("0123456789abcdef"), 4096)
	want := sha256.New()
	const chunks = 192 // 12 MiB; only one 64 KiB payload is held at a time.
	for i := 0; i < chunks; i++ {
		if i == chunks/2 {
			c.nc.Close() // interrupt an open transfer; reconnect and explicitly reopen
			time.Sleep(20 * time.Millisecond)
			c = connect(t, addr)
			id, st = c.create("transfer.partial", accessRW, dispOpen, 0)
			if st != statusSuccess {
				t.Fatal(st)
			}
		}
		if st := c.write(id, uint64(i*len(block)), block); st != statusSuccess {
			t.Fatal(st)
		}
		want.Write(block)
	}
	if st := c.close(id); st != statusSuccess {
		t.Fatal(st)
	}
	f, err := os.Open(filepath.Join(root, "transfer.partial"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got := sha256.New()
	if _, err := io.Copy(got, f); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want.Sum(nil), got.Sum(nil)) {
		t.Fatal("transfer checksum mismatch")
	}
}
func TestRouterShelfDirectoryBoundIsExplicit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= maxDirectoryEntries; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprint(i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := connect(t, startServer(t, Config{Root: root, DiskMetadata: true}))
	id, st := c.create("", accessRead, dispOpen, optDirectory)
	if st != statusSuccess {
		t.Fatal(st)
	}
	st, _ = c.do(cmdQueryDirectory, queryDirectoryBody(id, classNames, "*", maxIO))
	if st != statusInsufficientResources {
		t.Fatalf("oversized listing should fail explicitly: %x", st)
	}
}

func TestRouterShelfCloseReportsDeleteFailure(t *testing.T) {
	root := t.TempDir()
	c := connect(t, startServer(t, Config{Root: root, DiskMetadata: true}))
	id, st := c.create("folder", accessRW, dispCreate, optDirectory)
	if st != statusSuccess {
		t.Fatal(st)
	}
	if st, _ := c.do(cmdSetInfo, setInfoBody(id, fileDisposition, []byte{1})); st != statusSuccess {
		t.Fatal(st)
	}
	if err := os.WriteFile(filepath.Join(root, "folder", "external-change"), []byte("external writer"), 0600); err != nil {
		t.Fatal(err)
	}
	if st := c.close(id); st != statusDirectoryNotEmpty {
		t.Fatalf("delete failure acknowledged as success: %x", st)
	}
}

func TestRouterShelfConcurrentWritesThroughRename(t *testing.T) {
	root := t.TempDir()
	addr := startServer(t, Config{Root: root, DiskMetadata: true})
	a, b := connect(t, addr), connect(t, addr)
	ia, st := a.create("shared", accessRW, dispCreate, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	ib, st := b.create("shared", accessRW, dispOpen, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	const count = 32
	one, two := bytes.Repeat([]byte{1}, maxIO), bytes.Repeat([]byte{2}, maxIO)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < count; i++ {
			if st := a.write(ia, uint64(2*i*maxIO), one); st != statusSuccess {
				t.Errorf("writer a: %x", st)
				return
			}
		}
	}()
	for i := 0; i < count; i++ {
		if st := b.write(ib, uint64((2*i+1)*maxIO), two); st != statusSuccess {
			t.Fatal(st)
		}
	}
	<-done
	if st, _ := b.do(cmdSetInfo, setInfoBody(ib, fileRename, renameData("renamed", false))); st != statusSuccess {
		t.Fatal(st)
	}
	a.close(ia)
	b.close(ib)
	f, err := os.Open(filepath.Join(root, "renamed"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	block := make([]byte, maxIO)
	for i := 0; i < count*2; i++ {
		if _, err := io.ReadFull(f, block); err != nil {
			t.Fatal(err)
		}
		want := one
		if i%2 == 1 {
			want = two
		}
		if !bytes.Equal(block, want) {
			t.Fatalf("concurrent data damaged at chunk %d", i)
		}
	}
}

func TestRouterShelfRefusesNonregularData(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	c := connect(t, startServer(t, Config{Root: root, DiskMetadata: true}))
	if _, st := c.create("pipe", accessRead, dispOpen, 0); st != statusAccessDenied {
		t.Fatalf("FIFO read should be denied: %x", st)
	}
	if _, st := c.create("pipe", accessRW, dispOverwrite, 0); st != statusAccessDenied {
		t.Fatalf("FIFO overwrite should be denied: %x", st)
	}
}
