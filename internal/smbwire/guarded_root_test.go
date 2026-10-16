// RouterShelf addition. Licensed under Apache-2.0; see LICENSE.

package smbserver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtectedDiskRootProtocol(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".smb-service", ".public-share-service", "parent/private", "ordinary", "destination"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{".smb-service/binary", ".public-share-service/key", "parent/private/secret"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte("private sentinel"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(".public-share-service", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(root, "parent/private/secret"), filepath.Join(root, "hardlink")); err != nil {
		t.Fatal(err)
	}
	c := connect(t, startServer(t, Config{Root: root, DiskMetadata: true, ProtectedPaths: []string{".smb-service", ".public-share-service", "parent/private"}}))
	for _, name := range []string{".smb-service/binary", ".public-share-service/key", "alias/key", "hardlink", "parent/private/secret", ".SMB-SERVICE/binary", ".ſmb-service/binary"} {
		name = strings.ReplaceAll(name, "/", `\`)
		for _, disp := range []uint32{dispOpen, dispOpenIf, dispOverwriteIf} {
			if id, st := c.create(name, accessRW, disp, 0); st == statusSuccess {
				c.close(id)
				t.Fatalf("protected path opened: %s", name)
			}
		}
	}
	for _, name := range c.list("") {
		if strings.EqualFold(name, ".smb-service") || name == ".public-share-service" || name == "alias" || name == "hardlink" {
			t.Fatalf("protected name listed: %q", name)
		}
	}
	for _, name := range c.list("parent") {
		if name == "private" {
			t.Fatal("nested protected directory listed")
		}
	}
	parent, st := c.create("parent", accessRW, dispOpen, optDirectory)
	if st != statusSuccess {
		t.Fatal(st)
	}
	if st, _ := c.do(cmdSetInfo, setInfoBody(parent, fileRename, renameData("relocated", false))); st == statusSuccess {
		t.Fatal("renamed ancestor of protected data")
	}
	c.close(parent)
	id, st := c.create(`ordinary\file`, accessRW, dispCreate, 0)
	if st != statusSuccess {
		t.Fatal(st)
	}
	if st := c.write(id, 0, []byte("ordinary data")); st != statusSuccess {
		t.Fatal(st)
	}
	basic := make([]byte, 40)
	le.PutUint64(basic[16:], filetime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))
	if st, _ := c.do(cmdSetInfo, setInfoBody(id, fileBasic, basic)); st != statusSuccess {
		t.Fatalf("descriptor timestamps: %x", st)
	}
	if st, _ := c.do(cmdSetInfo, setInfoBody(id, fileRename, renameData(`destination\moved`, false))); st != statusSuccess {
		t.Fatalf("cross-directory rename: %x", st)
	}
	if st, _ := c.do(cmdSetInfo, setInfoBody(id, fileRename, renameData(`.smb-service\replacement`, true))); st == statusSuccess {
		t.Fatal("renamed into protected service directory")
	}
	c.close(id)
	if data, err := os.ReadFile(filepath.Join(root, "destination/moved")); err != nil || !bytes.Equal(data, []byte("ordinary data")) {
		t.Fatalf("ordinary file corrupted: %q %v", data, err)
	}
	for _, p := range []string{".smb-service/binary", ".public-share-service/key", "parent/private/secret"} {
		if b, err := os.ReadFile(filepath.Join(root, p)); err != nil || string(b) != "private sentinel" {
			t.Fatalf("private data modified: %s", p)
		}
	}
}
