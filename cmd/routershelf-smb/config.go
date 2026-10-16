package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

// SMB deliberately has no defaults derived from the web service's settings.
type config struct {
	Listen             string   `json:"listen"`
	Mount              string   `json:"storage_mount"`
	Root               string   `json:"root"`
	Share              string   `json:"share"`
	User               string   `json:"user"`
	Hash               string   `json:"nt_hash"`
	ReadOnly           bool     `json:"read_only"`
	Allow              []string `json:"allow"`
	ProtectedPaths     []string `json:"protected_paths"`
	PublicRoots        []string `json:"public_roots"`
	AllowPublicOverlap bool     `json:"allow_public_overlap"`
	UID                int      `json:"uid"`
	GID                int      `json:"gid"`
	Diagnostics        bool     `json:"diagnostics"`
	CaseInsensitive    bool     `json:"case_insensitive"`
	prefixes           []netip.Prefix
	ntHash             [16]byte
	ready              func()
	path               string
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func overlap(a, b string) bool { return within(a, b) || within(b, a) }
func canonical(p string) (string, error) {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return "", errors.New("paths must be clean and absolute")
	}
	return filepath.EvalSymlinks(p)
}

func loadConfig(args []string) (config, bool, error) {
	c := config{Listen: "127.0.0.1:1445", Share: "Storage", ReadOnly: true, UID: 65533, GID: 65533, CaseInsensitive: true}
	fs := flag.NewFlagSet("routershelf-smb", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", os.Getenv("RS_SMB_CONFIG"), "Private JSON configuration (RS_SMB_CONFIG)")
	check := fs.Bool("check-config", false, "Validate without listening or writing")
	if err := fs.Parse(args); err != nil {
		return c, false, err
	}
	if len(fs.Args()) != 0 || *path == "" {
		return c, false, errors.New("use -config /absolute/private/smb.json or RS_SMB_CONFIG; see docs/smb.md")
	}
	p, err := canonical(*path)
	if err != nil {
		return c, false, errors.New("configuration must exist at an absolute path")
	}
	c.path = p
	f, err := os.Open(p)
	if err != nil {
		return c, false, errors.New("cannot open private SMB configuration")
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 32768 {
		return c, false, errors.New("SMB configuration must be a regular file with mode 0600 or 0400 on a permission-enforcing filesystem")
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Uid != 0 && int(st.Uid) != os.Geteuid() {
		return c, false, errors.New("configuration must be owned by root or the current account")
	}
	dec := json.NewDecoder(io.LimitReader(f, 32769))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, false, errors.New("invalid SMB JSON configuration (unknown field, type, or syntax)")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return c, false, errors.New("configuration must contain one JSON object under 32 KiB")
	}
	if err := c.validate(); err != nil {
		return c, false, err
	}
	return c, *check, nil
}
func (c *config) validate() error {
	host, port, err := net.SplitHostPort(c.Listen)
	ip, e := netip.ParseAddr(host)
	n, pe := strconv.Atoi(port)
	if err != nil || e != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() || pe != nil || n < 1 || n > 65535 {
		return errors.New("listen must be a specific numeric IPv4 address and valid port")
	}
	// Never permit a public address or a wildcard listener. Prefixes also filter peers.
	if !ip.IsPrivate() && !ip.IsLoopback() {
		return errors.New("SMB listener must be private IPv4 or loopback")
	}
	if len(c.Allow) == 0 {
		return errors.New("allow must explicitly list trusted private IPv4 client prefixes")
	}
	for _, v := range c.Allow {
		p, err := netip.ParsePrefix(v)
		if err != nil || !p.Addr().Is4() || p != p.Masked() || p.Bits() < 8 {
			return errors.New("invalid trusted client prefix")
		}
		last := p.Addr().As4()
		bits := p.Bits()
		for i := bits; i < 32; i++ {
			last[i/8] |= 1 << uint(7-i%8)
		}
		if !(p.Addr().IsPrivate() && netip.AddrFrom4(last).IsPrivate() || p.Addr().IsLoopback() && netip.AddrFrom4(last).IsLoopback()) {
			return errors.New("client prefixes must stay inside private IPv4 or loopback space")
		}
		c.prefixes = append(c.prefixes, p)
	}
	if c.UID <= 0 || c.GID <= 0 || c.UID > 2147483647 || c.GID > 2147483647 {
		return errors.New("uid and gid must be non-root positive IDs")
	}
	for _, s := range []string{c.Share, c.User} {
		if strings.IndexFunc(s, unicode.IsControl) >= 0 || len(s) < 1 || len(s) > 64 || strings.ContainsAny(s, "\\/:*?\"<>| \t\r\n\x00") {
			return errors.New("share and user require 1–64 plain characters without spaces or separators")
		}
	}
	if strings.EqualFold(c.Share, "IPC$") {
		return errors.New("reserved share name")
	}
	hash, err := hex.DecodeString(c.Hash)
	if err != nil || len(hash) != 16 {
		return errors.New("nt_hash must contain 32 hexadecimal characters")
	}
	copy(c.ntHash[:], hash)
	if c.Hash == "00000000000000000000000000000000" || strings.EqualFold(c.Hash, "31d6cfe0d16ae931b73c59d7e0c089c0") {
		return errors.New("replace placeholder or empty-password NT hash")
	}
	if c.Mount == "/" {
		return errors.New("storage_mount must select storage, not the system root")
	}
	c.Mount, err = canonical(c.Mount)
	if err != nil || c.Mount == "/" {
		return errors.New("storage_mount must be an existing absolute storage directory outside the system root")
	}
	c.Root, err = canonical(c.Root)
	if err != nil {
		return errors.New("selected root is unavailable")
	}
	fi, err := os.Stat(c.Root)
	if err != nil || !fi.IsDir() || c.Root == c.Mount || !within(c.Mount, c.Root) {
		return errors.New("root must be an existing directory strictly below storage_mount; it is never created automatically")
	}
	if within(c.Root, c.path) {
		return errors.New("private SMB configuration cannot be inside the share")
	}

	protected := append([]string{filepath.Join(c.Mount, ".public-share-service"), filepath.Join(c.Mount, ".smb-service")}, c.ProtectedPaths...)
	if exe, err := os.Executable(); err == nil {
		protected = append(protected, exe)
	}
	for _, p := range protected {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return errors.New("protected_paths must use clean absolute paths")
		}
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		} else if !os.IsNotExist(err) {
			return errors.New("cannot resolve a protected path")
		}
		if within(c.Root, p) || within(p, c.Root) {
			return errors.New("SMB root overlaps a private service or protected path")
		}
	}
	public := append([]string{filepath.Join(c.Mount, "public-storage")}, c.PublicRoots...)
	for _, p := range public {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return errors.New("public_roots must use clean absolute paths")
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			p = resolved
		} else if !os.IsNotExist(err) {
			return errors.New("cannot resolve a public root")
		}
		if overlap(c.Root, p) && !c.AllowPublicOverlap {
			return errors.New("SMB overlaps public web storage; select a private sibling or explicitly set allow_public_overlap")
		}
	}
	return nil
}
