// RouterShelf addition. Licensed under Apache-2.0; see LICENSE.

package smbserver

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// guardedRoot keeps excluded service subtrees inaccessible in an explicitly
// selected disk-root view. Parent directories are pinned with os.Root, and
// symlinks/hard-linked regular files are refused in this profile. A lexical
// filter alone would let aliases and renames expose credentials.
type guardedRoot struct {
	*os.Root
	denied []string
}

func foldedPrefix(p, prefix string) bool {
	if prefix == "." {
		return true
	}
	a, b := strings.Split(p, "/"), strings.Split(prefix, "/")
	if len(a) < len(b) {
		return false
	}
	for i := range b {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
func (r *guardedRoot) allowed(p string, ancestors bool) bool {
	if p != "." && !fs.ValidPath(p) {
		return false
	}
	for _, d := range r.denied {
		if foldedPrefix(p, d) || ancestors && foldedPrefix(d, p) {
			return false
		}
	}
	return true
}

func (r *guardedRoot) parent(p string) (*os.Root, string, func(), error) {
	if !r.allowed(p, false) {
		return nil, "", nil, fs.ErrPermission
	}
	dir, leaf := path.Dir(p), path.Base(p)
	current := r.Root
	cleanup := func() {
		if current != r.Root {
			current.Close()
		}
	}
	if dir == "." {
		return current, leaf, cleanup, nil
	}
	for _, part := range strings.Split(dir, "/") {
		before, err := current.Lstat(part)
		if err != nil {
			cleanup()
			return nil, "", nil, err
		}
		if !before.IsDir() {
			cleanup()
			return nil, "", nil, fs.ErrPermission
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			cleanup()
			return nil, "", nil, err
		}
		actual, err := next.Stat(".")
		if err != nil || !os.SameFile(before, actual) {
			next.Close()
			cleanup()
			return nil, "", nil, fs.ErrPermission
		}
		cleanup()
		current = next
	}
	return current, leaf, cleanup, nil
}
func safeInfo(fi os.FileInfo) bool {
	if fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if fi.Mode().IsRegular() {
		if st, ok := sysStat(fi); ok && st.nlink > 1 {
			return false
		}
	}
	return true
}
func (r *guardedRoot) Stat(p string) (os.FileInfo, error) {
	if len(r.denied) == 0 {
		return r.Root.Stat(p)
	}
	return r.Lstat(p)
}
func (r *guardedRoot) Lstat(p string) (os.FileInfo, error) {
	if len(r.denied) == 0 {
		return r.Root.Lstat(p)
	}
	parent, leaf, close, err := r.parent(p)
	if err != nil {
		return nil, err
	}
	defer close()
	fi, err := parent.Lstat(leaf)
	if err == nil && !safeInfo(fi) {
		return nil, fs.ErrPermission
	}
	return fi, err
}
func (r *guardedRoot) Open(p string) (*os.File, error) { return r.OpenFile(p, os.O_RDONLY, 0) }
func (r *guardedRoot) OpenFile(p string, flags int, mode os.FileMode) (*os.File, error) {
	if len(r.denied) == 0 {
		return r.Root.OpenFile(p, flags, mode)
	}
	parent, leaf, close, err := r.parent(p)
	if err != nil {
		return nil, err
	}
	defer close()
	f, err := parent.OpenFile(leaf, flags&^os.O_TRUNC|unix.O_NOFOLLOW|unix.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil || !safeInfo(fi) || !(fi.IsDir() || fi.Mode().IsRegular()) {
		f.Close()
		return nil, fs.ErrPermission
	}
	if flags&os.O_TRUNC != 0 {
		if err := f.Truncate(0); err != nil {
			f.Close()
			return nil, err
		}
	}
	return f, nil
}
func (r *guardedRoot) Mkdir(p string, mode os.FileMode) error {
	if len(r.denied) == 0 {
		return r.Root.Mkdir(p, mode)
	}
	parent, leaf, close, err := r.parent(p)
	if err != nil {
		return err
	}
	defer close()
	return parent.Mkdir(leaf, mode)
}
func (r *guardedRoot) Remove(p string) error {
	if len(r.denied) == 0 {
		return r.Root.Remove(p)
	}
	if !r.allowed(p, true) {
		return fs.ErrPermission
	}
	parent, leaf, close, err := r.parent(p)
	if err != nil {
		return err
	}
	defer close()
	return parent.Remove(leaf)
}
func (r *guardedRoot) Rename(from, to string) error {
	if len(r.denied) == 0 {
		return r.Root.Rename(from, to)
	}
	if !r.allowed(from, true) || !r.allowed(to, true) {
		return fs.ErrPermission
	}
	a, old, closeA, err := r.parent(from)
	if err != nil {
		return err
	}
	defer closeA()
	b, newName, closeB, err := r.parent(to)
	if err != nil {
		return err
	}
	defer closeB()
	ad, err := a.Open(".")
	if err != nil {
		return err
	}
	defer ad.Close()
	bd, err := b.Open(".")
	if err != nil {
		return err
	}
	defer bd.Close()
	return unix.Renameat(int(ad.Fd()), old, int(bd.Fd()), newName)
}
func (r *guardedRoot) Chmod(p string, mode os.FileMode) error {
	if len(r.denied) == 0 {
		return r.Root.Chmod(p, mode)
	}
	f, err := r.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Chmod(mode)
}
func (r *guardedRoot) Chtimes(p string, atime, mtime time.Time) error {
	if len(r.denied) == 0 {
		return r.Root.Chtimes(p, atime, mtime)
	}
	f, err := r.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if atime.IsZero() {
		atime = fi.ModTime()
		if st, ok := sysStat(fi); ok {
			atime = st.atime
		}
	}
	if mtime.IsZero() {
		mtime = fi.ModTime()
	}
	return unix.Futimes(int(f.Fd()), []unix.Timeval{unix.NsecToTimeval(atime.UnixNano()), unix.NsecToTimeval(mtime.UnixNano())})
}
func newGuardedRoot(root *os.Root, denied []string) (*guardedRoot, error) {
	for _, p := range denied {
		if !fs.ValidPath(p) || p == "." {
			return nil, errors.New("invalid protected relative path")
		}
	}
	return &guardedRoot{Root: root, denied: append([]string(nil), denied...)}, nil
}
